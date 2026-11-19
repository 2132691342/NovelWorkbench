// 本文件覆盖 AI 上下文的装配链路：优先级排序 → 空块跳过 → 超预算整块丢弃 → 模板渲染 → 系统提示词。
// 装配顺序与预算裁剪直接决定模型能看到什么，任一规则变化都会影响所有 AI 产出，因此合并为一条用例。

package ai

import (
	"strings"
	"testing"
)

// TestContextAssemblyChain 覆盖上下文装配链路的全部环节。
func TestContextAssemblyChain(t *testing.T) {
	blocks := []ContextBlock{
		{BlockKey: "history", Priority: PriorityHistory, ContentText: strings.Repeat("h", 100)},
		{BlockKey: "rag", Priority: PriorityRAG, ContentText: strings.Repeat("r", 100)},
		{BlockKey: "skill", Priority: PrioritySkill, ContentText: strings.Repeat("s", 100)},
		{BlockKey: "chapter_summary", Priority: PriorityChapterSummary, ContentText: strings.Repeat("c", 100)},
		{BlockKey: "empty", Priority: PriorityCharacter, ContentText: ""},
	}

	// 预算充足：按优先级降序装配，空块不占预算。
	assembled, dropped := AssembleContext(blocks, 1000)
	if len(dropped) != 0 {
		t.Fatalf("预算充足时不应丢弃块，实际 %v", dropped)
	}
	if !strings.HasPrefix(assembled, strings.Repeat("s", 100)) {
		t.Fatalf("最高优先级 skill 块应排在最前: %q", assembled)
	}
	if strings.Contains(assembled, "empty") {
		t.Fatal("空块不应进入装配结果")
	}
	for _, key := range []string{"rag", "chapter_summary", "history"} {
		if !strings.Contains(assembled, strings.Repeat(map[string]string{
			"rag": "r", "chapter_summary": "c", "history": "h",
		}[key], 100)) {
			t.Fatalf("块 %s 未进入装配结果", key)
		}
	}

	// 预算不足：从低优先级开始整块丢弃，高优先级块必须保留。
	assembled, dropped = AssembleContext(blocks, 250)
	droppedText := strings.Join(dropped, ",")
	if !strings.Contains(droppedText, "history") || !strings.Contains(droppedText, "chapter_summary") {
		t.Fatalf("应按优先级从低到高丢弃，实际 %v", dropped)
	}
	if !strings.Contains(assembled, strings.Repeat("s", 100)) || !strings.Contains(assembled, strings.Repeat("r", 100)) {
		t.Fatal("高优先级块必须保留")
	}
	if strings.Contains(assembled, strings.Repeat("h", 100)) {
		t.Fatal("被丢弃块不应进入装配结果")
	}

	// 同优先级保持输入顺序，保证输出确定性。
	samePriority := []ContextBlock{
		{BlockKey: "a", Priority: 50, ContentText: "AAA"},
		{BlockKey: "b", Priority: 50, ContentText: "BBB"},
	}
	assembled, _ = AssembleContext(samePriority, 100)
	if strings.Index(assembled, "AAA") > strings.Index(assembled, "BBB") {
		t.Fatalf("同优先级应保持输入顺序: %q", assembled)
	}

	// 模板渲染：命中的占位符替换为值，缺失的替换为空串。
	rendered := RenderPrompt("上下文：{{context}}；正文：{{chapter_content}}；缺失：{{missing}}",
		map[string]string{"context": "CTX", "chapter_content": "BODY"})
	if rendered != "上下文：CTX；正文：BODY；缺失：" {
		t.Fatalf("占位符渲染结果不符: %q", rendered)
	}

	// 系统提示词注入项目文风约束。
	if !strings.Contains(SystemPrompt("冷峻文风"), "冷峻文风") {
		t.Fatal("系统提示词应注入项目文风约束")
	}
}
