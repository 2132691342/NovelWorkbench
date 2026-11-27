// 本文件实现模型供应商业务：启动播种、供应商 CRUD、密钥封存与连通性测试。
// 供应商只承载「网关 + 凭据」，具体模型与其角色授权见 model_catalog_service.go。

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

// ProviderSeed 启动期模型播种参数（来自配置文件，仅供应商表为空时生效）。
type ProviderSeed struct {
	ProviderName  string  // 供应商名称
	ProtocolCode  string  // 协议
	BaseURL       string  // 网关地址
	APIKey        string  // 明文密钥
	ChatModelName string  // 对话模型名
	Temperature   float64 // 采样温度
	MaxTokens     int     // 输出上限
	TimeoutSec    int     // 调用超时（秒）
}

// ModelProviderService 模型生态业务：供应商、模型目录、角色/用户授权与调用规格解析。
type ModelProviderService struct {
	providerRepo *repo.ModelProviderRepo // 模型生态仓储
	roleRepo     *repo.RoleRepo          // 角色仓储（授权与管理员判定）
	userRepo     *repo.UserRepo          // 用户仓储（用户直授的登录名展示）
	gateway      *ai.Client              // 模型网关（流式调用与用量上报）
	secretKey    string                  // 密钥封存材料（取 JWT 密钥派生）
	timeout      time.Duration           // 默认调用超时
}

// NewModelProviderService 构造模型生态服务。
func NewModelProviderService(providerRepo *repo.ModelProviderRepo, roleRepo *repo.RoleRepo, userRepo *repo.UserRepo,
	gateway *ai.Client, secretKey string, timeout time.Duration) *ModelProviderService {
	return &ModelProviderService{providerRepo: providerRepo, roleRepo: roleRepo, userRepo: userRepo,
		gateway: gateway, secretKey: secretKey, timeout: timeout}
}

// SeedDefault 按配置播种「一个供应商 + 一个默认模型」，并授权给内置角色（幂等）。
// 判重按 provider_name 与 base_url：同名或同网关已存在即跳过，避免手工建的供应商让播种永久失效。
func (s *ModelProviderService) SeedDefault(ctx context.Context, seed ProviderSeed) error {
	baseURL := strings.TrimRight(seed.BaseURL, "/")
	existing, err := s.providerRepo.FindByNameOrBaseURL(ctx, seed.ProviderName, baseURL)
	if err != nil {
		return pkg.WrapBizError(domain.RespCodeStorageError, "供应商播种前查询失败", err)
	}
	if existing != nil {
		return nil // 配置即出厂默认，已播种过（或同名/同网关供应商已存在）就不再重复插
	}
	cipherText, err := pkg.SealSecret(seed.APIKey, s.secretKey)
	if err != nil {
		return pkg.WrapBizError(domain.RespCodeInternalError, "供应商密钥封存失败", err)
	}
	providerDO := &domain.ModelProviderDO{
		ProviderULID:       pkg.NewULID(domain.IDPrefixModelProvider),
		ProviderName:       seed.ProviderName,
		ProtocolCode:       seed.ProtocolCode,
		BaseURL:            baseURL,
		APIKeyCipher:       cipherText,
		EnabledFlag:        true,
		LastTestStatusCode: domain.ProviderTestStatusUntested,
		RemarkText:         "配置文件引导创建，可在模型供应商页修改",
	}
	if err := s.providerRepo.Create(ctx, providerDO); err != nil {
		if repo.IsDuplicateKey(err) {
			return nil // 并发启动另一实例已播种，视为成功
		}
		return pkg.WrapBizError(domain.RespCodeStorageError, "供应商播种失败", err)
	}
	modelDO := &domain.ModelCatalogDO{
		ModelULID:          pkg.NewULID(domain.IDPrefixModelCatalog),
		ProviderULID:       providerDO.ProviderULID,
		ModelName:          seed.ChatModelName,
		ModelLabel:         seed.ChatModelName,
		Temperature:        seed.Temperature,
		MaxTokens:          seed.MaxTokens,
		EnabledFlag:        true,
		DefaultFlag:        true,
		LastTestStatusCode: domain.ProviderTestStatusUntested,
	}
	if err := s.providerRepo.CreateModel(ctx, modelDO); err != nil {
		if repo.IsDuplicateKey(err) {
			return nil // 同供应商下该模型已存在（并发播种），不重复插
		}
		// 回滚刚建的供应商：否则下次启动按名称判重会跳过它，出厂默认模型永远补不上。
		if deleteErr := s.providerRepo.Delete(ctx, providerDO.ProviderULID); deleteErr != nil {
			pkg.SLogger().Warnw("播种失败后回滚供应商失败", "provider_ulid", providerDO.ProviderULID, "error", deleteErr)
		}
		return pkg.WrapBizError(domain.RespCodeStorageError, "默认模型播种失败", err)
	}
	return s.grantToBuiltinRoles(ctx, modelDO.ModelULID)
}

// MigrateLegacyModels 把旧版「供应商自带一个模型」的数据迁移为模型目录行（幂等）。
// 逐供应商判断：只迁移「该供应商在模型目录里没有任何行」的 legacy 模型，
// 避免目录被清空后重复灌入，也避免管理员手工加过一个模型就让所有旧数据永远迁不过来。
func (s *ModelProviderService) MigrateLegacyModels(ctx context.Context) error {
	hasColumn, err := s.providerRepo.HasLegacyModelColumn(ctx)
	if err != nil {
		return pkg.WrapBizError(domain.RespCodeStorageError, "旧模型列检查失败", err)
	}
	if !hasColumn {
		return nil // 新库没有旧列，无需迁移
	}
	rows, err := s.providerRepo.ListLegacyProviderModels(ctx)
	if err != nil {
		return pkg.WrapBizError(domain.RespCodeStorageError, "旧模型数据读取失败", err)
	}
	if len(rows) == 0 {
		return nil
	}
	coveredULIDList, err := s.providerRepo.ListProviderULIDsWithModel(ctx)
	if err != nil {
		return pkg.WrapBizError(domain.RespCodeStorageError, "模型目录归属读取失败", err)
	}
	coveredSet := make(map[string]bool, len(coveredULIDList))
	for _, providerULID := range coveredULIDList {
		coveredSet[providerULID] = true
	}
	var defaultModelULID string
	migratedCount := 0
	for _, row := range rows {
		if coveredSet[row.ProviderULID] {
			// 该供应商已有模型目录行：旧列值是过期的，清掉以免目录被清空后又被灌回。
			if err := s.providerRepo.ClearLegacyModelName(ctx, row.ProviderULID); err != nil {
				return pkg.WrapBizError(domain.RespCodeStorageError, "旧模型列清理失败", err)
			}
			continue
		}
		do := &domain.ModelCatalogDO{
			ModelULID:          pkg.NewULID(domain.IDPrefixModelCatalog),
			ProviderULID:       row.ProviderULID,
			ModelName:          row.ChatModelName,
			ModelLabel:         row.ChatModelName,
			Temperature:        row.Temperature,
			MaxTokens:          row.MaxTokens,
			EnabledFlag:        true,
			LastTestStatusCode: row.LastTestStatusCode,
		}
		if do.LastTestStatusCode == "" {
			do.LastTestStatusCode = domain.ProviderTestStatusUntested
		}
		if err := s.providerRepo.CreateModel(ctx, do); err != nil {
			if repo.IsDuplicateKey(err) {
				coveredSet[row.ProviderULID] = true // 并发实例已迁移该供应商，跳过
				continue
			}
			return pkg.WrapBizError(domain.RespCodeStorageError, "旧模型迁移失败", err)
		}
		coveredSet[row.ProviderULID] = true
		migratedCount++
		if err := s.grantToBuiltinRoles(ctx, do.ModelULID); err != nil {
			return err
		}
		// 清掉旧列值：这是「已迁移」的持久标记，目录被清空后也不会再灌回旧数据。
		if err := s.providerRepo.ClearLegacyModelName(ctx, row.ProviderULID); err != nil {
			return pkg.WrapBizError(domain.RespCodeStorageError, "旧模型列清理失败", err)
		}
		if row.DefaultFlag && defaultModelULID == "" {
			defaultModelULID = do.ModelULID
		}
	}
	if migratedCount == 0 {
		return nil
	}
	if defaultModelULID != "" {
		if err := s.providerRepo.ClearDefaultModel(ctx); err != nil {
			return err
		}
		if err := s.providerRepo.UpdateModel(ctx, defaultModelULID, map[string]any{"default_flag": true}); err != nil {
			return pkg.WrapBizError(domain.RespCodeStorageError, "旧默认模型迁移失败", err)
		}
	}
	pkg.SLogger().Infow("旧版模型数据已迁移", "count", migratedCount, "default_model_ulid", defaultModelULID)
	return nil
}

// grantToBuiltinRoles 把模型授权给全部内置角色（管理员与普通用户开箱可用，可随时撤销）。
func (s *ModelProviderService) grantToBuiltinRoles(ctx context.Context, modelULID string) error {
	roleList, err := s.roleRepo.ListAll(ctx)
	if err != nil {
		return pkg.WrapBizError(domain.RespCodeStorageError, "内置角色读取失败", err)
	}
	roleULIDList := make([]string, 0, len(roleList))
	for i := range roleList {
		if roleList[i].BuiltinFlag {
			roleULIDList = append(roleULIDList, roleList[i].RoleULID)
		}
	}
	if err := s.providerRepo.ReplaceGrants(ctx, modelULID, roleULIDList); err != nil {
		return pkg.WrapBizError(domain.RespCodeStorageError, "默认模型授权失败", err)
	}
	return nil
}

// ListProviders 查询供应商列表（含模型数量统计，密钥仅回显掩码）。
func (s *ModelProviderService) ListProviders(ctx context.Context) ([]domain.ModelProviderVO, error) {
	dos, err := s.providerRepo.List(ctx)
	if err != nil {
		return nil, pkg.WrapBizError(domain.RespCodeStorageError, "供应商列表读取失败", err)
	}
	modelDOS, err := s.providerRepo.ListModels(ctx, "")
	if err != nil {
		return nil, pkg.WrapBizError(domain.RespCodeStorageError, "模型目录读取失败", err)
	}
	totalMap := make(map[string]int, len(dos))
	enabledMap := make(map[string]int, len(dos))
	for i := range modelDOS {
		totalMap[modelDOS[i].ProviderULID]++
		if modelDOS[i].EnabledFlag {
			enabledMap[modelDOS[i].ProviderULID]++
		}
	}
	vos := make([]domain.ModelProviderVO, 0, len(dos))
	for i := range dos {
		vo := s.toProviderVO(&dos[i])
		vo.ModelCount = totalMap[vo.ProviderULID]
		vo.EnabledModelCount = enabledMap[vo.ProviderULID]
		vos = append(vos, vo)
	}
	return vos, nil
}

// SaveProvider 创建/更新供应商（provider_ulid 为空创建）。
func (s *ModelProviderService) SaveProvider(ctx context.Context, req *domain.ModelProviderREQ) (domain.ModelProviderVO, error) {
	baseURL := strings.TrimRight(req.BaseURL, "/")
	if req.ProtocolCode == domain.ProtocolCodeAnthropic && strings.HasSuffix(baseURL, "/v1") {
		baseURL = strings.TrimSuffix(baseURL, "/v1") // Anthropic SDK 自行拼 /v1/messages
	}
	if req.ProviderULID == "" {
		cipherText, err := s.sealKey(req.APIKeyText)
		if err != nil {
			return domain.ModelProviderVO{}, err
		}
		if cipherText == "" {
			return domain.ModelProviderVO{}, pkg.NewBizError(domain.RespCodeInvalidParam, "首次创建必须填写 API Key")
		}
		do := &domain.ModelProviderDO{
			ProviderULID:       pkg.NewULID(domain.IDPrefixModelProvider),
			ProviderName:       req.ProviderName,
			ProtocolCode:       req.ProtocolCode,
			BaseURL:            baseURL,
			APIKeyCipher:       cipherText,
			EnabledFlag:        boolValueOf(req.EnabledFlag, true),
			SortOrder:          req.SortOrder,
			RemarkText:         req.RemarkText,
			LastTestStatusCode: domain.ProviderTestStatusUntested,
		}
		if err := s.providerRepo.Create(ctx, do); err != nil {
			if repo.IsDuplicateKey(err) {
				return domain.ModelProviderVO{}, pkg.NewBizError(domain.RespCodeConflict,
					"已存在同名供应商："+req.ProviderName)
			}
			return domain.ModelProviderVO{}, pkg.WrapBizError(domain.RespCodeStorageError, "供应商创建失败", err)
		}
		return s.toProviderVO(do), nil
	}
	current, err := s.providerRepo.GetByULID(ctx, req.ProviderULID)
	if err != nil {
		return domain.ModelProviderVO{}, err
	}
	if current == nil {
		return domain.ModelProviderVO{}, pkg.NewBizError(domain.RespCodeNotFound, "供应商不存在")
	}
	fields := map[string]any{
		"provider_name": req.ProviderName,
		"protocol_code": req.ProtocolCode,
		"base_url":      baseURL,
		"sort_order":    req.SortOrder,
		"remark_text":   req.RemarkText,
	}
	if req.APIKeyText != "" {
		cipherText, sealErr := s.sealKey(req.APIKeyText)
		if sealErr != nil {
			return domain.ModelProviderVO{}, sealErr
		}
		fields["api_key_cipher"] = cipherText
	}
	if req.EnabledFlag != nil {
		fields["enabled_flag"] = *req.EnabledFlag
	}
	if err := s.providerRepo.Update(ctx, req.ProviderULID, fields); err != nil {
		if repo.IsDuplicateKey(err) {
			return domain.ModelProviderVO{}, pkg.NewBizError(domain.RespCodeConflict,
				"已存在同名供应商："+req.ProviderName)
		}
		return domain.ModelProviderVO{}, pkg.WrapBizError(domain.RespCodeStorageError, "供应商更新失败", err)
	}
	updated, err := s.providerRepo.GetByULID(ctx, req.ProviderULID)
	if err != nil || updated == nil {
		return domain.ModelProviderVO{}, pkg.WrapBizError(domain.RespCodeStorageError, "供应商回读失败", err)
	}
	return s.toProviderVO(updated), nil
}

// DeleteProvider 删除供应商（其下仍有模型时拒绝，避免误删模型与授权）。
func (s *ModelProviderService) DeleteProvider(ctx context.Context, req *domain.ModelProviderDeleteREQ) error {
	current, err := s.providerRepo.GetByULID(ctx, req.ProviderULID)
	if err != nil {
		return err
	}
	if current == nil {
		return nil // 幂等：目标已不存在，视为删除成功
	}
	modelCount, err := s.providerRepo.CountModelsByProvider(ctx, req.ProviderULID)
	if err != nil {
		return err
	}
	if modelCount > 0 {
		return pkg.NewBizError(domain.RespCodeBizFailed, "该供应商下还有模型，请先删除或迁移模型")
	}
	return s.providerRepo.Delete(ctx, req.ProviderULID)
}

// TestProvider 供应商连通性测试：用其下首个启用模型发一次极小对话，结果回写供应商记录。
func (s *ModelProviderService) TestProvider(ctx context.Context, req *domain.ModelProviderTestREQ) (domain.ModelProviderTestDTO, error) {
	providerDO, err := s.providerRepo.GetByULID(ctx, req.ProviderULID)
	if err != nil {
		return domain.ModelProviderTestDTO{}, err
	}
	if providerDO == nil {
		return domain.ModelProviderTestDTO{}, pkg.NewBizError(domain.RespCodeNotFound, "供应商不存在")
	}
	modelDOS, err := s.providerRepo.ListModels(ctx, req.ProviderULID)
	if err != nil {
		return domain.ModelProviderTestDTO{}, err
	}
	var probeModel *domain.ModelCatalogDO
	for i := range modelDOS {
		if modelDOS[i].EnabledFlag {
			probeModel = &modelDOS[i]
			break
		}
	}
	if probeModel == nil {
		return domain.ModelProviderTestDTO{}, pkg.NewBizError(domain.RespCodeBizFailed, "该供应商下没有启用的模型，无法测试连通性")
	}
	dto := s.probe(ctx, providerDO, probeModel)
	fields := map[string]any{
		"last_test_status_code": dto.StatusCode,
		"last_test_message":     truncateRunes(dto.MessageText, 480),
		"last_test_at":          time.Now(),
	}
	if err := s.providerRepo.Update(ctx, providerDO.ProviderULID, fields); err != nil {
		pkg.SLogger().Warnw("供应商测试结果回写失败", "provider_ulid", providerDO.ProviderULID, "error", err)
	}
	return domain.ModelProviderTestDTO{
		StatusCode:   dto.StatusCode,
		MessageText:  dto.MessageText,
		LatencyMs:    dto.LatencyMs,
		ReplyText:    dto.ReplyText,
		ProbeModelID: probeModel.ModelULID,
	}, nil
}

// probe 执行一次极小对话验证网关与模型可用（供应商/模型共用的唯一探测实现）。
func (s *ModelProviderService) probe(ctx context.Context, providerDO *domain.ModelProviderDO,
	modelDO *domain.ModelCatalogDO) domain.ModelCatalogTestDTO {
	spec, err := s.specOf(providerDO, modelDO)
	if err != nil {
		return domain.ModelCatalogTestDTO{StatusCode: domain.ProviderTestStatusFailed, MessageText: err.Error()}
	}
	startedAt := time.Now()
	result, probeErr := s.gateway.Probe(ctx, spec)
	latencyMs := time.Since(startedAt).Milliseconds()
	if probeErr != nil {
		return domain.ModelCatalogTestDTO{
			StatusCode:  domain.ProviderTestStatusFailed,
			MessageText: "对话通道失败：" + probeErr.Error(),
			LatencyMs:   latencyMs,
		}
	}
	return domain.ModelCatalogTestDTO{
		StatusCode:  domain.ProviderTestStatusSuccess,
		MessageText: "对话通道连通",
		LatencyMs:   latencyMs,
		ReplyText:   result.ReplyText,
	}
}

// toProviderVO 组装供应商视图对象（密钥只给掩码）。
func (s *ModelProviderService) toProviderVO(do *domain.ModelProviderDO) domain.ModelProviderVO {
	masked := ""
	configured := do.APIKeyCipher != ""
	if configured {
		if plainText, err := pkg.OpenSecret(do.APIKeyCipher, s.secretKey); err == nil {
			masked = maskSecret(plainText)
		} else {
			masked = "****（无法解密，请重新填写）"
		}
	}
	return domain.ModelProviderVO{
		ProviderULID:       do.ProviderULID,
		ProviderName:       do.ProviderName,
		ProtocolCode:       do.ProtocolCode,
		BaseURL:            do.BaseURL,
		APIKeyMasked:       masked,
		APIKeyConfigured:   configured,
		EnabledFlag:        do.EnabledFlag,
		SortOrder:          do.SortOrder,
		RemarkText:         do.RemarkText,
		LastTestStatusCode: do.LastTestStatusCode,
		LastTestMessage:    do.LastTestMessage,
		LastTestAt:         pkg.FormatTime(do.LastTestAt),
		UpdatedAt:          pkg.FormatTime(do.UpdatedAt),
	}
}

// specOf 将「供应商 + 模型」转为调用规格（解封密钥）。
func (s *ModelProviderService) specOf(providerDO *domain.ModelProviderDO,
	modelDO *domain.ModelCatalogDO) (ai.ProviderSpec, error) {
	apiKey, err := pkg.OpenSecret(providerDO.APIKeyCipher, s.secretKey)
	if err != nil {
		return ai.ProviderSpec{}, pkg.NewBizError(domain.RespCodeBizFailed, "供应商密钥无法解密，请重新填写 API Key")
	}
	return ai.ProviderSpec{
		ProviderULID:  providerDO.ProviderULID,
		ProviderName:  providerDO.ProviderName,
		ProtocolCode:  providerDO.ProtocolCode,
		BaseURL:       providerDO.BaseURL,
		APIKey:        apiKey,
		ChatModelName: modelDO.ModelName,
		Temperature:   modelDO.Temperature,
		MaxTokens:     modelDO.MaxTokens,
		Timeout:       s.timeout,
	}, nil
}

// sealKey 封存明文密钥（空串返回空串，供"保持不变"语义使用）。
func (s *ModelProviderService) sealKey(plainText string) (string, error) {
	if plainText == "" {
		return "", nil
	}
	cipherText, err := pkg.SealSecret(plainText, s.secretKey)
	if err != nil {
		return "", pkg.WrapBizError(domain.RespCodeInternalError, "密钥封存失败", err)
	}
	return cipherText, nil
}

// maskSecret 生成密钥掩码（保留首 4 位与末 3 位）。
func maskSecret(secretText string) string {
	if len(secretText) <= 10 {
		return "****"
	}
	return secretText[:4] + "***" + secretText[len(secretText)-3:]
}

// boolValueOf 可选布尔取值（nil 用默认）。
func boolValueOf(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}
