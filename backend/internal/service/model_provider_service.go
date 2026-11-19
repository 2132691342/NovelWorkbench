// 本文件实现模型供应商业务：启动播种、CRUD、默认切换、密钥解密与连通性测试。

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

// ProviderSeed 启动期供应商播种参数（来自配置文件，仅供应商表为空时生效）。
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

// ModelProviderService 模型供应商业务：CRUD、默认切换、连通性测试与调用规格解析。
type ModelProviderService struct {
	providerRepo *repo.ModelProviderRepo // 供应商仓储
	gateway      *ai.Client              // 模型网关（流式调用与用量上报）
	secretKey    string                  // 密钥封存材料（取 JWT 密钥派生）
	timeout      time.Duration           // 默认调用超时
}

// NewModelProviderService 构造模型供应商服务。
func NewModelProviderService(providerRepo *repo.ModelProviderRepo, gateway *ai.Client, secretKey string, timeout time.Duration) *ModelProviderService {
	return &ModelProviderService{providerRepo: providerRepo, gateway: gateway, secretKey: secretKey, timeout: timeout}
}

// SeedDefault 供应商表为空时按配置文件播种一条默认供应商（幂等）。
func (s *ModelProviderService) SeedDefault(ctx context.Context, seed ProviderSeed) error {
	count, err := s.providerRepo.Count(ctx)
	if err != nil || count > 0 {
		return err
	}
	cipherText, err := pkg.SealSecret(seed.APIKey, s.secretKey)
	if err != nil {
		return pkg.WrapBizError(domain.RespCodeInternalError, "供应商密钥封存失败", err)
	}
	do := &domain.ModelProviderDO{
		ProviderULID:       pkg.NewULID(domain.IDPrefixModelProvider),
		ProviderName:       seed.ProviderName,
		ProtocolCode:       seed.ProtocolCode,
		BaseURL:            strings.TrimRight(seed.BaseURL, "/"),
		APIKeyCipher:       cipherText,
		ChatModelName:      seed.ChatModelName,
		Temperature:        seed.Temperature,
		MaxTokens:          seed.MaxTokens,
		EnabledFlag:        true,
		DefaultFlag:        true,
		LastTestStatusCode: domain.ProviderTestStatusUntested,
		RemarkText:         "配置文件引导创建，可在模型供应商页修改",
	}
	if err := s.providerRepo.Create(ctx, do); err != nil {
		return pkg.WrapBizError(domain.RespCodeStorageError, "供应商播种失败", err)
	}
	return nil
}

// List 查询供应商列表（密钥仅回显掩码）。
func (s *ModelProviderService) List(ctx context.Context) ([]domain.ModelProviderVO, error) {
	dos, err := s.providerRepo.List(ctx)
	if err != nil {
		return nil, pkg.WrapBizError(domain.RespCodeStorageError, "供应商列表读取失败", err)
	}
	vos := make([]domain.ModelProviderVO, 0, len(dos))
	for i := range dos {
		vos = append(vos, s.toVO(&dos[i]))
	}
	return vos, nil
}

// Save 创建/更新供应商（provider_ulid 为空创建；默认标记全局唯一）。
func (s *ModelProviderService) Save(ctx context.Context, req *domain.ModelProviderREQ) (domain.ModelProviderVO, error) {
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
			ChatModelName:      req.ChatModelName,
			Temperature:        temperatureOf(req.Temperature),
			MaxTokens:          req.MaxTokens,
			EnabledFlag:        boolValueOf(req.EnabledFlag, true),
			DefaultFlag:        boolValueOf(req.DefaultFlag, false),
			SortOrder:          req.SortOrder,
			RemarkText:         req.RemarkText,
			LastTestStatusCode: domain.ProviderTestStatusUntested,
		}
		if do.DefaultFlag {
			if err := s.providerRepo.ClearDefault(ctx); err != nil {
				return domain.ModelProviderVO{}, err
			}
		}
		if err := s.providerRepo.Create(ctx, do); err != nil {
			return domain.ModelProviderVO{}, pkg.WrapBizError(domain.RespCodeStorageError, "供应商创建失败", err)
		}
		return s.voOf(ctx, do.ProviderULID)
	}
	current, err := s.providerRepo.GetByULID(ctx, req.ProviderULID)
	if err != nil {
		return domain.ModelProviderVO{}, err
	}
	if current == nil {
		return domain.ModelProviderVO{}, pkg.NewBizError(domain.RespCodeNotFound, "供应商不存在")
	}
	fields := map[string]any{
		"provider_name":   req.ProviderName,
		"protocol_code":   req.ProtocolCode,
		"base_url":        baseURL,
		"chat_model_name": req.ChatModelName,
		"temperature":     temperatureOf(req.Temperature),
		"max_tokens":      req.MaxTokens,
		"sort_order":      req.SortOrder,
		"remark_text":     req.RemarkText,
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
	if req.DefaultFlag != nil {
		if *req.DefaultFlag {
			if err := s.providerRepo.ClearDefault(ctx); err != nil {
				return domain.ModelProviderVO{}, err
			}
		}
		fields["default_flag"] = *req.DefaultFlag
	}
	if err := s.providerRepo.Update(ctx, req.ProviderULID, fields); err != nil {
		return domain.ModelProviderVO{}, pkg.WrapBizError(domain.RespCodeStorageError, "供应商更新失败", err)
	}
	return s.voOf(ctx, req.ProviderULID)
}

// Delete 删除供应商（默认供应商需先切换）。
func (s *ModelProviderService) Delete(ctx context.Context, req *domain.ModelProviderDeleteREQ) error {
	current, err := s.providerRepo.GetByULID(ctx, req.ProviderULID)
	if err != nil {
		return err
	}
	if current == nil {
		return pkg.NewBizError(domain.RespCodeNotFound, "供应商不存在")
	}
	if current.DefaultFlag {
		return pkg.NewBizError(domain.RespCodeBizFailed, "默认供应商不可删除，请先切换默认")
	}
	return s.providerRepo.Delete(ctx, req.ProviderULID)
}

// Test 连通性测试（只验证对话通道，结果回写供应商记录）。
func (s *ModelProviderService) Test(ctx context.Context, req *domain.ModelProviderTestREQ) (domain.ModelProviderTestDTO, error) {
	current, err := s.providerRepo.GetByULID(ctx, req.ProviderULID)
	if err != nil {
		return domain.ModelProviderTestDTO{}, err
	}
	if current == nil {
		return domain.ModelProviderTestDTO{}, pkg.NewBizError(domain.RespCodeNotFound, "供应商不存在")
	}
	spec, err := s.specOf(current)
	if err != nil {
		return domain.ModelProviderTestDTO{}, err
	}
	startedAt := time.Now()
	probe, probeErr := s.gateway.Probe(ctx, spec)
	latencyMs := time.Since(startedAt).Milliseconds()
	dto := domain.ModelProviderTestDTO{
		StatusCode:  domain.ProviderTestStatusSuccess,
		LatencyMs:   latencyMs,
		ReplyText:   probe.ReplyText,
		MessageText: "对话通道连通",
	}
	if probeErr != nil {
		dto.StatusCode = domain.ProviderTestStatusFailed
		dto.MessageText = "对话通道失败：" + probeErr.Error()
	}
	fields := map[string]any{
		"last_test_status_code": dto.StatusCode,
		"last_test_message":     truncateRunes(dto.MessageText, 480),
		"last_test_at_ms":       time.Now().UnixMilli(),
	}
	if err := s.providerRepo.Update(ctx, current.ProviderULID, fields); err != nil {
		pkg.SLogger().Warnw("供应商测试结果回写失败", "provider_ulid", current.ProviderULID, "error", err)
	}
	return dto, nil
}

// ChatSpec 返回默认供应商的对话规格（无可用供应商时给出明确业务错误）。
func (s *ModelProviderService) ChatSpec(ctx context.Context) (ai.ProviderSpec, error) {
	current, err := s.providerRepo.FindDefault(ctx)
	if err != nil {
		return ai.ProviderSpec{}, pkg.WrapBizError(domain.RespCodeStorageError, "默认供应商读取失败", err)
	}
	if current == nil {
		return ai.ProviderSpec{}, pkg.NewBizError(domain.RespCodeAIProviderError, "尚未配置可用的模型供应商，请到「模型供应商」页添加并设为默认")
	}
	return s.specOf(current)
}

// ChatSpecOf 返回指定供应商的对话规格（providerULID 为空时取默认）。
func (s *ModelProviderService) ChatSpecOf(ctx context.Context, providerULID string) (ai.ProviderSpec, error) {
	if providerULID == "" {
		return s.ChatSpec(ctx)
	}
	current, err := s.providerRepo.GetByULID(ctx, providerULID)
	if err != nil {
		return ai.ProviderSpec{}, err
	}
	if current == nil || !current.EnabledFlag {
		return ai.ProviderSpec{}, pkg.NewBizError(domain.RespCodeInvalidParam, "所选供应商不存在或已停用")
	}
	return s.specOf(current)
}

// Capability 返回当前 AI 能力探测结果（仅对话能力）。
func (s *ModelProviderService) Capability(ctx context.Context) (domain.ModelCapabilityVO, error) {
	vo := domain.ModelCapabilityVO{}
	defaultProvider, err := s.providerRepo.FindDefault(ctx)
	if err != nil {
		return vo, pkg.WrapBizError(domain.RespCodeStorageError, "供应商读取失败", err)
	}
	if defaultProvider != nil {
		vo.ChatReady = true
		vo.DefaultProviderULID = defaultProvider.ProviderULID
		vo.DefaultProviderName = defaultProvider.ProviderName
		vo.DefaultChatModelName = defaultProvider.ChatModelName
	}
	if !vo.ChatReady {
		vo.HintText = "尚未配置模型供应商，AI 生成与检索重排不可用；知识库仍可进行关键词检索"
	} else {
		vo.HintText = "对话模型可用；知识库使用关键词检索，必要时由对话模型重排"
	}
	return vo, nil
}

// specOf 将供应商记录转为调用规格（解封密钥）。
func (s *ModelProviderService) specOf(do *domain.ModelProviderDO) (ai.ProviderSpec, error) {
	apiKey, err := pkg.OpenSecret(do.APIKeyCipher, s.secretKey)
	if err != nil {
		return ai.ProviderSpec{}, pkg.NewBizError(domain.RespCodeBizFailed, "供应商密钥无法解密，请重新填写 API Key")
	}
	return ai.ProviderSpec{
		ProviderULID:  do.ProviderULID,
		ProviderName:  do.ProviderName,
		ProtocolCode:  do.ProtocolCode,
		BaseURL:       do.BaseURL,
		APIKey:        apiKey,
		ChatModelName: do.ChatModelName,
		Temperature:   do.Temperature,
		MaxTokens:     do.MaxTokens,
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

// voOf 按 ULID 读取并组装视图对象。
func (s *ModelProviderService) voOf(ctx context.Context, providerULID string) (domain.ModelProviderVO, error) {
	do, err := s.providerRepo.GetByULID(ctx, providerULID)
	if err != nil {
		return domain.ModelProviderVO{}, err
	}
	if do == nil {
		return domain.ModelProviderVO{}, pkg.NewBizError(domain.RespCodeNotFound, "供应商不存在")
	}
	return s.toVO(do), nil
}

// toVO 组装供应商视图对象（密钥只给掩码）。
func (s *ModelProviderService) toVO(do *domain.ModelProviderDO) domain.ModelProviderVO {
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
		ChatModelName:      do.ChatModelName,
		Temperature:        do.Temperature,
		MaxTokens:          do.MaxTokens,
		EnabledFlag:        do.EnabledFlag,
		DefaultFlag:        do.DefaultFlag,
		SortOrder:          do.SortOrder,
		RemarkText:         do.RemarkText,
		LastTestStatusCode: do.LastTestStatusCode,
		LastTestMessage:    do.LastTestMessage,
		LastTestAtMs:       do.LastTestAtMs,
		UpdatedAtMs:        do.UpdatedAt.UnixMilli(),
	}
}

// maskSecret 生成密钥掩码（保留首 4 位与末 3 位）。
func maskSecret(secretText string) string {
	if len(secretText) <= 10 {
		return "****"
	}
	return secretText[:4] + "***" + secretText[len(secretText)-3:]
}

// temperatureOf 采样温度缺省值。
func temperatureOf(value *float64) float64 {
	if value == nil || *value <= 0 {
		return 0.7
	}
	return *value
}

// boolValueOf 可选布尔取值（nil 用默认）。
func boolValueOf(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}
