package ai

import (
	"strings"
	"testing"
)

// TestAssembleContext 覆盖预算裁剪链路：优先级保留、超预算丢弃、确定性输出。
func TestAssembleContext(t *testing.T) {
	blocks := []ContextBlock{
		{BlockKey: "history", Priority: PriorityHistory, ContentText: strings.Repeat("h", 100)},
		{BlockKey: "rag", Priority: PriorityRAG, ContentText: strings.Repeat("r", 100)},
		{BlockKey: "skill", Priority: PrioritySkill, ContentText: strings.Repeat("s", 100)},
		{BlockKey: "chapter_summary", Priority: PriorityChapterSummary, ContentText: strings.Repeat("c", 100)},
		{BlockKey: "empty", Priority: PriorityCharacter, ContentText: ""},
	}

	t.Run("预算充足时全量保留且空块跳过", func(t *testing.T) {
		assembled, dropped := AssembleContext(blocks, 1000)
		if len(dropped) != 0 {
			t.Fatalf("期望无丢弃，实际 %v", dropped)
		}
		if !strings.HasPrefix(assembled, strings.Repeat("s", 100)) {
			t.Fatalf("期望高优先级 skill 块在最前")
		}
		if strings.Contains(assembled, "e") {
			t.Fatalf("空块不应进入输出")
		}
	})

	t.Run("预算不足时按优先级从低到高丢弃", func(t *testing.T) {
		assembled, dropped := AssembleContext(blocks, 250)
		if len(dropped) != 2 {
			t.Fatalf("期望丢弃 2 块，实际 %v", dropped)
		}
		droppedSet := strings.Join(dropped, ",")
		if !strings.Contains(droppedSet, "history") || !strings.Contains(droppedSet, "chapter_summary") {
			t.Fatalf("期望丢弃 history 与 chapter_summary，实际 %v", dropped)
		}
		if !strings.Contains(assembled, strings.Repeat("s", 100)) || !strings.Contains(assembled, strings.Repeat("r", 100)) {
			t.Fatalf("skill 与 rag 块必须保留")
		}
	})

	t.Run("同优先级保持输入顺序", func(t *testing.T) {
		samePriority := []ContextBlock{
			{BlockKey: "a", Priority: 50, ContentText: "AAA"},
			{BlockKey: "b", Priority: 50, ContentText: "BBB"},
		}
		assembled, _ := AssembleContext(samePriority, 100)
		if strings.Index(assembled, "AAA") > strings.Index(assembled, "BBB") {
			t.Fatalf("同优先级应保持输入顺序，实际 %q", assembled)
		}
	})
}

// TestRenderPrompt 覆盖模板渲染链路：占位符替换与缺失占位符降级。
func TestRenderPrompt(t *testing.T) {
	rendered := RenderPrompt("上下文：{{context}}；正文：{{chapter_content}}；缺失：{{missing}}",
		map[string]string{"context": "CTX", "chapter_content": "BODY"})
	if rendered != "上下文：CTX；正文：BODY；缺失：" {
		t.Fatalf("渲染结果不符：%q", rendered)
	}
}
