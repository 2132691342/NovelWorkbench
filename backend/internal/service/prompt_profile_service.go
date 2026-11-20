// 本文件实现系统提示词档案业务：内置播种、列表、详情、保存、删除与对话期解析。

package service

import (
	"context"
	"strings"

	"github.com/2132691342/NovelWorkbench/backend/internal/ai"
	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
	"github.com/2132691342/NovelWorkbench/backend/internal/repo"
)

// PromptProfileService 系统提示词档案业务。
type PromptProfileService struct {
	profileRepo *repo.PromptProfileRepo // 档案仓储
	projectRepo *repo.ProjectRepo       // 项目仓储（归属校验）
}

// NewPromptProfileService 构造档案服务。
func NewPromptProfileService(profileRepo *repo.PromptProfileRepo, projectRepo *repo.ProjectRepo) *PromptProfileService {
	return &PromptProfileService{profileRepo: profileRepo, projectRepo: projectRepo}
}

// SeedDefaults 播种内置全局档案（幂等：按编码查缺补插，已存在则保留作者改动）。
func (s *PromptProfileService) SeedDefaults(ctx context.Context) error {
	for _, builtin := range ai.BuiltinPromptProfiles() {
		existing, err := s.profileRepo.GetByCode(ctx, domain.PromptProfileScopeGlobal, "", builtin.ProfileCode)
		if err != nil {
			return err
		}
		if existing != nil {
			continue
		}
		do := &domain.PromptProfileDO{
			PromptProfileULID: pkg.NewULID(domain.IDPrefixPromptProfile),
			PromptProfileCode: builtin.ProfileCode,
			PromptProfileName: builtin.ProfileName,
			ScopeCode:         domain.PromptProfileScopeGlobal,
			DescriptionText:   builtin.DescriptionText,
			SystemPromptText:  builtin.SystemPrompt,
			BuiltinFlag:       true,
			EnabledFlag:       true,
			SortOrder:         builtin.SortOrder,
		}
		if err := s.profileRepo.Create(ctx, do); err != nil {
			return err
		}
	}
	return nil
}

// List 查询档案列表：项目 ULID 非空时合并「项目档案 + 全局档案」。
func (s *PromptProfileService) List(ctx context.Context, userULID string, req *domain.PromptProfileListREQ) ([]domain.PromptProfileVO, error) {
	if req.ProjectULID != "" {
		if _, err := mustOwnProject(ctx, s.projectRepo, userULID, req.ProjectULID); err != nil {
			return nil, err
		}
	}
	dos, err := s.profileRepo.List(ctx, req)
	if err != nil {
		return nil, err
	}
	vos := make([]domain.PromptProfileVO, 0, len(dos))
	for i := range dos {
		vos = append(vos, toPromptProfileVO(&dos[i]))
	}
	return vos, nil
}

// Detail 查询单个档案。
func (s *PromptProfileService) Detail(ctx context.Context, userULID, promptProfileULID string) (domain.PromptProfileVO, error) {
	do, err := s.mustReadableProfile(ctx, userULID, promptProfileULID)
	if err != nil {
		return domain.PromptProfileVO{}, err
	}
	return toPromptProfileVO(do), nil
}

// Save 新建/更新档案（编码在同作用域内唯一；内置档案允许改正文但不允许改编码）。
func (s *PromptProfileService) Save(ctx context.Context, userULID string, req *domain.PromptProfileREQ) (domain.PromptProfileVO, error) {
	scopeCode := req.ScopeCode
	if scopeCode == "" {
		scopeCode = domain.PromptProfileScopeGlobal
	}
	if scopeCode == domain.PromptProfileScopeProject {
		if req.ProjectULID == "" {
			return domain.PromptProfileVO{}, pkg.NewBizError(domain.RespCodeInvalidParam, "项目档案必须指定项目")
		}
		if _, err := mustOwnProject(ctx, s.projectRepo, userULID, req.ProjectULID); err != nil {
			return domain.PromptProfileVO{}, err
		}
	}
	projectULID := ""
	if scopeCode == domain.PromptProfileScopeProject {
		projectULID = req.ProjectULID
	}
	if req.PromptProfileULID == "" {
		occupied, err := s.profileRepo.GetByCode(ctx, scopeCode, projectULID, req.PromptProfileCode)
		if err != nil {
			return domain.PromptProfileVO{}, err
		}
		if occupied != nil {
			return domain.PromptProfileVO{}, pkg.NewBizError(domain.RespCodeConflict, "同作用域下档案编码已存在: "+req.PromptProfileCode)
		}
		do := &domain.PromptProfileDO{
			PromptProfileULID: pkg.NewULID(domain.IDPrefixPromptProfile),
			PromptProfileCode: req.PromptProfileCode,
			PromptProfileName: req.PromptProfileName,
			ScopeCode:         scopeCode,
			ProjectULID:       projectULID,
			DescriptionText:   req.DescriptionText,
			SystemPromptText:  req.SystemPromptText,
			EnabledFlag:       true,
			SortOrder:         req.SortOrder,
		}
		if req.EnabledFlag != nil {
			do.EnabledFlag = *req.EnabledFlag
		}
		if err := s.profileRepo.Create(ctx, do); err != nil {
			return domain.PromptProfileVO{}, err
		}
		return toPromptProfileVO(do), nil
	}
	current, err := s.mustReadableProfile(ctx, userULID, req.PromptProfileULID)
	if err != nil {
		return domain.PromptProfileVO{}, err
	}
	if current.ScopeCode != scopeCode || current.ProjectULID != projectULID {
		return domain.PromptProfileVO{}, pkg.NewBizError(domain.RespCodeInvalidParam, "档案作用域不可变更")
	}
	if current.BuiltinFlag && current.PromptProfileCode != req.PromptProfileCode {
		return domain.PromptProfileVO{}, pkg.NewBizError(domain.RespCodeBizFailed, "内置档案编码不可修改")
	}
	occupiedCount, err := s.profileRepo.CountByCode(ctx, scopeCode, projectULID, req.PromptProfileCode, current.PromptProfileULID)
	if err != nil {
		return domain.PromptProfileVO{}, err
	}
	if occupiedCount > 0 {
		return domain.PromptProfileVO{}, pkg.NewBizError(domain.RespCodeConflict, "同作用域下档案编码已存在: "+req.PromptProfileCode)
	}
	fields := map[string]any{
		"prompt_profile_code": req.PromptProfileCode,
		"prompt_profile_name": req.PromptProfileName,
		"description_text":    req.DescriptionText,
		"system_prompt_text":  req.SystemPromptText,
		"sort_order":          req.SortOrder,
	}
	if req.EnabledFlag != nil {
		fields["enabled_flag"] = *req.EnabledFlag
	}
	if err := s.profileRepo.Update(ctx, current.PromptProfileULID, fields); err != nil {
		return domain.PromptProfileVO{}, err
	}
	updated, err := s.profileRepo.GetByULID(ctx, current.PromptProfileULID)
	if err != nil {
		return domain.PromptProfileVO{}, err
	}
	return toPromptProfileVO(updated), nil
}

// Delete 删除档案（内置档案不可删，可停用）。
func (s *PromptProfileService) Delete(ctx context.Context, userULID string, req *domain.PromptProfileDeleteREQ) error {
	current, err := s.mustReadableProfile(ctx, userULID, req.PromptProfileULID)
	if err != nil {
		return err
	}
	if current.BuiltinFlag {
		return pkg.NewBizError(domain.RespCodeBizFailed, "内置档案不允许删除，可停用")
	}
	return s.profileRepo.Delete(ctx, current.PromptProfileULID)
}

// SystemPromptOf 解析对话应使用的系统提示词：显式档案优先，其次项目默认、全局默认；
// 全部缺失时回退到 ai 内置基础提示词（保证链路永远有身份约束）。
func (s *PromptProfileService) SystemPromptOf(ctx context.Context, projectULID, promptProfileULID string) string {
	if strings.TrimSpace(promptProfileULID) != "" {
		if do, err := s.profileRepo.GetByULID(ctx, promptProfileULID); err == nil && do != nil &&
			do.EnabledFlag && (do.ScopeCode == domain.PromptProfileScopeGlobal || do.ProjectULID == projectULID) {
			return do.SystemPromptText
		}
	}
	globalProfile, err := s.profileRepo.GetDefaultGlobal(ctx)
	if err == nil && globalProfile != nil {
		return globalProfile.SystemPromptText
	}
	return ai.SystemPrompt("")
}

// mustReadableProfile 读取档案并校验可见性（全局档案任何人可读，项目档案需归属项目）。
func (s *PromptProfileService) mustReadableProfile(ctx context.Context, userULID, promptProfileULID string) (*domain.PromptProfileDO, error) {
	do, err := s.profileRepo.GetByULID(ctx, promptProfileULID)
	if err != nil {
		return nil, err
	}
	if do == nil {
		return nil, pkg.NewBizError(domain.RespCodeNotFound, "提示词档案不存在")
	}
	if do.ScopeCode == domain.PromptProfileScopeProject {
		if _, err := mustOwnProject(ctx, s.projectRepo, userULID, do.ProjectULID); err != nil {
			return nil, err
		}
	}
	return do, nil
}

// toPromptProfileVO 组装档案视图对象。
func toPromptProfileVO(do *domain.PromptProfileDO) domain.PromptProfileVO {
	return domain.PromptProfileVO{
		PromptProfileULID: do.PromptProfileULID,
		PromptProfileCode: do.PromptProfileCode,
		PromptProfileName: do.PromptProfileName,
		ScopeCode:         do.ScopeCode,
		ProjectULID:       do.ProjectULID,
		DescriptionText:   do.DescriptionText,
		SystemPromptText:  do.SystemPromptText,
		BuiltinFlag:       do.BuiltinFlag,
		EnabledFlag:       do.EnabledFlag,
		SortOrder:         do.SortOrder,
		UpdatedAtMs:       do.UpdatedAt.UnixMilli(),
	}
}
