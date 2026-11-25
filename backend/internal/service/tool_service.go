// 本文件实现工具资产的业务：内置工具播种、CRUD、试运行与下发给模型的工具声明。
// 各工具的确定性执行器在 tool_executor.go。

package service

import (
	"context"
	"strings"
	"time"

	"github.com/2132691342/NovelWorkbench/backend/internal/ai"
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
		"param_schema_json": paramJSON,
		"enabled_flag":      enabledFlag,
		"sort_order":        req.SortOrder,
	}
	if current.BuiltinFlag {
		customized := req.ToolName != current.ToolName || req.DescriptionText != current.DescriptionText ||
			paramJSON != current.ParamSchemaJSON
		fields["customized_flag"] = customized
	}
	if err := s.toolRepo.Update(ctx, req.ToolULID, fields); err != nil {
		return domain.ToolVO{}, err
	}
	current.ToolCode = req.ToolCode
	current.ToolName = req.ToolName
	current.DescriptionText = req.DescriptionText
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

// ToolDeclarations 返回下发给模型的工具声明（仅启用的工具）。
// 工具是否真的被调用完全由模型决定：这里只负责把「有哪几件工具、各自能吃哪些参数」讲清楚。
func (s *ToolService) ToolDeclarations(ctx context.Context) ([]ai.ToolSpec, error) {
	dos, err := s.toolRepo.List(ctx, &domain.ToolListREQ{EnabledOnly: true})
	if err != nil {
		return nil, err
	}
	specList := make([]ai.ToolSpec, 0, len(dos))
	for i := range dos {
		specList = append(specList, ai.ToolSpec{
			ToolCode:        dos[i].ToolCode,
			ToolName:        dos[i].ToolName,
			DescriptionText: dos[i].DescriptionText,
			ParamList:       toAIToolParams(unmarshalToolParams(dos[i].ParamSchemaJSON)),
		})
	}
	return specList, nil
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
