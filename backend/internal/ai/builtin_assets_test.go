// 内置创作资产的完整性回归：域形状 → 编码唯一 → 模板可渲染 → 指纹稳定。
// 这四步串成一条播种链路：任何一步被破坏，播种出的技能都无法在对话中正常触发，
// 因此合并为一条用例，失败时按步骤注释定位。

package ai

import (
	"net/http"
	"testing"
	"time"
)

// TestBuiltinAssetChain 覆盖内置资产从定义到可播种的完整链路。
func TestBuiltinAssetChain(t *testing.T) {
	domains := BuiltinDomains()

	// 步骤1：域形状。每个流程域必须有名称、一份档案与两个技能。
	if len(domains) < 8 {
		t.Fatalf("内置流程域不足 8 个：%d", len(domains))
	}
	for _, domain := range domains {
		if domain.DomainCode == "" || domain.DomainName == "" {
			t.Fatalf("流程域缺少编码或名称：%+v", domain)
		}
		if domain.Profile.ProfileCode == "" || domain.Profile.SystemPrompt == "" {
			t.Fatalf("域 %s 缺少档案或档案正文", domain.DomainCode)
		}
		if len(domain.SkillList) != 2 {
			t.Fatalf("域 %s 的技能数应为 2，实际 %d", domain.DomainCode, len(domain.SkillList))
		}
	}

	// 步骤2：编码唯一。技能编码与档案编码都必须全局唯一，否则播种时互相覆盖。
	skillCodeSet := map[string]bool{}
	profileCodeSet := map[string]bool{}
	for _, domain := range domains {
		if profileCodeSet[domain.Profile.ProfileCode] {
			t.Fatalf("档案编码重复：%s", domain.Profile.ProfileCode)
		}
		profileCodeSet[domain.Profile.ProfileCode] = true
		for _, skill := range domain.SkillList {
			if skillCodeSet[skill.CommandCode] {
				t.Fatalf("技能编码重复：%s", skill.CommandCode)
			}
			skillCodeSet[skill.CommandCode] = true
		}
	}

	// 步骤3：模板可渲染。占位符必须已登记，否则渲染后变量为空，技能会丢判据；
	// 指纹必须是 sha256 十六进制，格式变了会导致历史资产被误判为"内容已变"而全量重播种。
	for _, domain := range domains {
		for _, skill := range domain.SkillList {
			unknown := TemplatePlaceholders(skill.PromptTemplate, BuiltinPlaceholders)
			if len(unknown) > 0 {
				t.Fatalf("技能 %s 使用了未登记的占位符：%v", skill.CommandCode, unknown)
			}
			if len(skill.ContentHash()) != 64 {
				t.Fatalf("技能 %s 指纹异常", skill.CommandCode)
			}
		}
	}
}

// TestProviderInstanceChain 覆盖模型网关的实例构造链路：缓存键 → 流式客户端超时策略。
// 缓存键漏掉任一维度会让换密钥后继续用旧凭据；整体超时会拦腰切断长流。
func TestProviderInstanceChain(t *testing.T) {
	// 步骤1：缓存键。密钥、协议、地址、模型、温度、输出上限、超时任一变化都必须换键。
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
		changed := base
		variant.mutate(&changed)
		if base.cacheKey() == changed.cacheKey() {
			t.Fatalf("%s 后缓存键未变化，实例会被错误复用", variant.caseName)
		}
	}

	// 步骤2：流式客户端。整体超时会拦腰切断长流，只能靠首包超时与调用方 ctx 控制。
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
}
