// 本文件实现模型目录业务：模型 CRUD、默认模型切换、连通性测试与角色授权分配。

package service

import (
	"context"
	"strings"
	"time"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
)

// ListModels 查询模型目录（可按供应商过滤；带供应商快照、角色授权与角色名）。
func (s *ModelProviderService) ListModels(ctx context.Context, providerULID string) ([]domain.ModelCatalogVO, error) {
	modelDOS, err := s.providerRepo.ListModels(ctx, providerULID)
	if err != nil {
		return nil, pkg.WrapBizError(domain.RespCodeStorageError, "模型目录读取失败", err)
	}
	providerDOS, err := s.providerRepo.List(ctx)
	if err != nil {
		return nil, pkg.WrapBizError(domain.RespCodeStorageError, "供应商列表读取失败", err)
	}
	providerMap := make(map[string]*domain.ModelProviderDO, len(providerDOS))
	for i := range providerDOS {
		providerMap[providerDOS[i].ProviderULID] = &providerDOS[i]
	}
	modelULIDList := make([]string, 0, len(modelDOS))
	for i := range modelDOS {
		modelULIDList = append(modelULIDList, modelDOS[i].ModelULID)
	}
	grantMap, err := s.providerRepo.ListGrantsByModelULIDList(ctx, modelULIDList)
	if err != nil {
		return nil, pkg.WrapBizError(domain.RespCodeStorageError, "模型授权读取失败", err)
	}
	roleDOS, err := s.roleRepo.ListAll(ctx)
	if err != nil {
		return nil, pkg.WrapBizError(domain.RespCodeStorageError, "角色列表读取失败", err)
	}
	roleNameMap := make(map[string]string, len(roleDOS))
	for i := range roleDOS {
		roleNameMap[roleDOS[i].RoleULID] = roleDOS[i].RoleName
	}
	vos := make([]domain.ModelCatalogVO, 0, len(modelDOS))
	for i := range modelDOS {
		vos = append(vos, s.toModelVO(&modelDOS[i], providerMap[modelDOS[i].ProviderULID],
			grantMap[modelDOS[i].ModelULID], roleNameMap))
	}
	return vos, nil
}

// SaveModel 创建/更新模型（model_ulid 为空创建；default_flag 全局唯一；role_ulid_list 整体覆盖授权）。
func (s *ModelProviderService) SaveModel(ctx context.Context, req *domain.ModelCatalogREQ) (domain.ModelCatalogVO, error) {
	providerDO, err := s.providerRepo.GetByULID(ctx, req.ProviderULID)
	if err != nil {
		return domain.ModelCatalogVO{}, err
	}
	if providerDO == nil {
		return domain.ModelCatalogVO{}, pkg.NewBizError(domain.RespCodeNotFound, "所属供应商不存在")
	}
	if req.ModelULID == "" {
		do := &domain.ModelCatalogDO{
			ModelULID:          pkg.NewULID(domain.IDPrefixModelCatalog),
			ProviderULID:       req.ProviderULID,
			ModelName:          strings.TrimSpace(req.ModelName),
			ModelLabel:         strings.TrimSpace(req.ModelLabel),
			Temperature:        temperatureOf(req.Temperature),
			MaxTokens:          req.MaxTokens,
			ContextWindowCount: req.ContextWindowCount,
			EnabledFlag:        boolValueOf(req.EnabledFlag, true),
			DefaultFlag:        boolValueOf(req.DefaultFlag, false),
			SortOrder:          req.SortOrder,
			RemarkText:         req.RemarkText,
			LastTestStatusCode: domain.ProviderTestStatusUntested,
		}
		if do.ModelLabel == "" {
			do.ModelLabel = do.ModelName
		}
		if do.DefaultFlag {
			if err := s.providerRepo.ClearDefaultModel(ctx); err != nil {
				return domain.ModelCatalogVO{}, err
			}
		}
		if err := s.providerRepo.CreateModel(ctx, do); err != nil {
			return domain.ModelCatalogVO{}, pkg.WrapBizError(domain.RespCodeStorageError, "模型创建失败", err)
		}
		if err := s.providerRepo.ReplaceGrants(ctx, do.ModelULID, req.RoleULIDList); err != nil {
			return domain.ModelCatalogVO{}, pkg.WrapBizError(domain.RespCodeStorageError, "模型授权失败", err)
		}
		return s.modelVOOf(ctx, do.ModelULID)
	}
	current, err := s.providerRepo.GetModelByULID(ctx, req.ModelULID)
	if err != nil {
		return domain.ModelCatalogVO{}, err
	}
	if current == nil {
		return domain.ModelCatalogVO{}, pkg.NewBizError(domain.RespCodeNotFound, "模型不存在")
	}
	fields := map[string]any{
		"provider_ulid":        req.ProviderULID,
		"model_name":           strings.TrimSpace(req.ModelName),
		"model_label":          strings.TrimSpace(req.ModelLabel),
		"temperature":          temperatureOf(req.Temperature),
		"max_tokens":           req.MaxTokens,
		"context_window_count": req.ContextWindowCount,
		"sort_order":           req.SortOrder,
		"remark_text":          req.RemarkText,
	}
	if labelText, _ := fields["model_label"].(string); labelText == "" {
		fields["model_label"] = fields["model_name"]
	}
	if req.EnabledFlag != nil {
		fields["enabled_flag"] = *req.EnabledFlag
	}
	if req.DefaultFlag != nil {
		if *req.DefaultFlag {
			if err := s.providerRepo.ClearDefaultModel(ctx); err != nil {
				return domain.ModelCatalogVO{}, err
			}
		}
		fields["default_flag"] = *req.DefaultFlag
	}
	if err := s.providerRepo.UpdateModel(ctx, req.ModelULID, fields); err != nil {
		return domain.ModelCatalogVO{}, pkg.WrapBizError(domain.RespCodeStorageError, "模型更新失败", err)
	}
	if req.RoleULIDList != nil {
		if err := s.providerRepo.ReplaceGrants(ctx, req.ModelULID, req.RoleULIDList); err != nil {
			return domain.ModelCatalogVO{}, pkg.WrapBizError(domain.RespCodeStorageError, "模型授权失败", err)
		}
	}
	return s.modelVOOf(ctx, req.ModelULID)
}

// DeleteModel 删除模型（默认模型需先切换；授权行级联清理）。
func (s *ModelProviderService) DeleteModel(ctx context.Context, req *domain.ModelCatalogDeleteREQ) error {
	current, err := s.providerRepo.GetModelByULID(ctx, req.ModelULID)
	if err != nil {
		return err
	}
	if current == nil {
		return pkg.NewBizError(domain.RespCodeNotFound, "模型不存在")
	}
	if current.DefaultFlag {
		return pkg.NewBizError(domain.RespCodeBizFailed, "系统默认模型不可删除，请先把其他模型设为默认")
	}
	if err := s.providerRepo.DeleteGrantsByModel(ctx, req.ModelULID); err != nil {
		return pkg.WrapBizError(domain.RespCodeStorageError, "模型授权清理失败", err)
	}
	return s.providerRepo.DeleteModel(ctx, req.ModelULID)
}

// SetDefaultModel 切换系统默认模型（全局唯一）。
func (s *ModelProviderService) SetDefaultModel(ctx context.Context, req *domain.ModelCatalogTestREQ) error {
	current, err := s.providerRepo.GetModelByULID(ctx, req.ModelULID)
	if err != nil {
		return err
	}
	if current == nil {
		return pkg.NewBizError(domain.RespCodeNotFound, "模型不存在")
	}
	if !current.EnabledFlag {
		return pkg.NewBizError(domain.RespCodeBizFailed, "已停用的模型不能设为默认，请先启用")
	}
	if err := s.providerRepo.ClearDefaultModel(ctx); err != nil {
		return pkg.WrapBizError(domain.RespCodeStorageError, "默认模型切换失败", err)
	}
	if err := s.providerRepo.UpdateModel(ctx, req.ModelULID, map[string]any{"default_flag": true}); err != nil {
		return pkg.WrapBizError(domain.RespCodeStorageError, "默认模型切换失败", err)
	}
	return nil
}

// TestModel 模型连通性测试（结果回写模型记录）。
func (s *ModelProviderService) TestModel(ctx context.Context, req *domain.ModelCatalogTestREQ) (domain.ModelCatalogTestDTO, error) {
	modelDO, err := s.providerRepo.GetModelByULID(ctx, req.ModelULID)
	if err != nil {
		return domain.ModelCatalogTestDTO{}, err
	}
	if modelDO == nil {
		return domain.ModelCatalogTestDTO{}, pkg.NewBizError(domain.RespCodeNotFound, "模型不存在")
	}
	providerDO, err := s.providerRepo.GetByULID(ctx, modelDO.ProviderULID)
	if err != nil {
		return domain.ModelCatalogTestDTO{}, err
	}
	if providerDO == nil {
		return domain.ModelCatalogTestDTO{}, pkg.NewBizError(domain.RespCodeNotFound, "所属供应商不存在")
	}
	dto := s.probe(ctx, providerDO, modelDO)
	fields := map[string]any{
		"last_test_status_code": dto.StatusCode,
		"last_test_message":     truncateRunes(dto.MessageText, 480),
		"last_test_at_ms":       time.Now().UnixMilli(),
	}
	if err := s.providerRepo.UpdateModel(ctx, modelDO.ModelULID, fields); err != nil {
		pkg.SLogger().Warnw("模型测试结果回写失败", "model_ulid", modelDO.ModelULID, "error", err)
	}
	return dto, nil
}

// GrantModel 整体覆盖模型的角色授权（按角色分配模型使用权限）。
func (s *ModelProviderService) GrantModel(ctx context.Context, req *domain.ModelGrantREQ) error {
	modelDO, err := s.providerRepo.GetModelByULID(ctx, req.ModelULID)
	if err != nil {
		return err
	}
	if modelDO == nil {
		return pkg.NewBizError(domain.RespCodeNotFound, "模型不存在")
	}
	if err := s.providerRepo.ReplaceGrants(ctx, req.ModelULID, req.RoleULIDList); err != nil {
		return pkg.WrapBizError(domain.RespCodeStorageError, "模型授权失败", err)
	}
	return nil
}

// modelVOOf 按 ULID 读取并组装模型视图对象。
func (s *ModelProviderService) modelVOOf(ctx context.Context, modelULID string) (domain.ModelCatalogVO, error) {
	modelDO, err := s.providerRepo.GetModelByULID(ctx, modelULID)
	if err != nil {
		return domain.ModelCatalogVO{}, err
	}
	if modelDO == nil {
		return domain.ModelCatalogVO{}, pkg.NewBizError(domain.RespCodeNotFound, "模型不存在")
	}
	providerDO, err := s.providerRepo.GetByULID(ctx, modelDO.ProviderULID)
	if err != nil {
		return domain.ModelCatalogVO{}, err
	}
	grantMap, err := s.providerRepo.ListGrantsByModelULIDList(ctx, []string{modelULID})
	if err != nil {
		return domain.ModelCatalogVO{}, err
	}
	roleDOS, err := s.roleRepo.ListAll(ctx)
	if err != nil {
		return domain.ModelCatalogVO{}, err
	}
	roleNameMap := make(map[string]string, len(roleDOS))
	for i := range roleDOS {
		roleNameMap[roleDOS[i].RoleULID] = roleDOS[i].RoleName
	}
	return s.toModelVO(modelDO, providerDO, grantMap[modelULID], roleNameMap), nil
}

// toModelVO 组装模型视图对象（供应商快照 + 角色授权）。
func (s *ModelProviderService) toModelVO(do *domain.ModelCatalogDO, providerDO *domain.ModelProviderDO,
	roleULIDList []string, roleNameMap map[string]string) domain.ModelCatalogVO {
	vo := domain.ModelCatalogVO{
		ModelULID:          do.ModelULID,
		ProviderULID:       do.ProviderULID,
		ModelName:          do.ModelName,
		ModelLabel:         do.ModelLabel,
		Temperature:        do.Temperature,
		MaxTokens:          do.MaxTokens,
		ContextWindowCount: do.ContextWindowCount,
		EnabledFlag:        do.EnabledFlag,
		DefaultFlag:        do.DefaultFlag,
		SortOrder:          do.SortOrder,
		RemarkText:         do.RemarkText,
		RoleULIDList:       roleULIDList,
		RoleNameList:       make([]string, 0, len(roleULIDList)),
		LastTestStatusCode: do.LastTestStatusCode,
		LastTestMessage:    do.LastTestMessage,
		LastTestAtMs:       do.LastTestAtMs,
		UpdatedAtMs:        do.UpdatedAt.UnixMilli(),
	}
	if vo.RoleULIDList == nil {
		vo.RoleULIDList = []string{}
	}
	for _, roleULID := range roleULIDList {
		vo.RoleNameList = append(vo.RoleNameList, roleNameMap[roleULID])
	}
	if providerDO != nil {
		vo.ProviderName = providerDO.ProviderName
		vo.ProtocolCode = providerDO.ProtocolCode
		vo.BaseURL = providerDO.BaseURL
		vo.ProviderEnabled = providerDO.EnabledFlag
	}
	return vo
}

// temperatureOf 采样温度缺省值。
func temperatureOf(value *float64) float64 {
	if value == nil || *value <= 0 {
		return 0.7
	}
	return *value
}
