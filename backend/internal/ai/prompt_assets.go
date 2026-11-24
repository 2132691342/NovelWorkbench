// 本文件定义内置提示词资产的结构：一个流程域 = 1 份身份提示词 + 2 份完整指令提示词，
// 并提供注册表、内容指纹与只读访问器（播种与增量同步共用同一指纹算法）。

package ai

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
)

// 提示词种类编码：身份提示词整段会话常驻，指令提示词由 /编码 单次触发。
const (
	PromptKindSystem  = "system"  // 身份与底线（系统级消息）
	PromptKindCommand = "command" // 单次任务指令（系统级消息，/编码 触发）
)

// BuiltinCommandPrompt 内置指令提示词（单次任务，对话中以 /prompt_code 触发）。
type BuiltinCommandPrompt struct {
	PromptCode      string // 指令编码（对话中以 /编码 触发）
	PromptName      string // 指令名称
	CategoryCode    string // 分类：write/revise/outline/setting/review/other
	DescriptionText string // 一句话说明
	PromptText      string // 完整提示词正文（可含 {{context}} 等占位符）
	SortOrder       int    // 排序号
}

// BuiltinDomain 创作流程域：身份提示词定「以什么身份说话」，两份指令定「这一单活怎么干」。
type BuiltinDomain struct {
	DomainCode       string                 // 域编码（落库与分组用）
	DomainName       string                 // 域名称
	SystemPromptCode string                 // 身份提示词编码
	SystemPromptName string                 // 身份提示词名称
	SystemPromptText string                 // 身份提示词正文
	SortOrder        int                    // 域排序号（同时作为身份提示词排序号）
	CommandList      []BuiltinCommandPrompt // 域的两份指令提示词
}

// builtinDomainList 内置流程域清单，按创作作业顺序排列：
// 立项 → 架构 → 起草 → 打磨 → 去 AI 味 → 质检 → 连续性 → 定稿 → 上架 → 平台运营 → 短篇 → 改编。
var builtinDomainList = []BuiltinDomain{
	domainPitch,
	domainArchitect,
	domainDraft,
	domainStyle,
	domainDeAI,
	domainReview,
	domainContinuity,
	domainFinalize,
	domainOperation,
	domainPublish,
	domainShort,
	domainAdapt,
}

// BuiltinDomains 返回内置流程域定义（只读副本，指令列表逐项复制）。
func BuiltinDomains() []BuiltinDomain {
	domains := make([]BuiltinDomain, len(builtinDomainList))
	for i := range builtinDomainList {
		domains[i] = builtinDomainList[i]
		domains[i].CommandList = append([]BuiltinCommandPrompt(nil), builtinDomainList[i].CommandList...)
	}
	return domains
}

// BuiltinSystemPrompts 返回全部内置身份提示词定义（只读副本）。
func BuiltinSystemPrompts() []BuiltinDomain {
	domains := make([]BuiltinDomain, len(builtinDomainList))
	copy(domains, builtinDomainList)
	return domains
}

// BuiltinCommandPrompts 返回全部内置指令提示词定义（只读副本）。
func BuiltinCommandPrompts() []BuiltinCommandPrompt {
	prompts := make([]BuiltinCommandPrompt, 0, len(builtinDomainList)*2)
	for i := range builtinDomainList {
		prompts = append(prompts, builtinDomainList[i].CommandList...)
	}
	return prompts
}

// BuiltinCommandCodeSet 返回内置指令编码集合（用于播种时清理已下线指令）。
func BuiltinCommandCodeSet() map[string]bool {
	codeSet := make(map[string]bool, len(builtinDomainList)*2)
	for i := range builtinDomainList {
		for _, command := range builtinDomainList[i].CommandList {
			codeSet[command.PromptCode] = true
		}
	}
	return codeSet
}

// BuiltinSystemCodeSet 返回内置身份提示词编码集合（用于播种时清理已下线身份）。
func BuiltinSystemCodeSet() map[string]bool {
	codeSet := make(map[string]bool, len(builtinDomainList))
	for i := range builtinDomainList {
		codeSet[builtinDomainList[i].SystemPromptCode] = true
	}
	return codeSet
}

// BuiltinCommandHashOf 返回指定内置指令提示词的出厂指纹；不在内置清单中返回空串。
func BuiltinCommandHashOf(promptCode string) string {
	for i := range builtinDomainList {
		for _, command := range builtinDomainList[i].CommandList {
			if command.PromptCode == promptCode {
				return command.ContentHash()
			}
		}
	}
	return ""
}

// BuiltinSystemHashOf 返回指定内置身份提示词的出厂指纹；不在内置清单中返回空串。
func BuiltinSystemHashOf(promptCode string) string {
	for i := range builtinDomainList {
		if builtinDomainList[i].SystemPromptCode == promptCode {
			return builtinDomainList[i].SystemContentHash()
		}
	}
	return ""
}

// BuiltinDomainOf 返回提示词编码所属流程域（编码与名称），非内置提示词返回空串。
func BuiltinDomainOf(promptCode string) (string, string) {
	for i := range builtinDomainList {
		if builtinDomainList[i].SystemPromptCode == promptCode {
			return builtinDomainList[i].DomainCode, builtinDomainList[i].DomainName
		}
		for _, command := range builtinDomainList[i].CommandList {
			if command.PromptCode == promptCode {
				return builtinDomainList[i].DomainCode, builtinDomainList[i].DomainName
			}
		}
	}
	return "", ""
}

// DomainNameOf 返回流程域展示名；未知编码返回空串。
func DomainNameOf(domainCode string) string {
	for i := range builtinDomainList {
		if builtinDomainList[i].DomainCode == domainCode {
			return builtinDomainList[i].DomainName
		}
	}
	return ""
}

// PromptAssetHash 计算提示词内容指纹：代码资产与数据库行共用此算法，
// 指纹一致说明「库里的内容就是这一版代码资产」，无需覆写。
func PromptAssetHash(promptCode, promptName, promptKindCode, categoryCode, descriptionText, promptText string, sortOrder int) string {
	return assetHash(strings.Join([]string{
		"prompt", promptCode, promptName, promptKindCode, categoryCode, descriptionText,
		strconv.Itoa(sortOrder), promptText,
	}, "\x1f"))
}

// ContentHash 计算代码资产中指令提示词的指纹。
func (c BuiltinCommandPrompt) ContentHash() string {
	return PromptAssetHash(c.PromptCode, c.PromptName, PromptKindCommand, c.CategoryCode,
		c.DescriptionText, c.PromptText, c.SortOrder)
}

// SystemContentHash 计算代码资产中身份提示词的指纹。
func (d BuiltinDomain) SystemContentHash() string {
	return PromptAssetHash(d.SystemPromptCode, d.SystemPromptName, PromptKindSystem, "other",
		d.DomainName+"：身份与底线", d.SystemPromptText, d.SortOrder)
}

// assetHash 对规范化文本取 sha256 十六进制摘要。
func assetHash(text string) string {
	digest := sha256.Sum256([]byte(text))
	return hex.EncodeToString(digest[:])
}
