// 本文件定义内置创作资产的结构：一个流程域 = 1 份系统提示词档案 + 2 个技能，
// 并提供注册表、代码资产指纹与只读访问器（播种与增量同步共用同一指纹算法）。

package ai

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
)

// BuiltinSkill 内置技能定义（单次任务指令，对话中以 /code 触发）。
type BuiltinSkill struct {
	CommandCode     string // 技能编码（对话中以 /code 触发）
	CommandName     string // 技能名称
	CategoryCode    string // 分类：write/revise/outline/setting/review/other
	DescriptionText string // 一句话说明
	PromptTemplate  string // 提示词模板（{{占位符}}）
	SortOrder       int    // 排序号
}

// BuiltinPromptProfile 内置系统提示词档案（AI 的身份与底线，与技能分工）。
type BuiltinPromptProfile struct {
	ProfileCode     string // 档案编码
	ProfileName     string // 档案名
	DescriptionText string // 一句话说明
	SystemPrompt    string // 系统提示词正文
	SortOrder       int    // 排序号
}

// BuiltinDomain 创作流程域：档案定身份，两个技能定动作，缺一不成立。
type BuiltinDomain struct {
	DomainCode string               // 域编码（文档与测试用，不落库）
	DomainName string               // 域名称
	Profile    BuiltinPromptProfile // 域的系统提示词档案
	SkillList  []BuiltinSkill       // 域的两个技能（顺序即展示顺序）
}

// builtinDomainList 内置流程域清单，按创作作业顺序排列：
// 起草 → 架构 → 打磨 → 去 AI 味 → 质检 → 连续性 → 定稿 → 上架。
var builtinDomainList = []BuiltinDomain{
	domainDraft,
	domainArchitect,
	domainStyle,
	domainDeAI,
	domainReview,
	domainContinuity,
	domainFinalize,
	domainOperation,
	domainPitch,
}

// BuiltinDomains 返回内置流程域定义（只读副本，技能列表逐项复制）。
func BuiltinDomains() []BuiltinDomain {
	domains := make([]BuiltinDomain, len(builtinDomainList))
	for i := range builtinDomainList {
		domains[i] = builtinDomainList[i]
		domains[i].SkillList = append([]BuiltinSkill(nil), builtinDomainList[i].SkillList...)
	}
	return domains
}

// BuiltinSkills 返回全部内置技能定义（只读副本）。
func BuiltinSkills() []BuiltinSkill {
	skills := make([]BuiltinSkill, 0, len(builtinDomainList)*2)
	for i := range builtinDomainList {
		skills = append(skills, builtinDomainList[i].SkillList...)
	}
	return skills
}

// BuiltinPromptProfiles 返回全部内置档案定义（只读副本）。
func BuiltinPromptProfiles() []BuiltinPromptProfile {
	profiles := make([]BuiltinPromptProfile, 0, len(builtinDomainList))
	for i := range builtinDomainList {
		profiles = append(profiles, builtinDomainList[i].Profile)
	}
	return profiles
}

// BuiltinSkillCodeSet 返回内置技能编码集合（用于播种时清理已下线技能）。
func BuiltinSkillCodeSet() map[string]bool {
	codeSet := make(map[string]bool, len(builtinDomainList)*2)
	for i := range builtinDomainList {
		for _, skill := range builtinDomainList[i].SkillList {
			codeSet[skill.CommandCode] = true
		}
	}
	return codeSet
}

// BuiltinSkillHashOf 返回指定内置技能的出厂指纹；不在内置清单中返回空串。
func BuiltinSkillHashOf(skillCode string) string {
	for i := range builtinDomainList {
		for _, skill := range builtinDomainList[i].SkillList {
			if skill.CommandCode == skillCode {
				return skill.ContentHash()
			}
		}
	}
	return ""
}

// BuiltinProfileHashOf 返回指定内置档案的出厂指纹；不在内置清单中返回空串。
func BuiltinProfileHashOf(profileCode string) string {
	for i := range builtinDomainList {
		if builtinDomainList[i].Profile.ProfileCode == profileCode {
			return builtinDomainList[i].Profile.ContentHash()
		}
	}
	return ""
}

// BuiltinSkillDomainOf 返回技能所属流程域（编码与名称），非内置技能返回空串。
func BuiltinSkillDomainOf(skillCode string) (string, string) {
	for i := range builtinDomainList {
		for _, skill := range builtinDomainList[i].SkillList {
			if skill.CommandCode == skillCode {
				return builtinDomainList[i].DomainCode, builtinDomainList[i].DomainName
			}
		}
	}
	return "", ""
}

// BuiltinProfileDomainOf 返回档案所属流程域（编码与名称），非内置档案返回空串。
func BuiltinProfileDomainOf(profileCode string) (string, string) {
	for i := range builtinDomainList {
		if builtinDomainList[i].Profile.ProfileCode == profileCode {
			return builtinDomainList[i].DomainCode, builtinDomainList[i].DomainName
		}
	}
	return "", ""
}

// SkillAssetHash 计算技能内容指纹：代码资产与数据库行共用此算法，
// 指纹一致说明「库里的内容就是这一版代码资产」，无需覆写。
func SkillAssetHash(skillCode, skillName, categoryCode, descriptionText, promptTemplate string, sortOrder int) string {
	return assetHash(strings.Join([]string{
		"skill", skillCode, skillName, categoryCode, descriptionText,
		strconv.Itoa(sortOrder), promptTemplate,
	}, "\x1f"))
}

// PromptProfileAssetHash 计算档案内容指纹（与技能同一算法，前缀区分类型）。
func PromptProfileAssetHash(profileCode, profileName, descriptionText, systemPrompt string, sortOrder int) string {
	return assetHash(strings.Join([]string{
		"profile", profileCode, profileName, descriptionText,
		strconv.Itoa(sortOrder), systemPrompt,
	}, "\x1f"))
}

// ContentHash 计算代码资产的技能指纹。
func (s BuiltinSkill) ContentHash() string {
	return SkillAssetHash(s.CommandCode, s.CommandName, s.CategoryCode, s.DescriptionText, s.PromptTemplate, s.SortOrder)
}

// ContentHash 计算代码资产的档案指纹。
func (p BuiltinPromptProfile) ContentHash() string {
	return PromptProfileAssetHash(p.ProfileCode, p.ProfileName, p.DescriptionText, p.SystemPrompt, p.SortOrder)
}

// assetHash 对规范化文本取 sha256 十六进制摘要。
func assetHash(text string) string {
	digest := sha256.Sum256([]byte(text))
	return hex.EncodeToString(digest[:])
}
