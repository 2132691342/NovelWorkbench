// 本文件实现 AI 开书业务：生成开书方案（预览）与按方案落库（项目 + 卷章 + 角色 + 设定 + 大纲）。

package service

import (
	"context"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"gorm.io/gorm"

	"github.com/2132691342/NovelWorkbench/backend/internal/ai"
	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
	"github.com/2132691342/NovelWorkbench/backend/internal/repo"
)

// 开书方案的规模约束：模型输出不可信，落库前一律收敛到区间内。
const (
	bootstrapMinChapterCount     = 3                 // 缩规模重试的下限
	bootstrapDefaultChapterCount = 12                // 未指定时的默认章节数
	bootstrapMaxVolumeCount      = 3                 // 卷数上限
	bootstrapMaxCharacterCount   = 8                 // 角色数上限
	bootstrapMaxLoreCount        = 10                // 设定数上限
	bootstrapNameMaxLen          = 128               // 名称类字段上限（与列宽一致）
	bootstrapSummaryMaxLen       = 2000              // 梗概/简介上限
	bootstrapPlanTimeout         = 300 * time.Second // 方案生成超时：结构化长输出，比单轮对话慢得多
	bootstrapPlanMaxTokens       = 8000              // 方案输出上限：卷章 + 角色 + 设定一次性出全
)

// BootstrapService AI 开书业务：生成方案与落库。
type BootstrapService struct {
	providerSvc   *ModelProviderService // 模型选择与授权校验
	aiClient      *ai.Client            // 模型网关
	projectRepo   *repo.ProjectRepo     // 项目仓储
	volumeRepo    *repo.VolumeRepo      // 卷仓储
	chapterRepo   *repo.ChapterRepo     // 章节仓储
	charRepo      *repo.CharacterRepo   // 角色仓储
	loreRepo      *repo.LoreRepo        // 设定仓储
	outlineRepo   *repo.OutlineRepo     // 大纲仓储
	db            *gorm.DB              // 事务入口
	auditRecorder pkg.AuditRecorder     // 审计写入（AI 开书方案留痕，可为 nil）
}

// NewBootstrapService 构造开书服务。
func NewBootstrapService(providerSvc *ModelProviderService, aiClient *ai.Client, projectRepo *repo.ProjectRepo,
	volumeRepo *repo.VolumeRepo, chapterRepo *repo.ChapterRepo, charRepo *repo.CharacterRepo,
	loreRepo *repo.LoreRepo, outlineRepo *repo.OutlineRepo, db *gorm.DB) *BootstrapService {
	return &BootstrapService{
		providerSvc: providerSvc, aiClient: aiClient,
		projectRepo: projectRepo, volumeRepo: volumeRepo, chapterRepo: chapterRepo,
		charRepo: charRepo, loreRepo: loreRepo, outlineRepo: outlineRepo, db: db,
	}
}

// SetAuditRecorder 注入审计写入器（开书方案生成留痕，链路 ID 取请求的 X-Request-ID）。
func (s *BootstrapService) SetAuditRecorder(recorder pkg.AuditRecorder) {
	s.auditRecorder = recorder
}

// GeneratePlan 用一句创意生成开书方案（不落库，供作者预览与修改）。
func (s *BootstrapService) GeneratePlan(ctx context.Context, userULID string, req *domain.ProjectBootstrapREQ) (domain.ProjectBootstrapPlanVO, error) {
	spec, err := s.providerSvc.ChatSpecOf(ctx, userULID, req.ModelULID)
	if err != nil {
		return domain.ProjectBootstrapPlanVO{}, err
	}
	chapterCount := req.TargetChapterCount
	if chapterCount <= 0 {
		chapterCount = bootstrapDefaultChapterCount
	}
	// 开书是长结构化输出：首包与总时长都比单轮对话长，且 token 上限要够放完整方案。
	if spec.Timeout < bootstrapPlanTimeout {
		spec.Timeout = bootstrapPlanTimeout
	}
	if spec.MaxTokens < bootstrapPlanMaxTokens {
		spec.MaxTokens = bootstrapPlanMaxTokens
	}
	planCtx, cancel := context.WithTimeout(ctx, bootstrapPlanTimeout)
	defer cancel()
	planStartedAt := time.Now()
	plan, err := s.aiClient.GenerateBootstrapPlan(planCtx, spec, req.IdeaText, req.GenreCode, chapterCount)
	if err != nil && chapterCount > bootstrapMinChapterCount {
		// 结构化长输出可能撞到模型输出上限被截断：缩半规模重试一次，宁可章少也不给半份方案。
		retryCount := chapterCount / 2
		if retryCount < bootstrapMinChapterCount {
			retryCount = bootstrapMinChapterCount
		}
		pkg.SLogger().Warnw("开书方案生成失败，缩规模重试", "chapters", chapterCount, "retry_chapters", retryCount, "error", err)
		plan, err = s.aiClient.GenerateBootstrapPlan(planCtx, spec, req.IdeaText, req.GenreCode, retryCount)
		chapterCount = retryCount
	}
	// AI 调用留痕：开书方案是链路里最贵的一步，模型、token 与耗时都要可查（重试合并为一条）
	recordAICall(ctx, s.auditRecorder, aiCallMeta{
		ActionCode:  domain.AuditActionAIBootstrapPlan,
		ActionName:  "AI 开书方案",
		TraceID:     pkg.RequestIDFromContext(ctx),
		UserULID:    userULID,
		ModelName:   spec.ChatModelName,
		PromptChars: utf8.RuneCountInString(req.IdeaText),
	}, time.Since(planStartedAt), estimateTokenCount(utf8.RuneCountInString(req.IdeaText), 0), err)
	if err != nil {
		return domain.ProjectBootstrapPlanVO{}, pkg.WrapBizError(domain.RespCodeAIProviderError, "开书方案生成失败", err)
	}
	return normalizeBootstrapPlan(plan, req.GenreCode, chapterCount), nil
}

// ApplyPlan 按方案落库：项目、卷章、角色、设定与大纲节点在同一事务内创建。
func (s *BootstrapService) ApplyPlan(ctx context.Context, userULID string, req *domain.ProjectBootstrapApplyREQ) (domain.ProjectBootstrapResultVO, error) {
	plan := req.Plan
	if strings.TrimSpace(plan.ProjectName) == "" {
		return domain.ProjectBootstrapResultVO{}, pkg.NewBizError(domain.RespCodeInvalidParam, "方案缺少书名")
	}
	if len(plan.VolumeList) == 0 {
		return domain.ProjectBootstrapResultVO{}, pkg.NewBizError(domain.RespCodeInvalidParam, "方案缺少卷章结构")
	}
	projectULID := pkg.NewULID(domain.IDPrefixProject)
	result := domain.ProjectBootstrapResultVO{}
	err := repo.WithTx(s.db, func(tx *gorm.DB) error {
		project := &domain.ProjectDO{
			ProjectULID:   projectULID,
			UserULID:      userULID,
			ProjectName:   pkg.TruncateText(plan.ProjectName, bootstrapNameMaxLen),
			GenreCode:     pkg.TruncateText(plan.GenreCode, 32),
			StyleText:     plan.StyleText,
			SynopsisText:  buildSynopsis(plan),
			ProjectStatus: "draft",
		}
		if err := repo.NewProjectRepo(tx).Create(ctx, project); err != nil {
			return err
		}
		if err := createBootstrapVolumes(ctx, tx, projectULID, plan, &result); err != nil {
			return err
		}
		if err := createBootstrapCharacters(ctx, tx, projectULID, plan, &result); err != nil {
			return err
		}
		if err := createBootstrapLore(ctx, tx, projectULID, plan, &result); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return domain.ProjectBootstrapResultVO{}, err
	}
	project, err := s.projectRepo.GetByULID(ctx, projectULID)
	if err != nil || project == nil {
		return domain.ProjectBootstrapResultVO{}, pkg.WrapBizError(domain.RespCodeStorageError, "项目读取失败", err)
	}
	result.ProjectVO = toProjectVO(project, result.VolumeCount, result.ChapterCount)
	return result, nil
}

// createBootstrapVolumes 创建卷、章节与对应大纲节点（大纲按卷 → 章两级挂载）。
func createBootstrapVolumes(ctx context.Context, tx *gorm.DB, projectULID string,
	plan domain.ProjectBootstrapPlanVO, result *domain.ProjectBootstrapResultVO) error {
	volumeRepo := repo.NewVolumeRepo(tx)
	chapterRepo := repo.NewChapterRepo(tx)
	outlineRepo := repo.NewOutlineRepo(tx)
	chapterOrder := 0
	for volumeIndex, volumePlan := range plan.VolumeList {
		volumeDO := &domain.VolumeDO{
			VolumeULID:  pkg.NewULID(domain.IDPrefixVolume),
			ProjectULID: projectULID,
			VolumeName:  pkg.TruncateText(defaultText(volumePlan.VolumeName, "第一卷"), bootstrapNameMaxLen),
			VolumeOrder: volumeIndex + 1,
			SummaryText: pkg.TruncateText(volumePlan.SummaryText, bootstrapSummaryMaxLen),
		}
		if err := volumeRepo.Create(ctx, volumeDO); err != nil {
			return err
		}
		result.VolumeCount++
		outlineVolume := &domain.OutlineNodeDO{
			OutlineULID:    pkg.NewULID(domain.IDPrefixOutline),
			ProjectULID:    projectULID,
			NodeTypeCode:   "volume",
			TitleText:      volumeDO.VolumeName,
			SummaryText:    volumeDO.SummaryText,
			NodeStatusCode: "planning",
			SortOrder:      volumeIndex + 1,
		}
		if err := outlineRepo.CreateNode(ctx, outlineVolume); err != nil {
			return err
		}
		for chapterIndex, chapterPlan := range volumePlan.ChapterList {
			chapterOrder++
			chapterDO := &domain.ChapterDO{
				ChapterULID:   pkg.NewULID(domain.IDPrefixChapter),
				ProjectULID:   projectULID,
				VolumeULID:    volumeDO.VolumeULID,
				ChapterName:   pkg.TruncateText(defaultText(chapterPlan.ChapterName, "第"+strconv.Itoa(chapterOrder)+"章"), bootstrapNameMaxLen),
				ChapterOrder:  chapterOrder,
				SummaryText:   pkg.TruncateText(chapterPlan.SummaryText, bootstrapSummaryMaxLen),
				ChapterStatus: "draft",
			}
			if err := chapterRepo.Create(ctx, chapterDO); err != nil {
				return err
			}
			result.ChapterCount++
			outlineChapter := &domain.OutlineNodeDO{
				OutlineULID:       pkg.NewULID(domain.IDPrefixOutline),
				ProjectULID:       projectULID,
				ParentOutlineULID: outlineVolume.OutlineULID,
				NodeTypeCode:      "chapter",
				TitleText:         chapterDO.ChapterName,
				SummaryText:       chapterDO.SummaryText,
				NodeStatusCode:    "draft",
				SortOrder:         chapterIndex + 1,
			}
			if err := outlineRepo.CreateNode(ctx, outlineChapter); err != nil {
				return err
			}
		}
	}
	return nil
}

// createBootstrapCharacters 创建角色卡（角色类型收敛到枚举，未知值按配角处理）。
func createBootstrapCharacters(ctx context.Context, tx *gorm.DB, projectULID string,
	plan domain.ProjectBootstrapPlanVO, result *domain.ProjectBootstrapResultVO) error {
	charRepo := repo.NewCharacterRepo(tx)
	for _, item := range plan.CharacterList {
		if strings.TrimSpace(item.CharacterName) == "" {
			continue
		}
		do := &domain.CharacterDO{
			CharacterULID:   pkg.NewULID(domain.IDPrefixCharacter),
			ProjectULID:     projectULID,
			CharacterName:   pkg.TruncateText(item.CharacterName, 64),
			RoleTypeCode:    normalizeRoleType(item.RoleTypeCode),
			PersonalityText: item.PersonalityText,
			BackgroundText:  item.BackgroundText,
			GoalText:        item.GoalText,
		}
		if err := charRepo.Create(ctx, do); err != nil {
			return err
		}
		result.CharacterCount++
	}
	return nil
}

// createBootstrapLore 创建设定条目（类型收敛到枚举，未知值按规则卡处理）。
func createBootstrapLore(ctx context.Context, tx *gorm.DB, projectULID string,
	plan domain.ProjectBootstrapPlanVO, result *domain.ProjectBootstrapResultVO) error {
	loreRepo := repo.NewLoreRepo(tx)
	for index, item := range plan.LoreList {
		if strings.TrimSpace(item.LoreName) == "" {
			continue
		}
		do := &domain.LoreDO{
			LoreULID:        pkg.NewULID(domain.IDPrefixLore),
			ProjectULID:     projectULID,
			LoreName:        pkg.TruncateText(item.LoreName, bootstrapNameMaxLen),
			LoreTypeCode:    normalizeLoreType(item.LoreTypeCode),
			ContentText:     item.ContentText,
			ContentModeCode: domain.LoreContentModeMarkdown,
			SortOrder:       index + 1,
		}
		if err := loreRepo.Create(ctx, do); err != nil {
			return err
		}
		result.LoreCount++
	}
	return nil
}

// normalizeBootstrapPlan 收敛模型输出：名称补默认值、卷章数量截断、枚举值校正。
func normalizeBootstrapPlan(plan ai.BootstrapPlan, genreCode string, chapterCount int) domain.ProjectBootstrapPlanVO {
	vo := domain.ProjectBootstrapPlanVO{
		ProjectName:      pkg.TruncateText(strings.TrimSpace(plan.ProjectName), bootstrapNameMaxLen),
		GenreCode:        defaultText(strings.TrimSpace(plan.GenreCode), genreCode),
		SellingPointText: strings.TrimSpace(plan.SellingPointText),
		SynopsisText:     strings.TrimSpace(plan.SynopsisText),
		StyleText:        strings.TrimSpace(plan.StyleText),
		VolumeList:       make([]domain.BootstrapVolumeVO, 0, len(plan.VolumeList)),
		CharacterList:    make([]domain.BootstrapCharacterVO, 0, len(plan.CharacterList)),
		LoreList:         make([]domain.BootstrapLoreVO, 0, len(plan.LoreList)),
	}
	chapterOrder := 0
	for _, volume := range plan.VolumeList {
		if len(vo.VolumeList) >= bootstrapMaxVolumeCount {
			break
		}
		volumeVO := domain.BootstrapVolumeVO{
			VolumeName:  pkg.TruncateText(defaultText(strings.TrimSpace(volume.VolumeName), "第"+strconv.Itoa(len(vo.VolumeList)+1)+"卷"), bootstrapNameMaxLen),
			SummaryText: pkg.TruncateText(volume.SummaryText, bootstrapSummaryMaxLen),
			VolumeOrder: len(vo.VolumeList) + 1,
			ChapterList: make([]domain.BootstrapChapterVO, 0, len(volume.ChapterList)),
		}
		for _, chapter := range volume.ChapterList {
			if chapterOrder >= chapterCount {
				break
			}
			chapterOrder++
			volumeVO.ChapterList = append(volumeVO.ChapterList, domain.BootstrapChapterVO{
				ChapterName:  pkg.TruncateText(defaultText(strings.TrimSpace(chapter.ChapterName), "第"+strconv.Itoa(chapterOrder)+"章"), bootstrapNameMaxLen),
				SummaryText:  pkg.TruncateText(chapter.SummaryText, bootstrapSummaryMaxLen),
				ChapterOrder: chapterOrder,
			})
		}
		if len(volumeVO.ChapterList) == 0 && chapterOrder >= chapterCount {
			break
		}
		vo.VolumeList = append(vo.VolumeList, volumeVO)
	}
	for _, character := range plan.CharacterList {
		if len(vo.CharacterList) >= bootstrapMaxCharacterCount || strings.TrimSpace(character.CharacterName) == "" {
			continue
		}
		vo.CharacterList = append(vo.CharacterList, domain.BootstrapCharacterVO{
			CharacterName:   pkg.TruncateText(character.CharacterName, 64),
			RoleTypeCode:    normalizeRoleType(character.RoleTypeCode),
			PersonalityText: character.PersonalityText,
			GoalText:        character.GoalText,
			BackgroundText:  character.BackgroundText,
		})
	}
	for _, lore := range plan.LoreList {
		if len(vo.LoreList) >= bootstrapMaxLoreCount || strings.TrimSpace(lore.LoreName) == "" {
			continue
		}
		vo.LoreList = append(vo.LoreList, domain.BootstrapLoreVO{
			LoreName:     pkg.TruncateText(lore.LoreName, bootstrapNameMaxLen),
			LoreTypeCode: normalizeLoreType(lore.LoreTypeCode),
			ContentText:  lore.ContentText,
		})
	}
	return vo
}

// buildSynopsis 简介优先用模型给的简介，缺失时回落卖点，保证列表页不出现空简介。
func buildSynopsis(plan domain.ProjectBootstrapPlanVO) string {
	if strings.TrimSpace(plan.SynopsisText) != "" {
		return pkg.TruncateText(plan.SynopsisText, bootstrapSummaryMaxLen)
	}
	return pkg.TruncateText(plan.SellingPointText, bootstrapSummaryMaxLen)
}

// normalizeRoleType 角色类型收敛：未知值按配角（supporting）处理。
func normalizeRoleType(roleTypeCode string) string {
	switch roleTypeCode {
	case "protagonist", "antagonist", "supporting", "minor":
		return roleTypeCode
	default:
		return "supporting"
	}
}

// normalizeLoreType 设定类型收敛：未知值按规则（rule）处理。
func normalizeLoreType(loreTypeCode string) string {
	switch loreTypeCode {
	case "world", "rule", "item", "place", "event", "note":
		return loreTypeCode
	default:
		return "rule"
	}
}

// defaultText 空值回落默认文案。
func defaultText(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
