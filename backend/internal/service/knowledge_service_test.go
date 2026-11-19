package service

import (
	"math"
	"strings"
	"testing"

	"github.com/2132691342/NovelWorkbench/backend/internal/ai"
	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
)

// searchHit 构造检索命中。
func searchHit(chunkULID string) domain.KnowledgeChunkVO {
	return domain.KnowledgeChunkVO{ChunkULID: chunkULID, ChunkText: chunkULID}
}

// TestKnowledgeRetrievalChain 覆盖无向量检索的完整链路：
// 文本切片 → 查询词抽取 → 检索模式归一 → RRF 融合排序 → chat 重排解析与 fail-open。
func TestKnowledgeRetrievalChain(t *testing.T) {
	// ---- 环节一：切片 ----
	chunks := chunkText("  短文本  ", 100, 10)
	if len(chunks) != 1 || chunks[0] != "短文本" {
		t.Fatalf("短文本应产出单切片: %v", chunks)
	}
	if chunks := chunkText("   \n  ", 100, 10); len(chunks) != 0 {
		t.Fatalf("空白文本不应产出切片: %v", chunks)
	}
	chunks = chunkText("abcdefghijklmnopqrstuvwxy", 10, 3)
	wantChunks := []string{"abcdefghij", "hijklmnopq", "opqrstuvwx", "vwxy"}
	if len(chunks) != len(wantChunks) {
		t.Fatalf("len(chunks) = %d, want %d: %v", len(chunks), len(wantChunks), chunks)
	}
	for i := range wantChunks {
		if chunks[i] != wantChunks[i] {
			t.Fatalf("chunks[%d] = %q, want %q", i, chunks[i], wantChunks[i])
		}
	}
	for i := 1; i < len(chunks); i++ {
		overlap := chunks[i-1][len(chunks[i-1])-3:]
		if overlap != chunks[i][:len(overlap)] {
			t.Fatalf("相邻切片应共享重叠区 %q", overlap)
		}
	}
	chunks = chunkText("aaaaaaa\nbbbbbbbbbbb", 12, 0)
	if len(chunks) != 2 || chunks[0] != "aaaaaaa" || chunks[1] != "bbbbbbbbbbb" {
		t.Fatalf("窗口内边界应优先于硬切: %v", chunks)
	}
	chunks = chunkText(strings.Repeat("x", 20), 10, 0)
	if len(chunks) != 2 || chunks[0] != strings.Repeat("x", 10) || chunks[1] != strings.Repeat("x", 10) {
		t.Fatalf("精确窗口应切成两段: %v", chunks)
	}
	chunks = chunkText("abcdefghij", 4, 99)
	if len(chunks) < 3 {
		t.Fatalf("overlap ≥ chunkSize 应收敛为渐进切分: %v", chunks)
	}
	if chunks := chunkText("abcd", 2, -5); len(chunks) != 2 {
		t.Fatalf("负 overlap 应收敛: %v", chunks)
	}

	// ---- 环节二：查询词抽取 ----
	terms := extractQueryTerms("主角的名字与命运")
	joinedTerms := strings.Join(terms, ",")
	if !strings.Contains(joinedTerms, "主角") || !strings.Contains(joinedTerms, "名字") {
		t.Fatalf("CJK 应切为 2-gram: %v", terms)
	}
	latinTerms := extractQueryTerms("Hello, world! hello")
	if len(latinTerms) != 2 || latinTerms[0] != "hello" || latinTerms[1] != "world" {
		t.Fatalf("拉丁词应去重去标点: %v", latinTerms)
	}

	// ---- 环节三：检索模式归一（未知模式按 keyword） ----
	if got := normalizeSearchMode("semantic"); got != domain.SearchModeKeyword {
		t.Fatalf("未知模式 = %q, want keyword", got)
	}
	if got := normalizeSearchMode(""); got != domain.SearchModeKeyword {
		t.Fatalf("空模式 = %q, want keyword", got)
	}
	if got := normalizeSearchMode("hybrid"); got != domain.SearchModeHybrid {
		t.Fatalf("hybrid 模式 = %q, want hybrid", got)
	}

	// ---- 环节四：RRF 双通道融合（同切片跨通道得分累加并标 hybrid） ----
	results := fuseSearchResults(
		[]domain.KnowledgeChunkVO{searchHit("T1"), searchHit("T2")},
		[]domain.KnowledgeChunkVO{searchHit("T2"), searchHit("T3")},
		2,
	)
	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(results))
	}
	if results[0].ChunkULID != "T2" {
		t.Fatalf("跨通道命中的切片应排首位: %q", results[0].ChunkULID)
	}
	wantScore := 1.0/62.0 + 1.0/61.0
	if math.Abs(results[0].ScoreValue-wantScore) > 1e-12 {
		t.Fatalf("T2 得分 = %v, want %v", results[0].ScoreValue, wantScore)
	}
	if results[0].SourceMode != domain.SearchModeHybrid {
		t.Fatalf("T2 source_mode = %q, want hybrid", results[0].SourceMode)
	}
	if results[1].ChunkULID != "T1" || results[1].SourceMode != "term" {
		t.Fatalf("results[1] = %+v, want T1/term", results[1])
	}

	// ---- 环节五：RRF 单通道排序与 TopK 截断 ----
	results = fuseSearchResults(
		[]domain.KnowledgeChunkVO{searchHit("T1"), searchHit("T2"), searchHit("T3")},
		nil, 2,
	)
	if len(results) != 2 {
		t.Fatalf("TopK 截断后 len = %d, want 2", len(results))
	}
	for i := range results {
		if results[i].SourceMode != "term" {
			t.Fatalf("results[%d] source_mode = %q, want term", i, results[i].SourceMode)
		}
	}
	if results[0].ChunkULID != "T1" || results[1].ChunkULID != "T2" {
		t.Fatalf("单通道应按 1/(60+rank) 降序: %q,%q", results[0].ChunkULID, results[1].ChunkULID)
	}
	if math.Abs(results[0].ScoreValue-1.0/61.0) > 1e-12 || math.Abs(results[1].ScoreValue-1.0/62.0) > 1e-12 {
		t.Fatalf("单通道得分不符: %v,%v", results[0].ScoreValue, results[1].ScoreValue)
	}

	// ---- 环节六：重排结果解析（严格 JSON + 候选校验 + fail-open） ----
	allowed := []string{"C1", "C2", "C3"}
	ids, err := ai.ParseRerankIDs(`{"ids":["C2","C1"]}`, allowed)
	if err != nil || len(ids) != 3 || ids[0] != "C2" || ids[1] != "C1" || ids[2] != "C3" {
		t.Fatalf("重排解析应重排并补齐尾部: %v, err = %v", ids, err)
	}
	for _, bad := range []string{`{"ids":["C1","C1"]}`, `{"ids":["C9"]}`, "not json", "```json {\"ids\":[]} ```"} {
		if _, err := ai.ParseRerankIDs(bad, allowed); err == nil {
			t.Fatalf("重排解析 %q 应失败并降级", bad)
		}
	}
}
