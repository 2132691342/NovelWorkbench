package ai

import (
	"strings"
	"testing"
)

// TestContextAssemblyChain 覆盖上下文装配链路：优先级排序、超预算丢弃、空块跳过、同优先级稳定、占位符渲染。
// 装配顺序与预算裁剪直接决定模型能看到什么，任一规则变化都会影响所有 AI 产出，因此合并为一条用例。
func TestContextAssemblyChain(t *testing.T) {
	blocks := []ContextBlock{
		{BlockKey: "history", Priority: PriorityHistory, ContentText: strings.Repeat("h", 100)},
		{BlockKey: "rag", Priority: PriorityRAG, ContentText: strings.Repeat("r", 100)},
		{BlockKey: "skill", Priority: PrioritySkill, ContentText: strings.Repeat("s", 100)},
		{BlockKey: "chapter_summary", Priority: PriorityChapterSummary, ContentText: strings.Repeat("c", 100)},
		{BlockKey: "empty", Priority: PriorityCharacter, ContentText: ""},
	}

	assembled, dropped := AssembleContext(blocks, 1000)
	if len(dropped) != 0 {
		t.Fatalf("预算充足时不应丢弃块，实际 %v", dropped)
	}
	if !strings.HasPrefix(assembled, strings.Repeat("s", 100)) {
		t.Fatalf("最高优先级 skill 块应排在最前: %q", assembled)
	}
	// 4 个非空块各 100 字符 + 3 个块间分隔符，尾部空白已裁剪；空块被跳过
	if len(assembled) != 406 {
		t.Fatalf("空块不应进入装配结果，len = %d, want 406", len(assembled))
	}

	assembled, dropped = AssembleContext(blocks, 250)
	if len(dropped) != 2 {
		t.Fatalf("超预算应丢弃 2 块，实际 %v", dropped)
	}
	droppedText := strings.Join(dropped, ",")
	if !strings.Contains(droppedText, "history") || !strings.Contains(droppedText, "chapter_summary") {
		t.Fatalf("应按优先级从低到高丢弃，实际 %v", dropped)
	}
	if !strings.Contains(assembled, strings.Repeat("s", 100)) || !strings.Contains(assembled, strings.Repeat("r", 100)) {
		t.Fatal("高优先级块必须保留")
	}

	samePriority := []ContextBlock{
		{BlockKey: "a", Priority: 50, ContentText: "AAA"},
		{BlockKey: "b", Priority: 50, ContentText: "BBB"},
	}
	assembled, _ = AssembleContext(samePriority, 100)
	if strings.Index(assembled, "AAA") > strings.Index(assembled, "BBB") {
		t.Fatalf("同优先级应保持输入顺序: %q", assembled)
	}

	rendered := RenderPrompt("上下文：{{context}}；正文：{{chapter_content}}；缺失：{{missing}}",
		map[string]string{"context": "CTX", "chapter_content": "BODY"})
	if rendered != "上下文：CTX；正文：BODY；缺失：" {
		t.Fatalf("占位符渲染结果不符: %q", rendered)
	}

	if !strings.Contains(SystemPrompt("冷峻文风"), "冷峻文风") {
		t.Fatal("系统提示词应注入项目文风约束")
	}
}
