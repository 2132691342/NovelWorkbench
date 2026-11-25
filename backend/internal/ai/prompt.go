// 本文件定义 AI 上下文的优先级体系与装配规则：上下文块、单块收敛与提示词渲染。

package ai

import (
	"regexp"
	"sort"
	"strings"
)

// 上下文块优先级常量：数值越大越靠前（当前章节与显式引用永远完整保留）。
const (
	PriorityCurrentChapter = 100 // 当前编辑章节正文
	PriorityExplicitRef    = 90  // @显式引用的文档/章节/摘要/角色/设定/会话
	PriorityRAG            = 70  // 知识库召回片段
	PriorityChapterSummary = 60  // 前文章节梗概
	PriorityCharacter      = 50  // 项目角色卡与设定全量
)

// ContextBlock 待注入的上下文块。
type ContextBlock struct {
	BlockKey    string // 块标识（如 rag/chapter_summary），用于收敛统计
	Priority    int    // 优先级（见常量）
	ContentText string // 块内容
}

// AssembleContext 按优先级从高到低拼接全部上下文块，返回装配文本与被收敛的块标识。
// 不按总预算丢弃：作者引用的素材一律进上下文，只在单块过长时保留首尾（见 CondenseBlock）。
// 同优先级按输入顺序保留，保证输出确定性。
func AssembleContext(blocks []ContextBlock) (string, []string) {
	sorted := make([]ContextBlock, len(blocks))
	copy(sorted, blocks)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Priority > sorted[j].Priority })

	var builder strings.Builder
	condensedKeys := make([]string, 0)
	for _, block := range sorted {
		if strings.TrimSpace(block.ContentText) == "" {
			continue // 空块直接跳过
		}
		contentText := CondenseBlock(block.ContentText, block.Priority)
		if contentText != block.ContentText {
			condensedKeys = append(condensedKeys, block.BlockKey)
		}
		builder.WriteString(contentText)
		builder.WriteString("\n\n")
	}
	return strings.TrimSpace(builder.String()), condensedKeys
}

// 单块收敛上限（rune）：显式引用给足原文，背景类素材收紧一些。
// 数值刻意宽松：宁可上下文长，也不把成段素材压成摘要句。
const (
	blockRunesExplicit = 20000 // 当前章与显式引用
	blockRunesDefault  = 6000  // 召回、梗概、角色与设定
)

// CondenseBlock 按块优先级收敛单块正文：未超上限原样返回，超长保留首尾并标注省略字数。
func CondenseBlock(contentText string, priority int) string {
	limit := blockRunesDefault
	if priority >= PriorityExplicitRef {
		limit = blockRunesExplicit
	}
	return CondenseRunes(contentText, limit*3/5, limit*2/5)
}

// placeholderPattern 匹配 {{占位符}} 模板变量（包级预编译，避免热路径重复编译）。
var placeholderPattern = regexp.MustCompile(`\{\{\s*(\w+)\s*\}\}`)

// BuiltinPlaceholders 内置占位符白名单：这些变量由系统在装配上下文时自动注入，
// 提示词可直接使用而无需额外登记。
var BuiltinPlaceholders = map[string]bool{
	"user_input": true, "chapter_content": true, "context": true, "prompt_code": true,
}

// RenderPrompt 渲染 {{占位符}} 模板：命中 valueMap 替换为值，缺失占位符替换为空串。
func RenderPrompt(template string, valueMap map[string]string) string {
	return placeholderPattern.ReplaceAllStringFunc(template, func(match string) string {
		key := placeholderPattern.ReplaceAllString(match, "$1")
		return valueMap[key]
	})
}

// TemplatePlaceholders 返回模板中未登记在 known 里的占位符（去重、按出现顺序）。
func TemplatePlaceholders(template string, known map[string]bool) []string {
	unknown := make([]string, 0, 4)
	for _, match := range placeholderPattern.FindAllStringSubmatch(template, -1) {
		key := match[1]
		if known[key] {
			continue
		}
		duplicated := false
		for _, existing := range unknown {
			if existing == key {
				duplicated = true
				break
			}
		}
		if !duplicated {
			unknown = append(unknown, key)
		}
	}
	return unknown
}

// SystemPrompt 生成小说创作助手基础提示词（无身份提示词时的兜底，注入项目文风约束）。
func SystemPrompt(styleText string) string {
	return ComposeSystemPrompt("", styleText)
}

// ComposeSystemPrompt 组合最终系统提示词：身份提示词正文（为空用内置兜底）+ 项目文风约束。
// 身份提示词决定身份与底线，文风约束属于项目事实，二者始终同时下发。
func ComposeSystemPrompt(profileText, styleText string) string {
	var builder strings.Builder
	if strings.TrimSpace(profileText) == "" {
		builder.WriteString("你是 NovelWorkbench 的小说创作助手，服务于长篇小说创作。\n")
		builder.WriteString("要求：直接输出创作内容本身，不解释过程、不加引号、不输出与创作无关的客套话。\n")
		builder.WriteString("保持人物、情节、设定与给定上下文一致；不确定的设定不要擅自编造为既定事实。\n")
	} else {
		builder.WriteString(strings.TrimSpace(profileText))
		builder.WriteString("\n")
	}
	if styleText != "" {
		builder.WriteString("\n本项目文风约束：\n")
		builder.WriteString(styleText)
	}
	return builder.String()
}
