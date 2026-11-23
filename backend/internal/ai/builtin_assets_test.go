// 内置资产与模型网关的场景回归。
// 资产侧：出厂流程域 → 编码唯一 → 模板可渲染 → 指纹稳定，任何一步被破坏都会让播种出的技能无法在对话中触发。
// 网关侧：实例缓存键 → 流式客户端超时策略，缓存键漏维度会让换密钥后继续用旧凭据，整体超时会拦腰切断长流。

package ai

import (
	"net/http"
	"testing"
	"time"
)

// TestBuiltinAssetChain 覆盖出厂资产从定义到可播种的完整链路。
func TestBuiltinAssetChain(t *testing.T) {
	domainList := BuiltinDomains()

	// 步骤1：域形状。每个流程域必须有名称、一份档案与两个技能。
	if len(domainList) < 8 {
		t.Fatalf("内置流程域不足 8 个：%d", len(domainList))
	}
	for _, item := range domainList {
		if item.DomainCode == "" || item.DomainName == "" || item.Profile.ProfileCode == "" ||
			item.Profile.SystemPrompt == "" || len(item.SkillList) != 2 {
			t.Fatalf("流程域形状不合法：%+v", item)
		}
	}

	// 步骤2：编码唯一。技能编码与档案编码全局唯一，否则播种时互相覆盖。
	skillCodeSet := map[string]bool{}
	profileCodeSet := map[string]bool{}
	for _, item := range domainList {
		if profileCodeSet[item.Profile.ProfileCode] {
			t.Fatalf("档案编码重复：%s", item.Profile.ProfileCode)
		}
		profileCodeSet[item.Profile.ProfileCode] = true
		for _, skill := range item.SkillList {
			if skillCodeSet[skill.CommandCode] {
				t.Fatalf("技能编码重复：%s", skill.CommandCode)
			}
			skillCodeSet[skill.CommandCode] = true
		}
	}

	// 步骤3：模板可渲染且指纹稳定。占位符未登记会让渲染后变量为空；
	// 指纹格式变化会让历史资产被误判为「内容已变」而全量重播种。
	for _, item := range domainList {
		for _, skill := range item.SkillList {
			if unknown := TemplatePlaceholders(skill.PromptTemplate, BuiltinPlaceholders); len(unknown) > 0 {
				t.Fatalf("技能 %s 使用了未登记的占位符：%v", skill.CommandCode, unknown)
			}
			if len(skill.ContentHash()) != 64 {
				t.Fatalf("技能 %s 指纹异常", skill.CommandCode)
			}
		}
	}
}

// TestProviderInstanceChain 覆盖模型网关的实例构造链路。
func TestProviderInstanceChain(t *testing.T) {
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
