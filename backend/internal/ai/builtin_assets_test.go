// 本文件校验内置创作资产的完整性：每个流程域一份档案 + 两个技能，编码唯一、
// 占位符已登记、指纹稳定，避免播种出无法保存或无法渲染的资产。

package ai

import (
	"testing"
)

// knownPlaceholders 校验模板占位符用的白名单（与 service.validateSkillTemplate 的 known 集合一致）。
var knownPlaceholders = map[string]bool{
	"user_input": true, "chapter_content": true, "context": true, "skill_code": true,
}

// TestBuiltinDomainShape 每个域必须有名称、一份档案与两个技能。
func TestBuiltinDomainShape(t *testing.T) {
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
}

// TestBuiltinCodesUnique 技能编码与档案编码都必须全局唯一。
func TestBuiltinCodesUnique(t *testing.T) {
	skillCodeSet := map[string]bool{}
	profileCodeSet := map[string]bool{}
	for _, domain := range BuiltinDomains() {
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
}

// TestBuiltinTemplatesRenderable 模板占位符必须可解析，且默认档案排序号最小。
func TestBuiltinTemplatesRenderable(t *testing.T) {
	firstSortOrder := -1
	for _, profile := range BuiltinPromptProfiles() {
		if firstSortOrder < 0 || profile.SortOrder < firstSortOrder {
			firstSortOrder = profile.SortOrder
		}
	}
	for _, domain := range BuiltinDomains() {
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
	// 默认档案取排序号最小者，必须是起草域的连载作者。
	defaultProfile := BuiltinDomains()[0].Profile
	for _, profile := range BuiltinPromptProfiles() {
		if profile.SortOrder < defaultProfile.SortOrder {
			defaultProfile = profile
		}
	}
	if defaultProfile.ProfileCode != "novelist" {
		t.Fatalf("默认档案应为 novelist，实际 %s", defaultProfile.ProfileCode)
	}
}
