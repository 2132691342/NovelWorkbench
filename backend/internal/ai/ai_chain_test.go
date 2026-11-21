// AI 能力链路回归：内置创作资产完整性 + 模型网关实例策略。
// 资产约束破坏会让播种出的技能无法保存或渲染；网关策略配错会让换密钥后继续用旧凭据、长流被整体超时拦腰切断。
// 两者都是「配错才在生产暴露」的问题，因此合并到本文件按链路组织。

package ai

import (
	"net/http"
	"testing"
	"time"
)

// TestBuiltinAssetIntegrity 覆盖内置资产的完整链路：域形状 → 编码唯一 → 占位符可解析 → 指纹稳定 → 默认档案。
func TestBuiltinAssetIntegrity(t *testing.T) {
	// 步骤1：域形状。每个流程域必须有名称、一份档案与两个技能。
	domains := BuiltinDomains()
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

	// 步骤2：编码唯一。技能编码与档案编码都必须全局唯一。
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

	// 步骤3：模板可渲染。占位符必须已登记，模板长度足以承载判据，内容指纹稳定。
	for _, domain := range domains {
		for _, skill := range domain.SkillList {
			unknown := TemplatePlaceholders(skill.PromptTemplate, BuiltinPlaceholders)
			if len(unknown) > 0 {
				t.Fatalf("技能 %s 使用了未登记的占位符：%v", skill.CommandCode, unknown)
			}
			if len(skill.PromptTemplate) < 200 {
				t.Fatalf("技能 %s 模板过短（%d 字符），不足以承载判据", skill.CommandCode, len(skill.PromptTemplate))
			}
			if len(skill.ContentHash()) != 64 {
				t.Fatalf("技能 %s 指纹异常", skill.CommandCode)
			}
		}
		if domain.Profile.SortOrder <= 0 {
			t.Fatalf("档案 %s 排序号必须为正", domain.Profile.ProfileCode)
		}
	}

	// 步骤4：默认档案取排序号最小者，必须是起草域的连载作者。
	defaultProfile := BuiltinPromptProfiles()[0]
	for _, profile := range BuiltinPromptProfiles() {
		if profile.SortOrder < defaultProfile.SortOrder {
			defaultProfile = profile
		}
	}
	if defaultProfile.ProfileCode != "novelist" {
		t.Fatalf("默认档案应为 novelist，实际 %s", defaultProfile.ProfileCode)
	}
}

// TestProviderInstancePolicy 覆盖模型网关的实例构造策略：缓存键区分凭据与参数 → 流式客户端不设整体超时。
func TestProviderInstancePolicy(t *testing.T) {
	// 步骤1：缓存键。密钥、协议、地址、模型、温度、输出上限、超时任一变化都必须换键，
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
		changed := base
		variant.mutate(&changed)
		if base.cacheKey() == changed.cacheKey() {
			t.Fatalf("%s 后缓存键未变化，实例会被错误复用", variant.caseName)
		}
	}
	if base.cacheKey() != base.cacheKey() {
		t.Fatal("同一份规格必须产生稳定的缓存键")
	}

	// 步骤2：流式客户端。整体超时会拦腰切断长流，只能靠连接/首包超时与调用方 ctx 控制。
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
