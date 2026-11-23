// 本文件实现工具业务：内置工具播种、CRUD 与执行器（章节体检 / 去 AI 味 / 摘要 / 角色出场 /
// 伏笔回收 / 设定检索）。执行器全部是确定性服务端逻辑，不额外消耗模型调用。

package service

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/2132691342/NovelWorkbench/backend/internal/ai"
	"github.com/2132691342/NovelWorkbench/backend/internal/ai/check"
	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
	"github.com/2132691342/NovelWorkbench/backend/internal/repo"
)

// 工具执行相关的规模上限。
const (
	toolDigestDefaultSentences = 8    // 摘要默认保留句数
	toolDigestMaxSentences     = 30   // 摘要句数上限
	toolResultMaxRunes         = 4000 // 工具结果注入上下文的最大字符数
	toolLoreLookupLimit        = 5    // 设定检索最多返回条数
	toolHookStaleDays          = 30   // 伏笔放置超过该天数提示优先回收
)

// ToolService 工具业务：播种、CRUD 与执行。
type ToolService struct {
	toolRepo      *repo.ToolRepo
	projectRepo   *repo.ProjectRepo
	chapterRepo   *repo.ChapterRepo
	characterRepo *repo.CharacterRepo
	loreRepo      *repo.LoreRepo
	hookRepo      *repo.ContinuityRepo
}

// NewToolService 构造工具服务。
func NewToolService(toolRepo *repo.ToolRepo, projectRepo *repo.ProjectRepo, chapterRepo *repo.ChapterRepo,
	characterRepo *repo.CharacterRepo, loreRepo *repo.LoreRepo, hookRepo *repo.ContinuityRepo) *ToolService {
	return &ToolService{toolRepo: toolRepo, projectRepo: projectRepo, chapterRepo: chapterRepo,
		characterRepo: characterRepo, loreRepo: loreRepo, hookRepo: hookRepo}
}

// SeedDefaults 播种并同步内置工具：缺则插入；出厂内容有代码新版本时覆盖；作者改过的保留；
// 已下线的内置工具直接删除。
func (s *ToolService) SeedDefaults(ctx context.Context) error {
	codeSet := ai.BuiltinToolCodeSet()
	builtinList, err := s.toolRepo.ListBuiltin(ctx)
	if err != nil {
		return err
	}
	for i := range builtinList {
		if !codeSet[builtinList[i].ToolCode] {
			if err := s.toolRepo.Delete(ctx, builtinList[i].ToolULID); err != nil {
				return err
			}
		}
	}
	for _, builtin := range ai.BuiltinTools() {
		existing, err := s.toolRepo.GetByCode(ctx, builtin.ToolCode)
		if err != nil {
			return err
		}
		paramJSON := marshalToolParams(toToolParamList(builtin.ParamList))
		assetHash := builtin.ContentHash()
		if existing == nil {
			do := &domain.ToolDO{
				ToolULID:        pkg.NewULID(domain.IDPrefixTool),
				ToolCode:        builtin.ToolCode,
				ToolName:        builtin.ToolName,
				ToolTypeCode:    domain.ToolTypeBuiltin,
				DescriptionText: builtin.DescriptionText,
				TriggerKeywords: builtin.TriggerKeywords,
				ParamSchemaJSON: paramJSON,
				EnabledFlag:     true,
				BuiltinFlag:     true,
				AssetHashText:   assetHash,
				SortOrder:       builtin.SortOrder,
			}
			if err := s.toolRepo.Create(ctx, do); err != nil {
				return err
			}
			continue
		}
		if existing.CustomizedFlag || existing.AssetHashText == assetHash {
			continue
		}
		if err := s.toolRepo.Update(ctx, existing.ToolULID, map[string]any{
			"tool_name":         builtin.ToolName,
			"description_text":  builtin.DescriptionText,
			"trigger_keywords":  builtin.TriggerKeywords,
			"param_schema_json": paramJSON,
			"sort_order":        builtin.SortOrder,
			"asset_hash_text":   assetHash,
		}); err != nil {
			return err
		}
	}
	return nil
}

// List 查询工具列表。
func (s *ToolService) List(ctx context.Context, req *domain.ToolListREQ) ([]domain.ToolVO, error) {
	dos, err := s.toolRepo.List(ctx, req)
	if err != nil {
		return nil, err
	}
	vos := make([]domain.ToolVO, 0, len(dos))
	for i := range dos {
		vos = append(vos, toToolVO(&dos[i]))
	}
	return vos, nil
}

// Save 创建或更新工具（编码唯一；内置工具不允许改编码与类型）。
func (s *ToolService) Save(ctx context.Context, req *domain.ToolREQ) (domain.ToolVO, error) {
	existingByCode, err := s.toolRepo.GetByCode(ctx, req.ToolCode)
	if err != nil {
		return domain.ToolVO{}, err
	}
	if existingByCode != nil && existingByCode.ToolULID != req.ToolULID {
		return domain.ToolVO{}, pkg.NewBizError(domain.RespCodeConflict, "工具编码已存在")
	}
	enabledFlag := true
	if req.EnabledFlag != nil {
		enabledFlag = *req.EnabledFlag
	}
	paramJSON := marshalToolParams(req.ParamList)
	if req.ToolULID == "" {
		do := &domain.ToolDO{
			ToolULID:        pkg.NewULID(domain.IDPrefixTool),
			ToolCode:        req.ToolCode,
			ToolName:        req.ToolName,
			ToolTypeCode:    domain.ToolTypeBuiltin,
			DescriptionText: req.DescriptionText,
			TriggerKeywords: req.TriggerKeywords,
			ParamSchemaJSON: paramJSON,
			EnabledFlag:     enabledFlag,
			SortOrder:       req.SortOrder,
		}
		if err := s.toolRepo.Create(ctx, do); err != nil {
			return domain.ToolVO{}, err
		}
		return toToolVO(do), nil
	}
	current, err := s.toolRepo.GetByULID(ctx, req.ToolULID)
	if err != nil {
		return domain.ToolVO{}, err
	}
	if current == nil {
		return domain.ToolVO{}, pkg.NewBizError(domain.RespCodeNotFound, "工具不存在")
	}
	if current.BuiltinFlag && current.ToolCode != req.ToolCode {
		return domain.ToolVO{}, pkg.NewBizError(domain.RespCodeBizFailed, "内置工具不允许修改编码")
	}
	fields := map[string]any{
		"tool_code":         req.ToolCode,
		"tool_name":         req.ToolName,
		"description_text":  req.DescriptionText,
		"trigger_keywords":  req.TriggerKeywords,
		"param_schema_json": paramJSON,
		"enabled_flag":      enabledFlag,
		"sort_order":        req.SortOrder,
	}
	if current.BuiltinFlag {
		customized := req.ToolName != current.ToolName || req.DescriptionText != current.DescriptionText ||
			req.TriggerKeywords != current.TriggerKeywords || paramJSON != current.ParamSchemaJSON
		fields["customized_flag"] = customized
	}
	if err := s.toolRepo.Update(ctx, req.ToolULID, fields); err != nil {
		return domain.ToolVO{}, err
	}
	current.ToolCode = req.ToolCode
	current.ToolName = req.ToolName
	current.DescriptionText = req.DescriptionText
	current.TriggerKeywords = req.TriggerKeywords
	current.ParamSchemaJSON = paramJSON
	current.EnabledFlag = enabledFlag
	current.SortOrder = req.SortOrder
	if customized, ok := fields["customized_flag"].(bool); ok {
		current.CustomizedFlag = customized
	}
	return toToolVO(current), nil
}

// Delete 删除工具（内置不可删）。
func (s *ToolService) Delete(ctx context.Context, req *domain.ToolDeleteREQ) error {
	current, err := s.toolRepo.GetByULID(ctx, req.ToolULID)
	if err != nil {
		return err
	}
	if current == nil {
		return pkg.NewBizError(domain.RespCodeNotFound, "工具不存在")
	}
	if current.BuiltinFlag {
		return pkg.NewBizError(domain.RespCodeBizFailed, "内置工具不允许删除，可停用")
	}
	return s.toolRepo.Delete(ctx, req.ToolULID)
}

// Invoke 执行工具（管理端试运行与对话期调用共用入口）。
func (s *ToolService) Invoke(ctx context.Context, userULID string, req *domain.ToolInvokeREQ) (domain.ToolInvokeDTO, error) {
	toolDO, err := s.toolRepo.GetByCode(ctx, req.ToolCode)
	if err != nil {
		return domain.ToolInvokeDTO{}, err
	}
	if toolDO == nil {
		return domain.ToolInvokeDTO{}, pkg.NewBizError(domain.RespCodeNotFound, "工具不存在: "+req.ToolCode)
	}
	if !toolDO.EnabledFlag {
		return domain.ToolInvokeDTO{}, pkg.NewBizError(domain.RespCodeBizFailed, "工具已停用: "+req.ToolCode)
	}
	startedAt := time.Now()
	dto, err := s.execute(ctx, userULID, toolDO, req)
	if err != nil {
		return domain.ToolInvokeDTO{}, err
	}
	dto.ToolCode = toolDO.ToolCode
	dto.ToolName = toolDO.ToolName
	dto.LatencyMs = time.Since(startedAt).Milliseconds()
	dto.ResultText = pkg.TruncateText(dto.ResultText, toolResultMaxRunes)
	return dto, nil
}

// ResolveTools 解析本轮要执行的工具：显式编码（请求体 + 正文 #code）优先，关键词自动触发补位。
// 显式编码不存在或已停用返回 missingList，由调用方决定报错还是忽略。
func (s *ToolService) ResolveTools(ctx context.Context, explicitCodeList []string,
	contentText string) ([]domain.ToolVO, []string, error) {
	dos, err := s.toolRepo.List(ctx, &domain.ToolListREQ{EnabledOnly: false})
	if err != nil {
		return nil, nil, err
	}
	parsedList, _ := parseHashToolCodes(contentText)
	codeList := mergeToolCodes(explicitCodeList, parsedList)
	explicitList, missingList := selectToolsByCode(dos, codeList)
	excludeSet := make(map[string]bool, len(explicitList))
	for i := range explicitList {
		excludeSet[explicitList[i].ToolCode] = true
	}
	remainLimit := toolTotalLimit - len(explicitList)
	autoList := matchToolsByKeyword(dos, contentText, excludeSet, remainLimit)
	return append(explicitList, autoList...), missingList, nil
}

// execute 分发到具体工具实现。
func (s *ToolService) execute(ctx context.Context, userULID string, toolDO *domain.ToolDO,
	req *domain.ToolInvokeREQ) (domain.ToolInvokeDTO, error) {
	contentText, err := s.resolveContent(ctx, userULID, req)
	if err != nil {
		return domain.ToolInvokeDTO{}, err
	}
	switch toolDO.ToolCode {
	case "chapter_check":
		return s.runChapterCheck(contentText), nil
	case "deai_scan":
		return s.runDeAIScan(contentText), nil
	case "chapter_digest":
		return s.runChapterDigest(contentText, req.ParamValue), nil
	case "character_scan":
		return s.runCharacterScan(ctx, userULID, req.ProjectULID, contentText), nil
	case "foreshadow_scan":
		return s.runForeshadowScan(ctx, userULID, req.ProjectULID, contentText), nil
	case "lore_lookup":
		return s.runLoreLookup(ctx, userULID, req.ProjectULID, req.ParamValue), nil
	default:
		return domain.ToolInvokeDTO{}, pkg.NewBizError(domain.RespCodeBizFailed, "工具没有服务端实现: "+toolDO.ToolCode)
	}
}

// resolveContent 取待检正文：显式正文 > 指定章节 > 空。
func (s *ToolService) resolveContent(ctx context.Context, userULID string, req *domain.ToolInvokeREQ) (string, error) {
	if strings.TrimSpace(req.ContentText) != "" {
		return req.ContentText, nil
	}
	if req.ChapterULID == "" {
		return "", pkg.NewBizError(domain.RespCodeInvalidParam, "请提供正文或指定章节")
	}
	chapter, err := s.chapterRepo.GetByULID(ctx, req.ChapterULID)
	if err != nil {
		return "", err
	}
	if chapter == nil {
		return "", pkg.NewBizError(domain.RespCodeNotFound, "章节不存在")
	}
	if _, err := mustOwnProject(ctx, s.projectRepo, userULID, chapter.ProjectULID); err != nil {
		return "", err
	}
	return chapter.ContentText, nil
}

// runChapterCheck 章节量化体检。
func (s *ToolService) runChapterCheck(contentText string) domain.ToolInvokeDTO {
	report := check.Analyze(contentText)
	var builder strings.Builder
	builder.WriteString("【章节体检】\n")
	builder.WriteString("字数 " + intToText(report.WordCount) + "，段落 " + intToText(int64(report.ParagraphCount)) +
		"，对话段占比 " + percentText(report.DialogueRatio) + "，综合分 " + intToText(int64(report.ScoreValue)) +
		"，结论 " + verdictLabel(report.VerdictCode) + "\n")
	if len(report.IssueList) == 0 {
		builder.WriteString("未发现门禁问题，可以发布。\n")
	}
	for _, issue := range report.IssueList {
		builder.WriteString("- [" + levelLabel(issue.LevelCode) + "] " + issue.TitleText)
		if issue.HitCount > 0 {
			builder.WriteString("（" + intToText(int64(issue.HitCount)) + " 处）")
		}
		builder.WriteString("：" + issue.DetailText + "\n")
	}
	metricMap := map[string]string{
		"word_count":   intToText(report.WordCount),
		"score":        intToText(int64(report.ScoreValue)),
		"verdict":      report.VerdictCode,
		"issue_count":  intToText(int64(len(report.IssueList))),
		"dash_count":   intToText(int64(report.DashCount)),
		"hook_present": boolText(report.HookPresent),
	}
	return domain.ToolInvokeDTO{ResultText: builder.String(), MetricMap: metricMap}
}

// runDeAIScan 去 AI 味扫描（复用体检的词面指标 + 结构指标）。
func (s *ToolService) runDeAIScan(contentText string) domain.ToolInvokeDTO {
	report := check.Analyze(contentText)
	var builder strings.Builder
	builder.WriteString("【去 AI 味扫描】\n")
	builder.WriteString("零容忍词 " + intToText(int64(report.BannedWordCount)) + " 处，警示词 " +
		intToText(int64(report.WarnWordCount)) + " 处，情绪命名 " + intToText(int64(report.EmotionWordCount)) +
		" 处，裸对话标签 " + intToText(int64(report.BareTagCount)) + " 处。\n")
	hit := false
	for _, issue := range report.IssueList {
		if issue.IssueCode == "banned_word" || issue.IssueCode == "warn_word" ||
			issue.IssueCode == "emotion_named" || issue.IssueCode == "bare_tag" ||
			issue.IssueCode == "paragraph_uniform" {
			hit = true
			builder.WriteString("- " + issue.TitleText + "：" + issue.DetailText + "\n")
		}
	}
	if !hit {
		builder.WriteString("未命中词面与结构硬指标。仍需人工确认：对白是否推动主线、情绪是否靠动作外化、章末是否留钩。\n")
	}
	metricMap := map[string]string{
		"banned_word_count":  intToText(int64(report.BannedWordCount)),
		"warn_word_count":    intToText(int64(report.WarnWordCount)),
		"emotion_word_count": intToText(int64(report.EmotionWordCount)),
		"bare_tag_count":     intToText(int64(report.BareTagCount)),
	}
	return domain.ToolInvokeDTO{ResultText: builder.String(), MetricMap: metricMap}
}

// runChapterDigest 抽取式摘要：每段首句 + 章末钩子。
func (s *ToolService) runChapterDigest(contentText string, paramValue map[string]string) domain.ToolInvokeDTO {
	maxSentences := toolDigestDefaultSentences
	if value := strings.TrimSpace(paramValue["max_sentences"]); value != "" {
		if parsed, ok := parseIntText(value); ok && parsed > 0 {
			maxSentences = parsed
		}
	}
	if maxSentences > toolDigestMaxSentences {
		maxSentences = toolDigestMaxSentences
	}
	paragraphList := splitNonEmptyLines(contentText)
	sentenceList := make([]string, 0, len(paragraphList))
	for _, paragraph := range paragraphList {
		firstSentence := firstSentenceOf(paragraph)
		if firstSentence == "" {
			continue
		}
		sentenceList = append(sentenceList, firstSentence)
		if len(sentenceList) >= maxSentences {
			break
		}
	}
	var builder strings.Builder
	builder.WriteString("【章节摘要（抽取式）】\n")
	if len(sentenceList) == 0 {
		builder.WriteString("正文为空，无法生成摘要。\n")
	}
	for index, sentence := range sentenceList {
		builder.WriteString(intToText(int64(index+1)) + ". " + sentence + "\n")
	}
	if tail := lastSentenceOf(contentText); tail != "" {
		builder.WriteString("章末钩子：" + tail + "\n")
	}
	metricMap := map[string]string{"sentence_count": intToText(int64(len(sentenceList)))}
	return domain.ToolInvokeDTO{ResultText: builder.String(), MetricMap: metricMap}
}

// runCharacterScan 统计项目角色在本章的出场次数与首次出现位置。
func (s *ToolService) runCharacterScan(ctx context.Context, userULID, projectULID, contentText string) domain.ToolInvokeDTO {
	if projectULID == "" {
		return domain.ToolInvokeDTO{ResultText: "【角色出场统计】\n需要指定项目（project_ulid）。\n"}
	}
	if _, err := mustOwnProject(ctx, s.projectRepo, userULID, projectULID); err != nil {
		return domain.ToolInvokeDTO{ResultText: "【角色出场统计】\n项目不存在或无权访问。\n"}
	}
	characterList, err := s.characterRepo.ListByProject(ctx, projectULID, "", "")
	if err != nil {
		return domain.ToolInvokeDTO{ResultText: "【角色出场统计】\n角色读取失败：" + err.Error() + "\n"}
	}
	type characterHit struct {
		name     string
		roleCode string
		count    int
		firstPos int
	}
	hitList := make([]characterHit, 0, len(characterList))
	for i := range characterList {
		name := characterList[i].CharacterName
		count := strings.Count(contentText, name)
		if count == 0 && characterList[i].AliasText != "" {
			for _, alias := range strings.Split(characterList[i].AliasText, ",") {
				if trimmed := strings.TrimSpace(alias); trimmed != "" {
					count += strings.Count(contentText, trimmed)
				}
			}
		}
		if count == 0 {
			continue
		}
		hitList = append(hitList, characterHit{name: name, roleCode: characterList[i].RoleTypeCode,
			count: count, firstPos: strings.Index(contentText, name)})
	}
	sort.Slice(hitList, func(i, j int) bool {
		if hitList[i].count != hitList[j].count {
			return hitList[i].count > hitList[j].count
		}
		return hitList[i].firstPos < hitList[j].firstPos
	})
	var builder strings.Builder
	builder.WriteString("【角色出场统计】\n")
	if len(hitList) == 0 {
		builder.WriteString("本章没有项目角色卡中的名字出现；确认是否用了化名，或这一章换了视角。\n")
	}
	for _, hit := range hitList {
		builder.WriteString("- " + hit.name + "（" + hit.roleCode + "）：" + intToText(int64(hit.count)) + " 次\n")
	}
	absentProtagonist := make([]string, 0, 2)
	for i := range characterList {
		if characterList[i].RoleTypeCode != "protagonist" {
			continue
		}
		appeared := false
		for _, hit := range hitList {
			if hit.name == characterList[i].CharacterName {
				appeared = true
				break
			}
		}
		if !appeared {
			absentProtagonist = append(absentProtagonist, characterList[i].CharacterName)
		}
	}
	if len(absentProtagonist) > 0 {
		builder.WriteString("主角未出场：" + strings.Join(absentProtagonist, "、") +
			"（连续缺席会让读者断线，若非刻意请补一处提及）。\n")
	}
	return domain.ToolInvokeDTO{ResultText: builder.String(),
		MetricMap: map[string]string{"appeared_count": intToText(int64(len(hitList)))}}
}

// runForeshadowScan 检查未回收伏笔与本章关联度。
func (s *ToolService) runForeshadowScan(ctx context.Context, userULID, projectULID, contentText string) domain.ToolInvokeDTO {
	if projectULID == "" {
		return domain.ToolInvokeDTO{ResultText: "【伏笔回收检查】\n需要指定项目（project_ulid）。\n"}
	}
	if _, err := mustOwnProject(ctx, s.projectRepo, userULID, projectULID); err != nil {
		return domain.ToolInvokeDTO{ResultText: "【伏笔回收检查】\n项目不存在或无权访问。\n"}
	}
	hookList, err := s.hookRepo.ListHooksByProject(ctx, projectULID, "")
	if err != nil {
		return domain.ToolInvokeDTO{ResultText: "【伏笔回收检查】\n伏笔读取失败：" + err.Error() + "\n"}
	}
	openList := make([]domain.ForeshadowHookDO, 0, len(hookList))
	for i := range hookList {
		if hookList[i].HookStatusCode == "open" || hookList[i].HookStatusCode == "planted" || hookList[i].HookStatusCode == "" {
			openList = append(openList, hookList[i])
		}
	}
	var builder strings.Builder
	builder.WriteString("【伏笔回收检查】\n未回收 " + intToText(int64(len(openList))) + " 条。\n")
	relatedCount := 0
	for i := range openList {
		hook := &openList[i]
		related := hookRelatedToContent(hook, contentText)
		if related {
			relatedCount++
		}
		ageDays := int(time.Since(hook.CreatedAt).Hours() / 24)
		line := "- " + pkg.TruncateText(hook.DescriptionText, 60)
		if hook.ImportanceCode != "" {
			line += "（" + hook.ImportanceCode + "）"
		}
		if related {
			line += " ← 本章出现相关词，适合回收"
		}
		if ageDays >= toolHookStaleDays {
			line += "｜已放置 " + intToText(int64(ageDays)) + " 天"
		}
		builder.WriteString(line + "\n")
	}
	if len(openList) == 0 {
		builder.WriteString("没有未回收伏笔，当前账是平的。\n")
	} else if relatedCount == 0 {
		builder.WriteString("本章未出现任何伏笔相关词：如果这一章本来该收，请回看伏笔登记。\n")
	}
	return domain.ToolInvokeDTO{ResultText: builder.String(),
		MetricMap: map[string]string{"open_count": intToText(int64(len(openList))),
			"related_count": intToText(int64(relatedCount))}}
}

// runLoreLookup 按关键词检索设定与知识库片段。
func (s *ToolService) runLoreLookup(ctx context.Context, userULID, projectULID string, paramValue map[string]string) domain.ToolInvokeDTO {
	queryText := strings.TrimSpace(paramValue["query_text"])
	if queryText == "" {
		return domain.ToolInvokeDTO{ResultText: "【设定检索】\n缺少检索词（query_text）。\n"}
	}
	if projectULID == "" {
		return domain.ToolInvokeDTO{ResultText: "【设定检索】\n需要指定项目（project_ulid）。\n"}
	}
	if _, err := mustOwnProject(ctx, s.projectRepo, userULID, projectULID); err != nil {
		return domain.ToolInvokeDTO{ResultText: "【设定检索】\n项目不存在或无权访问。\n"}
	}
	loreList, err := s.loreRepo.ListByProject(ctx, projectULID, "", queryText)
	if err != nil {
		return domain.ToolInvokeDTO{ResultText: "【设定检索】\n设定读取失败：" + err.Error() + "\n"}
	}
	var builder strings.Builder
	builder.WriteString("【设定检索】关键词：" + queryText + "\n")
	if len(loreList) == 0 {
		builder.WriteString("项目设定里没有命中。回答时请明确说明“设定里没有这条”，不要自行编造。\n")
		return domain.ToolInvokeDTO{ResultText: builder.String(), MetricMap: map[string]string{"hit_count": "0"}}
	}
	limit := len(loreList)
	if limit > toolLoreLookupLimit {
		limit = toolLoreLookupLimit
	}
	for i := range limit {
		builder.WriteString("- " + loreList[i].LoreName + "（" + loreList[i].LoreTypeCode + "）：" +
			pkg.TruncateText(stripHTMLTags(loreList[i].ContentText), 300) + "\n")
	}
	return domain.ToolInvokeDTO{ResultText: builder.String(),
		MetricMap: map[string]string{"hit_count": intToText(int64(len(loreList)))}}
}

// toToolVO 组装工具视图对象。
func toToolVO(do *domain.ToolDO) domain.ToolVO {
	return domain.ToolVO{
		ToolULID:        do.ToolULID,
		ToolCode:        do.ToolCode,
		ToolName:        do.ToolName,
		ToolTypeCode:    do.ToolTypeCode,
		DescriptionText: do.DescriptionText,
		TriggerKeywords: do.TriggerKeywords,
		ParamList:       unmarshalToolParams(do.ParamSchemaJSON),
		EnabledFlag:     do.EnabledFlag,
		BuiltinFlag:     do.BuiltinFlag,
		CustomizedFlag:  do.CustomizedFlag,
		SortOrder:       do.SortOrder,
		CreatedAt:       pkg.FormatTime(do.CreatedAt),
		UpdatedAt:       pkg.FormatTime(do.UpdatedAt),
	}
}

// marshalToolParams 序列化参数声明（空列表存 "[]"）。
func marshalToolParams(paramList []domain.ToolParamDO) string {
	if len(paramList) == 0 {
		return "[]"
	}
	data, err := json.Marshal(paramList)
	if err != nil {
		return "[]"
	}
	return string(data)
}

// unmarshalToolParams 反序列化参数声明（损坏数据按空列表处理）。
func unmarshalToolParams(paramJSON string) []domain.ToolParamDO {
	paramList := make([]domain.ToolParamDO, 0, 4)
	if paramJSON == "" {
		return paramList
	}
	if err := json.Unmarshal([]byte(paramJSON), &paramList); err != nil {
		return []domain.ToolParamDO{}
	}
	return paramList
}

// toToolParamList 内置参数声明转领域结构。
func toToolParamList(paramList []ai.ToolParam) []domain.ToolParamDO {
	result := make([]domain.ToolParamDO, 0, len(paramList))
	for _, param := range paramList {
		result = append(result, domain.ToolParamDO{
			ParamName: param.ParamName, ParamLabel: param.ParamLabel,
			RequiredFlag: param.RequiredFlag, DefaultValue: param.DefaultValue,
			DescriptionText: param.DescriptionText,
		})
	}
	return result
}

// hookRelatedToContent 判断伏笔描述里的关键词是否出现在正文（简易分词：中文双字片段）。
func hookRelatedToContent(hook *domain.ForeshadowHookDO, contentText string) bool {
	if strings.TrimSpace(contentText) == "" {
		return false
	}
	text := hook.DescriptionText + " " + hook.ExpectedPayoffText
	runes := []rune(text)
	for i := 0; i+1 < len(runes); i++ {
		pair := string(runes[i : i+2])
		if isHanPair(pair) && strings.Contains(contentText, pair) {
			return true
		}
	}
	return false
}

// isHanPair 判断两字符是否都是汉字（用于粗粒度关键词命中）。
func isHanPair(pair string) bool {
	runes := []rune(pair)
	if len(runes) != 2 {
		return false
	}
	for _, runeValue := range runes {
		if runeValue < 0x4E00 || runeValue > 0x9FFF {
			return false
		}
	}
	return true
}

// splitNonEmptyLines 按行切分并去掉空行。
func splitNonEmptyLines(text string) []string {
	rawList := strings.FieldsFunc(text, func(r rune) bool { return r == '\n' || r == '\r' })
	result := make([]string, 0, len(rawList))
	for _, item := range rawList {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// firstSentenceOf 取段落首句（到第一个句末标点为止）。
func firstSentenceOf(paragraph string) string {
	runes := []rune(strings.TrimSpace(paragraph))
	for index, runeValue := range runes {
		if strings.ContainsRune("。！？!?…", runeValue) && index >= 8 {
			return string(runes[:index+1])
		}
	}
	if len(runes) <= 60 {
		return string(runes)
	}
	return string(runes[:60]) + "…"
}

// lastSentenceOf 取末句（最后一个句末标点之后到结尾，过长则截断）。
func lastSentenceOf(text string) string {
	trimmed := strings.TrimSpace(text)
	runes := []rune(trimmed)
	start := len(runes) - 60
	if start < 0 {
		start = 0
	}
	return strings.TrimSpace(string(runes[start:]))
}

// stripHTMLTags 去掉富文本标签（设定正文可能是 HTML）。
func stripHTMLTags(text string) string {
	var builder strings.Builder
	inTag := false
	for _, runeValue := range text {
		switch {
		case runeValue == '<':
			inTag = true
		case runeValue == '>':
			inTag = false
			builder.WriteRune(' ')
		case !inTag:
			builder.WriteRune(runeValue)
		}
	}
	return strings.Join(strings.Fields(builder.String()), " ")
}

// intToText 整数转字符串。
func intToText(value int64) string {
	if value == 0 {
		return "0"
	}
	negative := value < 0
	if negative {
		value = -value
	}
	digits := make([]byte, 0, 12)
	for value > 0 {
		digits = append([]byte{byte('0' + value%10)}, digits...)
		value /= 10
	}
	if negative {
		return "-" + string(digits)
	}
	return string(digits)
}

// parseIntText 解析正整数文本。
func parseIntText(text string) (int, bool) {
	value := 0
	if text == "" {
		return 0, false
	}
	for _, runeValue := range text {
		if runeValue < '0' || runeValue > '9' {
			return 0, false
		}
		value = value*10 + int(runeValue-'0')
		if value > 100000 {
			return 0, false
		}
	}
	return value, true
}

// percentText 把 0-1 的比例格式化为百分比文本。
func percentText(ratio float64) string {
	return intToText(int64(ratio*100+0.5)) + "%"
}

// boolText 布尔转「是/否」。
func boolText(value bool) string {
	if value {
		return "是"
	}
	return "否"
}

// verdictLabel 体检结论中文标签。
func verdictLabel(verdictCode string) string {
	switch verdictCode {
	case check.VerdictPass:
		return "可发"
	case check.VerdictRevise:
		return "需修"
	default:
		return "不可发"
	}
}

// levelLabel 问题级别中文标签。
func levelLabel(levelCode string) string {
	switch levelCode {
	case "block":
		return "阻断"
	case "warn":
		return "警示"
	default:
		return "建议"
	}
}
