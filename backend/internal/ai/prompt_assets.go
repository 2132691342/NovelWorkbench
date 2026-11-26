// 本文件定义内置提示词资产：3 个流程域 × (1 份身份 + 2 份任务) = 9 条。
// 刻意保持少而清晰：作者要能一眼看懂每个 /命令 干什么，而不是在几十条方法论里挑。
// 复杂流程交给工作流引擎编排，提示词只负责「这一单活怎么干」。

package ai

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
)

// 提示词种类编码：身份提示词整段会话常驻，任务提示词由 /编码 单次触发。
const (
	PromptKindSystem  = "system"  // 身份与底线（系统级消息）
	PromptKindCommand = "command" // 单次任务（系统级消息，/编码 触发）
)

// BuiltinCommandPrompt 内置任务提示词（单次任务，对话中以 /prompt_code 触发）。
type BuiltinCommandPrompt struct {
	PromptCode      string // 任务编码（对话中以 /编码 触发，工作流按编码引用）
	PromptName      string // 任务名称
	CategoryCode    string // 分类：write/revise/outline/setting/review/other
	DescriptionText string // 一句话说明
	PromptText      string // 完整提示词正文（可含 {{context}} 等占位符）
	SortOrder       int    // 排序号
}

// BuiltinDomain 创作流程域：身份提示词定「以什么身份说话」，两份任务定「这一单活怎么干」。
type BuiltinDomain struct {
	DomainCode       string                 // 域编码（落库与分组用）
	DomainName       string                 // 域名称
	SystemPromptCode string                 // 身份提示词编码
	SystemPromptName string                 // 身份提示词名称
	SystemPromptText string                 // 身份提示词正文
	SortOrder        int                    // 域排序号（同时作为身份提示词排序号）
	CommandList      []BuiltinCommandPrompt // 域的两份任务提示词
}

// builtinDomainList 内置流程域清单，按创作作业顺序排列：写正文 → 修问题 → 做规划。
var builtinDomainList = []BuiltinDomain{
	domainDraft,
	domainRevise,
	domainPlan,
}

// BuiltinDomains 返回内置流程域定义（只读副本，任务列表逐项复制）。
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

// BuiltinCommandPrompts 返回全部内置任务提示词定义（只读副本）。
func BuiltinCommandPrompts() []BuiltinCommandPrompt {
	prompts := make([]BuiltinCommandPrompt, 0, len(builtinDomainList)*2)
	for i := range builtinDomainList {
		prompts = append(prompts, builtinDomainList[i].CommandList...)
	}
	return prompts
}

// BuiltinCommandCodeSet 返回内置任务编码集合（用于播种时清理已下线任务）。
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

// BuiltinCommandHashOf 返回指定内置任务的出厂指纹；不在内置清单中返回空串。
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

// BuiltinSystemHashOf 返回指定内置身份的出厂指纹；不在内置清单中返回空串。
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

// ContentHash 计算代码资产中任务提示词的指纹。
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
