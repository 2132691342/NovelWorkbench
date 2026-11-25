// 本文件实现工具的执行器：章节体检 / 去 AI 味 / 章节摘要 / 角色出场 / 伏笔回收 / 设定检索。
// 全部是确定性服务端逻辑，不额外消耗模型调用。

package service

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/2132691342/NovelWorkbench/backend/internal/ai/check"
	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
)

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
