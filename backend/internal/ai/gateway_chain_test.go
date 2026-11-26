// 模型网关链路的回归：实例缓存键维度 → 流式客户端超时策略。
// 缓存键漏掉任一维度都会让换密钥后继续复用旧实例、请求带旧凭据；
// 流式客户端的整体超时会拦腰切断长流，写作台的流式输出会半途哑掉。

package ai

import (
	"net/http"
	"testing"
	"time"
)

// TestModelGatewayChain 覆盖模型网关的实例构造。
func TestModelGatewayChain(t *testing.T) {
	// 步骤1：缓存键。密钥、协议、地址、模型、温度、输出上限、超时任一变化都必须换键。
	base := ProviderSpec{
		ProviderULID: "PRV_1", ProtocolCode: ProtocolOpenAI, BaseURL: "https://gw.example.com",
		APIKey: "key-a", ChatModelName: "m1", Temperature: 0.7, MaxTokens: 2048, Timeout: 120 * time.Second,
	}
	mutations := []func(spec *ProviderSpec){
		func(spec *ProviderSpec) { spec.APIKey = "key-b" },
		func(spec *ProviderSpec) { spec.ProtocolCode = ProtocolAnthropic },
		func(spec *ProviderSpec) { spec.BaseURL = "https://gw2.example.com" },
		func(spec *ProviderSpec) { spec.ChatModelName = "m2" },
		func(spec *ProviderSpec) { spec.Temperature = 0.9 },
		func(spec *ProviderSpec) { spec.MaxTokens = 4096 },
		func(spec *ProviderSpec) { spec.Timeout = 60 * time.Second },
	}
	for _, mutate := range mutations {
		changed := base
		mutate(&changed)
		if base.cacheKey() == changed.cacheKey() {
			t.Fatalf("配置变更后缓存键未变化，实例会被错误复用：%+v", changed)
		}
	}

	// 步骤2：流式客户端。整体超时会切断长流，只能靠首包超时与调用方 ctx 控制。
	client := newStreamHTTPClient(45 * time.Second)
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("应使用 *http.Transport，实际 %T", client.Transport)
	}
	if client.Timeout != 0 || transport.ResponseHeaderTimeout != 45*time.Second {
		t.Fatalf("流式客户端超时策略错误：整体 %v，首包 %v", client.Timeout, transport.ResponseHeaderTimeout)
	}
}
