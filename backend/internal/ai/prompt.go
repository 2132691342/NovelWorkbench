// 本文件定义 AI 上下文的优先级体系与装配规则：上下文块、字符预算裁剪与提示词模板渲染。

package ai

import (
	"regexp"
	"sort"
	"strings"
)

// 上下文块优先级常量：数值越大越先获得预算（技能提示与当前章节不可丢弃）。
const (
	PrioritySkill          = 100 // 技能提示词模板
	PriorityCurrentChapter = 90  // 当前编辑章节正文
	PrioritySkillRef       = 85  // 技能参考资料（低于当前章节，高于显式引用）
	PriorityExplicitRef    = 80  // @显式引用的文档/章节
	PriorityRAG            = 70  // 知识库召回片段
	PriorityChapterSummary = 60  // 前文章节梗概
	PriorityCharacter      = 50  // 相关角色卡
	PriorityHistory        = 40  // 最近聊天历史
)

// ContextBudgetChars 上下文装配的字符预算（约 8k token 输入侧，宁少勿超保证稳定）。
const ContextBudgetChars = 12000

// ContextBlock 待注入的上下文块。
type ContextBlock struct {
	BlockKey    string // 块标识（如 rag/chapter_summary），用于丢弃统计
	Priority    int    // 优先级（见常量）
	ContentText string // 块内容
}

// AssembleContext 按优先级从高到低装配上下文，超出预算的块被丢弃并返回其标识。
// 同优先级按输入顺序保留，保证输出确定性。
func AssembleContext(blocks []ContextBlock, budgetChars int) (string, []string) {
	sorted := make([]ContextBlock, len(blocks))
	copy(sorted, blocks)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Priority > sorted[j].Priority })

	var builder strings.Builder
	droppedKeys := make([]string, 0)
	for _, block := range sorted {
		if block.ContentText == "" {
			continue // 空块直接跳过
		}
		if builder.Len()+len(block.ContentText) > budgetChars {
			droppedKeys = append(droppedKeys, block.BlockKey)
			continue
		}
		builder.WriteString(block.ContentText)
		builder.WriteString("\n\n")
	}
	return strings.TrimSpace(builder.String()), droppedKeys
}

// placeholderPattern 匹配 {{占位符}} 模板变量（包级预编译，避免热路径重复编译）。
var placeholderPattern = regexp.MustCompile(`\{\{\s*(\w+)\s*\}\}`)

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
	seen := make(map[string]bool, 4)
	for _, match := range placeholderPattern.FindAllStringSubmatch(template, -1) {
		key := match[1]
		if known[key] || seen[key] {
			continue
		}
		seen[key] = true
		unknown = append(unknown, key)
	}
	return unknown
}

// SystemPrompt 生成小说创作助手系统提示词（注入项目文风约束）。
func SystemPrompt(styleText string) string {
	var builder strings.Builder
	builder.WriteString("你是 NovelWorkbench 的小说创作助手，服务于长篇小说创作。\n")
	builder.WriteString("要求：直接输出创作内容本身，不解释过程、不加引号、不输出与创作无关的客套话。\n")
	builder.WriteString("保持人物、情节、设定与给定上下文一致；不确定的设定不要擅自编造为既定事实。\n")
	if styleText != "" {
		builder.WriteString("本项目文风约束：\n")
		builder.WriteString(styleText)
	}
	return builder.String()
}
