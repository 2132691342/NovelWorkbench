// 本文件实现提示词业务：内置资产播种与指纹升级、作用域合并、CRUD 与对话期编码解析。

package service

import (
	"context"
	"strings"

	"github.com/2132691342/NovelWorkbench/backend/internal/ai"
	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
	"github.com/2132691342/NovelWorkbench/backend/internal/repo"
)

// PromptService 提示词业务：播种、CRUD、作用域合并解析。
type PromptService struct {
	promptRepo  *repo.PromptRepo  // 提示词仓储
	projectRepo *repo.ProjectRepo // 项目仓储（项目提示词归属校验）
}

// NewPromptService 构造提示词服务。
func NewPromptService(promptRepo *repo.PromptRepo, projectRepo *repo.ProjectRepo) *PromptService {
	return &PromptService{promptRepo: promptRepo, projectRepo: projectRepo}
}

// SeedDefaults 播种并同步内置提示词：缺则插入；出厂内容有代码新版本时覆盖正文；
// 作者改过的（customized_flag=true）保留不动；已下线的内置提示词直接删除。
func (s *PromptService) SeedDefaults(ctx context.Context) error {
	if err := s.dropRetiredBuiltins(ctx); err != nil {
		return err
	}
	for _, domainItem := range ai.BuiltinDomains() {
		if err := s.seedOne(ctx, &domainItem, domain.PromptKindSystem, domainItem.SystemPromptCode,
			domainItem.SystemPromptName, "other", domainItem.DomainName+"：身份与底线",
			domainItem.SystemPromptText, domainItem.SortOrder); err != nil {
			return err
		}
		for i := range domainItem.CommandList {
			command := &domainItem.CommandList[i]
			if err := s.seedOne(ctx, &domainItem, domain.PromptKindCommand, command.PromptCode,
				command.PromptName, command.CategoryCode, command.DescriptionText,
				command.PromptText, command.SortOrder); err != nil {
				return err
			}
		}
	}
	return nil
}

// seedOne 播种单条内置提示词：不存在则插入；指纹不一致且未自改则覆盖正文。
func (s *PromptService) seedOne(ctx context.Context, domainItem *ai.BuiltinDomain, kindCode, promptCode,
	promptName, categoryCode, descriptionText, promptText string, sortOrder int) error {
	assetHash := ai.PromptAssetHash(promptCode, promptName, kindCode, categoryCode, descriptionText, promptText, sortOrder)
	existing, err := s.promptRepo.GetByCode(ctx, domain.PromptScopeGlobal, "", promptCode)
	if err != nil {
		return err
	}
	if existing == nil {
		return s.promptRepo.Create(ctx, &domain.PromptDO{
			PromptULID:      pkg.NewULID(domain.IDPrefixPrompt),
			PromptCode:      promptCode,
			PromptName:      promptName,
			PromptKindCode:  kindCode,
			ScopeCode:       domain.PromptScopeGlobal,
			DomainCode:      domainItem.DomainCode,
			CategoryCode:    categoryCode,
			DescriptionText: descriptionText,
			PromptText:      promptText,
			EnabledFlag:     true,
			BuiltinFlag:     true,
			AssetHashText:   assetHash,
			SortOrder:       sortOrder,
		})
	}
	if existing.CustomizedFlag || existing.AssetHashText == assetHash {
		return nil // 作者改过或已是最新出厂内容：不动
	}
	return s.promptRepo.Update(ctx, existing.PromptULID, map[string]any{
		"prompt_name":      promptName,
		"prompt_kind_code": kindCode,
		"domain_code":      domainItem.DomainCode,
		"category_code":    categoryCode,
		"description_text": descriptionText,
		"prompt_text":      promptText,
		"sort_order":       sortOrder,
		"asset_hash_text":  assetHash,
	})
}

// dropRetiredBuiltins 删除不在当前内置清单中的旧内置提示词（用户自建提示词不受影响）。
func (s *PromptService) dropRetiredBuiltins(ctx context.Context) error {
	builtinList, err := s.promptRepo.ListBuiltin(ctx)
	if err != nil {
		return err
	}
	codeSet := ai.BuiltinCommandCodeSet()
	for code := range ai.BuiltinSystemCodeSet() {
		codeSet[code] = true
	}
	for i := range builtinList {
		if codeSet[builtinList[i].PromptCode] {
			continue
		}
		if err := s.promptRepo.Delete(ctx, builtinList[i].PromptULID); err != nil {
			return err
		}
	}
	return nil
}

// List 查询提示词列表：项目 ULID 非空时合并「项目提示词 + 全局提示词」并标记被覆盖项。
func (s *PromptService) List(ctx context.Context, req *domain.PromptListREQ) ([]domain.PromptVO, error) {
	if req.ProjectULID != "" {
		if err := s.mustProjectAccess(ctx, req.ProjectULID); err != nil {
			return nil, err
		}
	}
	dos, err := s.promptRepo.List(ctx, req)
	if err != nil {
		return nil, err
	}
	vos := make([]domain.PromptVO, 0, len(dos))
	for i := range dos {
		vo := toPromptVO(&dos[i])
		vo.OverriddenFlag = isOverriddenGlobalPrompt(&dos[i], dos)
		vos = append(vos, vo)
	}
	return vos, nil
}

// isOverriddenGlobalPrompt 判断全局提示词是否被同编码的启用项目提示词覆盖（纯函数）。
func isOverriddenGlobalPrompt(target *domain.PromptDO, all []domain.PromptDO) bool {
	if target.ScopeCode != domain.PromptScopeGlobal {
		return false
	}
	for i := range all {
		if all[i].ScopeCode == domain.PromptScopeProject && all[i].EnabledFlag && all[i].PromptCode == target.PromptCode {
			return true
		}
	}
	return false
}

// Detail 查询提示词详情。
func (s *PromptService) Detail(ctx context.Context, promptULID string) (domain.PromptVO, error) {
	do, err := s.promptRepo.GetByULID(ctx, promptULID)
	if err != nil {
		return domain.PromptVO{}, err
	}
	if do == nil {
		return domain.PromptVO{}, pkg.NewBizError(domain.RespCodeNotFound, "提示词不存在")
	}
	return toPromptVO(do), nil
}

// Save 创建或更新提示词（同作用域下编码唯一；项目提示词必须带项目且校验归属）。
func (s *PromptService) Save(ctx context.Context, req *domain.PromptREQ) (domain.PromptVO, error) {
	scopeCode := req.ScopeCode
	if scopeCode == "" {
		scopeCode = domain.PromptScopeGlobal
	}
	projectULID := ""
	if scopeCode == domain.PromptScopeProject {
		if req.ProjectULID == "" {
			return domain.PromptVO{}, pkg.NewBizError(domain.RespCodeInvalidParam, "项目提示词必须指定项目")
		}
		if err := s.mustProjectAccess(ctx, req.ProjectULID); err != nil {
			return domain.PromptVO{}, err
		}
		projectULID = req.ProjectULID
	}
	if err := validatePromptPlaceholders(req.PromptText); err != nil {
		return domain.PromptVO{}, err
	}
	kindCode := req.PromptKindCode
	if kindCode == "" {
		kindCode = domain.PromptKindCommand
	}
	// 指令提示词以 /编码 触发，与写作台的本地动作同占一个命名空间，冲突会让触发语义变得不可预期
	if kindCode == domain.PromptKindCommand && domain.IsReservedActionCode(req.PromptCode) {
		return domain.PromptVO{}, pkg.NewBizError(domain.RespCodeInvalidParam,
			"编码 /"+req.PromptCode+" 已被写作台本地动作占用，请换一个编码")
	}
	categoryCode := req.CategoryCode
	if categoryCode == "" {
		categoryCode = domain.PromptCategoryOther
	}
	existingByCode, err := s.promptRepo.GetByCode(ctx, scopeCode, projectULID, req.PromptCode)
	if err != nil {
		return domain.PromptVO{}, err
	}
	if existingByCode != nil && existingByCode.PromptULID != req.PromptULID {
		return domain.PromptVO{}, pkg.NewBizError(domain.RespCodeConflict, "同作用域下提示词编码已存在")
	}
	enabledFlag := true
	if req.EnabledFlag != nil {
		enabledFlag = *req.EnabledFlag
	}
	if req.PromptULID == "" {
		do := &domain.PromptDO{
			PromptULID:      pkg.NewULID(domain.IDPrefixPrompt),
			PromptCode:      req.PromptCode,
			PromptName:      req.PromptName,
			PromptKindCode:  kindCode,
			ScopeCode:       scopeCode,
			ProjectULID:     projectULID,
			CategoryCode:    categoryCode,
			DescriptionText: req.DescriptionText,
			PromptText:      req.PromptText,
			EnabledFlag:     enabledFlag,
			SortOrder:       req.SortOrder,
		}
		if err := s.promptRepo.Create(ctx, do); err != nil {
			return domain.PromptVO{}, err
		}
		return toPromptVO(do), nil
	}
	current, err := s.promptRepo.GetByULID(ctx, req.PromptULID)
	if err != nil {
		return domain.PromptVO{}, err
	}
	if current == nil {
		return domain.PromptVO{}, pkg.NewBizError(domain.RespCodeNotFound, "提示词不存在")
	}
	if current.BuiltinFlag && (current.PromptCode != req.PromptCode || current.ScopeCode != scopeCode ||
		current.PromptKindCode != kindCode) {
		return domain.PromptVO{}, pkg.NewBizError(domain.RespCodeBizFailed, "内置提示词不允许修改编码、作用域与种类")
	}
	fields := map[string]any{
		"prompt_code":      req.PromptCode,
		"prompt_name":      req.PromptName,
		"prompt_kind_code": kindCode,
		"scope_code":       scopeCode,
		"project_ulid":     projectULID,
		"category_code":    categoryCode,
		"description_text": req.DescriptionText,
		"prompt_text":      req.PromptText,
		"enabled_flag":     enabledFlag,
		"sort_order":       req.SortOrder,
	}
	if current.BuiltinFlag {
		// 内置资产：与出厂正文一致则仍算出厂内容，否则标记作者改动，后续代码升级不再覆盖。
		assetHash := ai.PromptAssetHash(req.PromptCode, req.PromptName, kindCode, categoryCode,
			req.DescriptionText, req.PromptText, req.SortOrder)
		customizedFlag := assetHash != ai.BuiltinCommandHashOf(req.PromptCode)
		if kindCode == domain.PromptKindSystem {
			customizedFlag = assetHash != ai.BuiltinSystemHashOf(req.PromptCode)
		}
		fields["customized_flag"] = customizedFlag
	}
	if err := s.promptRepo.Update(ctx, current.PromptULID, fields); err != nil {
		return domain.PromptVO{}, err
	}
	updated, err := s.promptRepo.GetByULID(ctx, current.PromptULID)
	if err != nil {
		return domain.PromptVO{}, err
	}
	return toPromptVO(updated), nil
}

// Delete 删除提示词（内置不可删，可停用）。
func (s *PromptService) Delete(ctx context.Context, req *domain.PromptDeleteREQ) error {
	current, err := s.promptRepo.GetByULID(ctx, req.PromptULID)
	if err != nil {
		return err
	}
	if current == nil {
		return pkg.NewBizError(domain.RespCodeNotFound, "提示词不存在")
	}
	if current.BuiltinFlag {
		return pkg.NewBizError(domain.RespCodeBizFailed, "内置提示词不允许删除，可停用")
	}
	return s.promptRepo.Delete(ctx, req.PromptULID)
}

// Resolve 解析用户输入为提示词：显式 prompt_code 优先，否则解析正文首 token 的 /code。
func (s *PromptService) Resolve(ctx context.Context, req *domain.PromptResolveREQ) (domain.PromptResolveDTO, error) {
	if err := s.mustProjectAccess(ctx, req.ProjectULID); err != nil {
		return domain.PromptResolveDTO{}, err
	}
	promptCode, remainderText := resolveSlashCode(req.PromptCode, req.ContentText)
	if promptCode == "" {
		return domain.PromptResolveDTO{RemainderText: remainderText}, nil
	}
	// 写作台本地动作由前端就地执行，后端不当提示词解析：
	// 直接调接口发「/help ...」时应按自由对话处理，而不是报「提示词不存在」。
	if domain.IsReservedActionCode(promptCode) {
		return domain.PromptResolveDTO{RemainderText: req.ContentText}, nil
	}
	prompt, err := s.resolveByCode(ctx, req.ProjectULID, promptCode)
	if err != nil {
		return domain.PromptResolveDTO{}, err
	}
	return domain.PromptResolveDTO{
		PromptULID:    prompt.PromptULID,
		PromptCode:    prompt.PromptCode,
		PromptName:    prompt.PromptName,
		PromptText:    prompt.PromptText,
		RemainderText: remainderText,
	}, nil
}

// SystemPromptOf 解析对话应使用的身份提示词：显式编码优先，其次启用的全局默认身份提示词，
// 全部缺失时回退到内置基础提示词（保证链路永远有身份约束）。
func (s *PromptService) SystemPromptOf(ctx context.Context, projectULID, promptCode string) string {
	if strings.TrimSpace(promptCode) != "" {
		if do, err := s.resolveByCode(ctx, projectULID, promptCode); err == nil {
			return do.PromptText
		}
	}
	globalPrompt, err := s.promptRepo.GetDefaultSystem(ctx)
	if err == nil && globalPrompt != nil {
		return globalPrompt.PromptText
	}
	return ai.SystemPrompt("")
}

// resolveByCode 按 (项目, 编码) 解析启用中的提示词：项目提示词优先于全局提示词。
func (s *PromptService) resolveByCode(ctx context.Context, projectULID, promptCode string) (*domain.PromptDO, error) {
	var projectPrompt *domain.PromptDO
	if projectULID != "" {
		found, err := s.promptRepo.GetByCode(ctx, domain.PromptScopeProject, projectULID, promptCode)
		if err != nil {
			return nil, err
		}
		projectPrompt = found
	}
	globalPrompt, err := s.promptRepo.GetByCode(ctx, domain.PromptScopeGlobal, "", promptCode)
	if err != nil {
		return nil, err
	}
	winner := pickPromptByScope(projectPrompt, globalPrompt)
	if winner == nil {
		return nil, pkg.NewBizError(domain.RespCodeBizFailed, "提示词不存在或已停用: "+promptCode)
	}
	return winner, nil
}

// pickPromptByScope 按作用域优先级挑选提示词（纯函数）：
// 启用中的项目提示词优先，其次启用中的全局提示词；停用与不存在一律不命中。
func pickPromptByScope(projectPrompt, globalPrompt *domain.PromptDO) *domain.PromptDO {
	if projectPrompt != nil && projectPrompt.EnabledFlag {
		return projectPrompt
	}
	if globalPrompt != nil && globalPrompt.EnabledFlag {
		return globalPrompt
	}
	return nil
}

// mustProjectAccess 校验项目存在（提示词作用域隔离用；不校验创建者，方便团队协作）。
func (s *PromptService) mustProjectAccess(ctx context.Context, projectULID string) error {
	project, err := s.projectRepo.GetByULID(ctx, projectULID)
	if err != nil {
		return err
	}
	if project == nil {
		return pkg.NewBizError(domain.RespCodeNotFound, "项目不存在")
	}
	return nil
}

// toPromptVO 组装提示词视图对象（内置提示词带上所属流程域，便于管理端按域归档）。
func toPromptVO(do *domain.PromptDO) domain.PromptVO {
	domainCode, domainName := do.DomainCode, ai.DomainNameOf(do.DomainCode)
	if do.BuiltinFlag {
		domainCode, domainName = ai.BuiltinDomainOf(do.PromptCode)
	}
	return domain.PromptVO{
		PromptULID:      do.PromptULID,
		PromptCode:      do.PromptCode,
		PromptName:      do.PromptName,
		PromptKindCode:  do.PromptKindCode,
		ScopeCode:       do.ScopeCode,
		ProjectULID:     do.ProjectULID,
		DomainCode:      domainCode,
		DomainName:      domainName,
		CategoryCode:    do.CategoryCode,
		DescriptionText: do.DescriptionText,
		PromptText:      do.PromptText,
		EnabledFlag:     do.EnabledFlag,
		BuiltinFlag:     do.BuiltinFlag,
		CustomizedFlag:  do.CustomizedFlag,
		SortOrder:       do.SortOrder,
		CreatedAt:       pkg.FormatTime(do.CreatedAt),
		UpdatedAt:       pkg.FormatTime(do.UpdatedAt),
	}
}
