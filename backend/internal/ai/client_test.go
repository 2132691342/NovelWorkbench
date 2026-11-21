// 模型网关的构造策略校验：实例缓存键必须随密钥与参数变化，长流客户端不得设整体超时。
// 两者都是「配错就会在生产里才暴露」的配置项，因此合并为一条用例守住。

package ai

import (
	"net/http"
	"testing"
	"time"
)

// TestProviderInstancePolicy 覆盖供应商实例的构造策略：缓存键区分密钥与参数 → 流式客户端无整体超时。
func TestProviderInstancePolicy(t *testing.T) {
	// 环节1：缓存键。密钥、协议、地址、模型、温度、输出上限任一变化都必须换键，
	// 否则换密钥后会继续用旧实例调用已失效的凭据。
	base := ProviderSpec{
		ProviderULID: "PRV_1", ProtocolCode: ProtocolOpenAI, BaseURL: "https://gw.example.com",
		APIKey: "key-a", ChatModelName: "m1", Temperature: 0.7, MaxTokens: 2048, Timeout: 120 * time.Second,
	}
	variants := []struct {
		caseName string
		mutate   func(spec *ProviderSpec)
	}{
		{"密钥变更", func(spec *ProviderSpec) { spec.APIKey = "key-b" }},
		{"协议变更", func(spec *ProviderSpec) { spec.ProtocolCode = ProtocolAnthropic }},
		{"地址变更", func(spec *ProviderSpec) { spec.BaseURL = "https://gw2.example.com" }},
		{"模型变更", func(spec *ProviderSpec) { spec.ChatModelName = "m2" }},
		{"温度变更", func(spec *ProviderSpec) { spec.Temperature = 0.9 }},
		{"输出上限变更", func(spec *ProviderSpec) { spec.MaxTokens = 4096 }},
		{"超时变更", func(spec *ProviderSpec) { spec.Timeout = 60 * time.Second }},
	}
	for _, variant := range variants {
		t.Run(variant.caseName, func(t *testing.T) {
			changed := base
			variant.mutate(&changed)
			if base.cacheKey() == changed.cacheKey() {
				t.Fatalf("%s 后缓存键未变化，实例会被错误复用", variant.caseName)
			}
		})
	}
	if base.cacheKey() != base.cacheKey() {
		t.Fatal("同一份规格必须产生稳定的缓存键")
	}

	// 环节2：流式客户端。整体超时会拦腰切断长流，只能靠连接/首包超时与调用方 ctx 控制。
	client := newStreamHTTPClient(45 * time.Second)
	if client.Timeout != 0 {
		t.Fatalf("流式客户端不得设置整体超时（会切断长流），实际 %v", client.Timeout)
	}
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("应使用 *http.Transport，实际 %T", client.Transport)
	}
	if transport.ResponseHeaderTimeout != 45*time.Second {
		t.Fatalf("首包超时应取供应商配置，实际 %v", transport.ResponseHeaderTimeout)
	}
	if transport.MaxIdleConnsPerHost <= 0 {
		t.Fatalf("连接池必须按并发上限收敛，实际 %d", transport.MaxIdleConnsPerHost)
	}
}
