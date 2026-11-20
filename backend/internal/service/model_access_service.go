// 本文件实现模型的「按用户可用性」判定：角色授权并集、默认模型解析与对话规格下发。
// 管理员（含 * 通配权限）可见全部启用模型；其他用户只能使用被授权角色的模型。

package service

import (
	"context"

	"github.com/2132691342/NovelWorkbench/backend/internal/ai"
	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
)

// ChatSpecOf 返回指定模型的对话规格（modelULID 为空时取当前用户的默认模型）。
// 用户必须对该模型有使用权限，否则返回明确的业务错误。
func (s *ModelProviderService) ChatSpecOf(ctx context.Context, userULID string, modelULID string) (ai.ProviderSpec, error) {
	allowedList, err := s.usableModelList(ctx, userULID)
	if err != nil {
		return ai.ProviderSpec{}, err
	}
	if len(allowedList) == 0 {
		return ai.ProviderSpec{}, pkg.NewBizError(domain.RespCodeAIProviderError,
			"当前账号还没有可用模型，请联系管理员在「模型供应商」中分配")
	}
	target := pickModel(allowedList, modelULID)
	if target == nil {
		return ai.ProviderSpec{}, pkg.NewBizError(domain.RespCodeForbidden,
			"所选模型未授权给你的角色，请换一个模型或联系管理员")
	}
	providerDO, err := s.providerRepo.GetByULID(ctx, target.ProviderULID)
	if err != nil {
		return ai.ProviderSpec{}, pkg.WrapBizError(domain.RespCodeStorageError, "供应商读取失败", err)
	}
	if providerDO == nil || !providerDO.EnabledFlag {
		return ai.ProviderSpec{}, pkg.NewBizError(domain.RespCodeAIProviderError, "所选模型的供应商已停用")
	}
	return s.specOf(providerDO, target)
}

// DefaultChatSpec 返回系统默认模型（供检索重排等后台链路使用，不区分用户）。
func (s *ModelProviderService) DefaultChatSpec(ctx context.Context) (ai.ProviderSpec, error) {
	modelDO, err := s.providerRepo.FindDefaultModel(ctx)
	if err != nil {
		return ai.ProviderSpec{}, pkg.WrapBizError(domain.RespCodeStorageError, "默认模型读取失败", err)
	}
	if modelDO == nil {
		return ai.ProviderSpec{}, pkg.NewBizError(domain.RespCodeAIProviderError, "尚未设置系统默认模型")
	}
	providerDO, err := s.providerRepo.GetByULID(ctx, modelDO.ProviderULID)
	if err != nil {
		return ai.ProviderSpec{}, pkg.WrapBizError(domain.RespCodeStorageError, "供应商读取失败", err)
	}
	if providerDO == nil || !providerDO.EnabledFlag {
		return ai.ProviderSpec{}, pkg.NewBizError(domain.RespCodeAIProviderError, "默认模型的供应商已停用")
	}
	return s.specOf(providerDO, modelDO)
}

// Capability 返回当前用户的 AI 能力探测：可用模型列表、默认模型与提示文案。
func (s *ModelProviderService) Capability(ctx context.Context, userULID string) (domain.ModelCapabilityVO, error) {
	allowedList, err := s.usableModelList(ctx, userULID)
	if err != nil {
		return domain.ModelCapabilityVO{}, err
	}
	vo := domain.ModelCapabilityVO{OptionList: make([]domain.ModelOptionVO, 0, len(allowedList))}
	providerDOS, err := s.providerRepo.List(ctx)
	if err != nil {
		return domain.ModelCapabilityVO{}, pkg.WrapBizError(domain.RespCodeStorageError, "供应商列表读取失败", err)
	}
	providerNameMap := make(map[string]string, len(providerDOS))
	for i := range providerDOS {
		providerNameMap[providerDOS[i].ProviderULID] = providerDOS[i].ProviderName
	}
	for i := range allowedList {
		modelDO := &allowedList[i]
		vo.OptionList = append(vo.OptionList, domain.ModelOptionVO{
			ModelULID:      modelDO.ModelULID,
			ModelName:      modelDO.ModelName,
			ModelLabel:     modelDO.ModelLabel,
			ProviderName:   providerNameMap[modelDO.ProviderULID],
			DefaultFlag:    modelDO.DefaultFlag,
			MaxTokens:      modelDO.MaxTokens,
			ContextWindow:  modelDO.ContextWindowCount,
			LastTestStatus: modelDO.LastTestStatusCode,
		})
	}
	defaultModel := pickModel(allowedList, "")
	if defaultModel == nil {
		vo.HintText = "当前账号还没有可用模型，请联系管理员在「模型供应商」中分配"
		return vo, nil
	}
	vo.ChatReady = true
	vo.DefaultModelULID = defaultModel.ModelULID
	vo.DefaultModelLabel = defaultModel.ModelLabel
	vo.DefaultModelName = defaultModel.ModelName
	vo.DefaultProviderName = providerNameMap[defaultModel.ProviderULID]
	vo.HintText = "对话模型可用；知识库使用关键词检索，必要时由对话模型重排"
	return vo, nil
}

// usableModelList 计算用户可用模型：管理员看全部启用模型，其余按角色授权取并集。
func (s *ModelProviderService) usableModelList(ctx context.Context, userULID string) ([]domain.ModelCatalogDO, error) {
	modelDOS, err := s.providerRepo.ListModels(ctx, "")
	if err != nil {
		return nil, pkg.WrapBizError(domain.RespCodeStorageError, "模型目录读取失败", err)
	}
	providerDOS, err := s.providerRepo.List(ctx)
	if err != nil {
		return nil, pkg.WrapBizError(domain.RespCodeStorageError, "供应商列表读取失败", err)
	}
	providerEnabledMap := make(map[string]bool, len(providerDOS))
	for i := range providerDOS {
		providerEnabledMap[providerDOS[i].ProviderULID] = providerDOS[i].EnabledFlag
	}
	isAdmin, err := s.isAdminUser(ctx, userULID)
	if err != nil {
		return nil, err
	}
	grantedSet := make(map[string]bool)
	if !isAdmin {
		roleDOS, err := s.roleRepo.ListRolesByUserULID(ctx, userULID)
		if err != nil {
			return nil, pkg.WrapBizError(domain.RespCodeStorageError, "用户角色读取失败", err)
		}
		roleULIDList := make([]string, 0, len(roleDOS))
		for i := range roleDOS {
			roleULIDList = append(roleULIDList, roleDOS[i].RoleULID)
		}
		modelULIDList, err := s.providerRepo.ListGrantedModelULIDList(ctx, roleULIDList)
		if err != nil {
			return nil, pkg.WrapBizError(domain.RespCodeStorageError, "模型授权读取失败", err)
		}
		for _, modelULID := range modelULIDList {
			grantedSet[modelULID] = true
		}
	}
	allowed := make([]domain.ModelCatalogDO, 0, len(modelDOS))
	for i := range modelDOS {
		modelDO := modelDOS[i]
		if !modelDO.EnabledFlag || !providerEnabledMap[modelDO.ProviderULID] {
			continue
		}
		if !isAdmin && !grantedSet[modelDO.ModelULID] {
			continue
		}
		allowed = append(allowed, modelDO)
	}
	return allowed, nil
}

// isAdminUser 判断用户是否为管理员（拥有 * 通配权限）。
func (s *ModelProviderService) isAdminUser(ctx context.Context, userULID string) (bool, error) {
	roleDOS, err := s.roleRepo.ListRolesByUserULID(ctx, userULID)
	if err != nil {
		return false, pkg.WrapBizError(domain.RespCodeStorageError, "用户角色读取失败", err)
	}
	for i := range roleDOS {
		for _, permCode := range unmarshalPermCodes(roleDOS[i].PermCodesJSON) {
			if permCode == domain.PermAdminAll {
				return true, nil
			}
		}
	}
	return false, nil
}

// pickModel 在可用模型中挑选目标：指定 ULID 优先，否则取默认模型，再否则取第一个。
func pickModel(allowedList []domain.ModelCatalogDO, modelULID string) *domain.ModelCatalogDO {
	if modelULID != "" {
		for i := range allowedList {
			if allowedList[i].ModelULID == modelULID {
				return &allowedList[i]
			}
		}
		return nil
	}
	for i := range allowedList {
		if allowedList[i].DefaultFlag {
			return &allowedList[i]
		}
	}
	if len(allowedList) == 0 {
		return nil
	}
	return &allowedList[0]
}
