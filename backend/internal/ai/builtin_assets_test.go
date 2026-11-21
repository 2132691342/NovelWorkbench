// 内置创作资产的完整性校验：流程域形状、编码唯一性、模板可渲染性。
// 这些约束一旦破坏，播种出的资产会无法保存、无法渲染或让前端按钮打到空技能上，因此整体作为一条用例守住。

package ai

import (
	"testing"
)

// knownPlaceholders 校验模板占位符用的白名单（与 service.validateSkillTemplate 的 known 集合一致）。
var knownPlaceholders = map[string]bool{
	"user_input": true, "chapter_content": true, "context": true, "skill_code": true,
}

// TestBuiltinAssetIntegrity 覆盖内置资产的完整链路：域形状 → 编码唯一 → 占位符可解析 → 默认档案正确。
func TestBuiltinAssetIntegrity(t *testing.T) {
	// 环节1：域形状。每个流程域必须有名称、一份档案与两个技能。
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

	// 环节2：编码唯一。技能编码与档案编码都必须全局唯一。
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
	// 前端快捷入口依赖固定编码，改动会让按钮打到空技能上。
	for _, required := range []string{"continue", "polish", "summary", "deai", "review"} {
		if !skillCodeSet[required] {
			t.Fatalf("缺少前端依赖的技能编码：%s", required)
		}
	}

	// 环节3：模板可渲染。占位符必须已登记，模板长度足以承载判据，指纹稳定。
	for _, domain := range domains {
		for _, skill := range domain.SkillList {
			unknown := TemplatePlaceholders(skill.PromptTemplate, knownPlaceholders)
			if len(unknown) > 0 {
				t.Fatalf("技能 %s 使用了未登记的占位符：%v", skill.CommandCode, unknown)
			}
			if len(skill.PromptTemplate) < 200 {
				t.Fatalf("技能 %s 模板过短（%d 字符），不足以承载判据", skill.CommandCode, len(skill.PromptTemplate))
			}
			if skill.ContentHash() == "" || len(skill.ContentHash()) != 64 {
				t.Fatalf("技能 %s 指纹异常", skill.CommandCode)
			}
		}
		if domain.Profile.SortOrder <= 0 {
			t.Fatalf("档案 %s 排序号必须为正", domain.Profile.ProfileCode)
		}
	}

	// 环节4：默认档案取排序号最小者，必须是起草域的连载作者。
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
