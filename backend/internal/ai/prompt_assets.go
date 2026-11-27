// 本文件定义内置提示词资产：4 个创作流程 × 2 份任务提示词 = 8 条，全部是 command 类。
// 刻意保持少而清晰：作者要能一眼看懂每个 /命令 干什么，而不是在几十条方法论里挑。
// 复杂流程交给工作流引擎编排，提示词只负责「这一单活怎么干」；身份不再内置，见 prompt_base.go。

package ai

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
)

// PromptKindCommand 任务提示词种类编码：内置资产只有这一种，由 /编码 单次触发。
const PromptKindCommand = "command"

// BuiltinCommandPrompt 内置任务提示词（单次任务，对话中以 /prompt_code 触发）。
type BuiltinCommandPrompt struct {
	PromptCode      string // 任务编码（对话中以 /编码 触发，工作流按编码引用）
	PromptName      string // 任务名称
	CategoryCode    string // 分类：write/revise/outline/setting/review/other
	DescriptionText string // 一句话说明：什么时候该用这一条
	PromptText      string // 完整提示词正文（可含 {{context}} 等占位符）
	SortOrder       int    // 排序号
}

// BuiltinDomain 创作流程定义：一个流程恰好两份任务提示词，作者按「我现在要干什么」选流程。
type BuiltinDomain struct {
	FlowCode    string                 // 流程编码（落库与分组用）
	FlowName    string                 // 流程名称（开书规划 / 写正文 / 修问题 / 质检）
	SortOrder   int                    // 流程排序号（按创作作业顺序）
	CommandList []BuiltinCommandPrompt // 流程的两份任务提示词
}

// builtinDomainList 内置流程清单，按创作作业顺序排列：开书规划 → 写正文 → 修问题 → 质检。
var builtinDomainList = []BuiltinDomain{
	domainPlan,
	domainDraft,
	domainRevise,
	domainCheck,
}

// BuiltinDomains 返回内置流程定义（只读副本，任务列表逐项复制）。
func BuiltinDomains() []BuiltinDomain {
	flows := make([]BuiltinDomain, len(builtinDomainList))
	for i := range builtinDomainList {
		flows[i] = builtinDomainList[i]
		flows[i].CommandList = append([]BuiltinCommandPrompt(nil), builtinDomainList[i].CommandList...)
	}
	return flows
}

// BuiltinCommandPrompts 返回全部内置任务提示词定义（只读副本）。
func BuiltinCommandPrompts() []BuiltinCommandPrompt {
	prompts := make([]BuiltinCommandPrompt, 0, len(builtinDomainList)*2)
	for i := range builtinDomainList {
		prompts = append(prompts, builtinDomainList[i].CommandList...)
	}
	return prompts
}

// BuiltinCommandCodeSet 返回内置任务编码集合（用于播种时清理已下线提示词，含已下线的身份类）。
func BuiltinCommandCodeSet() map[string]bool {
	codeSet := make(map[string]bool, len(builtinDomainList)*2)
	for i := range builtinDomainList {
		for _, command := range builtinDomainList[i].CommandList {
			codeSet[command.PromptCode] = true
		}
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

// BuiltinDomainOf 返回提示词编码所属流程（编码与名称），非内置提示词返回空串。
func BuiltinDomainOf(promptCode string) (string, string) {
	for i := range builtinDomainList {
		for _, command := range builtinDomainList[i].CommandList {
			if command.PromptCode == promptCode {
				return builtinDomainList[i].FlowCode, builtinDomainList[i].FlowName
			}
		}
	}
	return "", ""
}

// DomainNameOf 返回流程展示名；未知编码返回空串。
func DomainNameOf(domainCode string) string {
	for i := range builtinDomainList {
		if builtinDomainList[i].FlowCode == domainCode {
			return builtinDomainList[i].FlowName
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

// assetHash 对规范化文本取 sha256 十六进制摘要。
func assetHash(text string) string {
	digest := sha256.Sum256([]byte(text))
	return hex.EncodeToString(digest[:])
}
