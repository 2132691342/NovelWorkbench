package service

import (
	"math"
	"strings"
	"testing"

	"github.com/2132691342/NovelWorkbench/backend/internal/ai"
	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
)

// TestChunkTextShort 短文本：整段单切片（含去首尾空白与空文本返回空）。
func TestChunkTextShort(t *testing.T) {
	chunks := chunkText("  短文本  ", 100, 10)
	if len(chunks) != 1 || chunks[0] != "短文本" {
		t.Fatalf("chunks = %v, want [短文本]", chunks)
	}
	if chunks := chunkText("   \n  ", 100, 10); len(chunks) != 0 {
		t.Fatalf("blank text chunks = %v, want empty", chunks)
	}
}

// TestChunkTextWindowAndOverlap 滑窗与重叠：无边界文本按 chunkSize-overlap 步进，相邻切片共享重叠区。
func TestChunkTextWindowAndOverlap(t *testing.T) {
	chunks := chunkText("abcdefghijklmnopqrstuvwxy", 10, 3)
	want := []string{"abcdefghij", "hijklmnopq", "opqrstuvwx", "vwxy"}
	if len(chunks) != len(want) {
		t.Fatalf("len(chunks) = %d, want %d: %v", len(chunks), len(want), chunks)
	}
	for i := range want {
		if chunks[i] != want[i] {
			t.Fatalf("chunks[%d] = %q, want %q", i, chunks[i], want[i])
		}
	}
	for i := 1; i < len(chunks); i++ {
		if overlap := chunks[i-1][len(chunks[i-1])-3:]; overlap != chunks[i][:len(overlap)] {
			t.Fatalf("overlap between %q and %q = %q", chunks[i-1], chunks[i], overlap)
		}
	}
}

// TestChunkTextBoundary 边界优先：窗口内句末/段落边界早于硬切；精确窗口文本不越界。
func TestChunkTextBoundary(t *testing.T) {
	chunks := chunkText("aaaaaaa\nbbbbbbbbbbb", 12, 0)
	want := []string{"aaaaaaa", "bbbbbbbbbbb"}
	if len(chunks) != len(want) {
		t.Fatalf("len(chunks) = %d, want %d: %v", len(chunks), len(want), chunks)
	}
	for i := range want {
		if chunks[i] != want[i] {
			t.Fatalf("chunks[%d] = %q, want %q", i, chunks[i], want[i])
		}
	}
	exact := strings.Repeat("x", 20)
	chunks = chunkText(exact, 10, 0)
	if len(chunks) != 2 || chunks[0] != strings.Repeat("x", 10) || chunks[1] != strings.Repeat("x", 10) {
		t.Fatalf("exact window chunks = %v, want two 10-char chunks", chunks)
	}
}

// TestChunkTextOverlapClamp 重叠非法值收敛：overlap ≥ chunkSize 或负值不产生死循环或空转。
func TestChunkTextOverlapClamp(t *testing.T) {
	chunks := chunkText("abcdefghij", 4, 99)
	if len(chunks) < 3 {
		t.Fatalf("chunks = %v, want progressive split", chunks)
	}
	joined := strings.Join(chunks, "")
	if !strings.HasPrefix(joined, "abcd") {
		t.Fatalf("joined = %q, want prefix abcd", joined)
	}
	if chunks := chunkText("abcd", 2, -5); len(chunks) != 2 {
		t.Fatalf("negative overlap chunks = %v, want 2", chunks)
	}
}

// searchHit 构造检索命中。
func searchHit(chunkULID string) domain.KnowledgeChunkVO {
	return domain.KnowledgeChunkVO{ChunkULID: chunkULID, ChunkText: chunkULID}
}

// TestFuseSearchResultsBothChannels RRF 双通道：同切片得分累加并标 hybrid，排序按得分降序取 topK。
func TestFuseSearchResultsBothChannels(t *testing.T) {
	termHits := []domain.KnowledgeChunkVO{searchHit("T1"), searchHit("T2")}
	phraseHits := []domain.KnowledgeChunkVO{searchHit("T2"), searchHit("T3")}
	results := fuseSearchResults(termHits, phraseHits, 2)
	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(results))
	}
	if results[0].ChunkULID != "T2" {
		t.Fatalf("results[0] = %q, want T2", results[0].ChunkULID)
	}
	wantT2 := 1.0/62.0 + 1.0/61.0
	if math.Abs(results[0].ScoreValue-wantT2) > 1e-12 {
		t.Fatalf("T2 score = %v, want %v", results[0].ScoreValue, wantT2)
	}
	if results[0].SourceMode != domain.SearchModeHybrid {
		t.Fatalf("T2 source_mode = %q, want hybrid", results[0].SourceMode)
	}
	if results[1].ChunkULID != "T1" || results[1].SourceMode != "term" {
		t.Fatalf("results[1] = %+v, want T1/term", results[1])
	}
}

// TestFuseSearchResultsSingleChannel RRF 单通道：得分按 1/(60+rank) 递减，source_mode 单通道，topK 截断。
func TestFuseSearchResultsSingleChannel(t *testing.T) {
	termHits := []domain.KnowledgeChunkVO{searchHit("T1"), searchHit("T2"), searchHit("T3")}
	results := fuseSearchResults(termHits, nil, 2)
	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(results))
	}
	for i := range results {
		if results[i].SourceMode != "term" {
			t.Fatalf("results[%d] source_mode = %q, want term", i, results[i].SourceMode)
		}
	}
	if results[0].ChunkULID != "T1" || results[1].ChunkULID != "T2" {
		t.Fatalf("order = %q,%q, want T1,T2", results[0].ChunkULID, results[1].ChunkULID)
	}
	if math.Abs(results[0].ScoreValue-1.0/61.0) > 1e-12 || math.Abs(results[1].ScoreValue-1.0/62.0) > 1e-12 {
		t.Fatalf("scores = %v,%v, want %v,%v", results[0].ScoreValue, results[1].ScoreValue, 1.0/61.0, 1.0/62.0)
	}
}

// TestExtractQueryTerms CJK 2-gram 与拉丁词抽取：去停用词/标点并去重。
func TestExtractQueryTerms(t *testing.T) {
	terms := extractQueryTerms("主角的名字与命运")
	joined := strings.Join(terms, ",")
	if !strings.Contains(joined, "主角") || !strings.Contains(joined, "名字") {
		t.Fatalf("terms = %v, want CJK bigrams", terms)
	}
	latin := extractQueryTerms("Hello, world! hello")
	if len(latin) != 2 || latin[0] != "hello" || latin[1] != "world" {
		t.Fatalf("latin terms = %v, want [hello world]", latin)
	}
}

// TestNormalizeSearchMode 未知模式按 keyword 处理，仅 hybrid 保持双通道。
func TestNormalizeSearchMode(t *testing.T) {
	if got := normalizeSearchMode("semantic"); got != domain.SearchModeKeyword {
		t.Fatalf("unknown mode = %q, want keyword", got)
	}
	if got := normalizeSearchMode(""); got != domain.SearchModeKeyword {
		t.Fatalf("empty mode = %q, want keyword", got)
	}
	if got := normalizeSearchMode("hybrid"); got != domain.SearchModeHybrid {
		t.Fatalf("hybrid mode = %q, want hybrid", got)
	}
}

// TestParseRerankIDs 严格 JSON 重排解析：非法 ID、重复 ID、缺 ID 均 fail-open。
func TestParseRerankIDs(t *testing.T) {
	allowed := []string{"C1", "C2", "C3"}
	ids, err := ai.ParseRerankIDs(`{"ids":["C2","C1"]}`, allowed)
	if err != nil || len(ids) != 3 || ids[0] != "C2" || ids[1] != "C1" || ids[2] != "C3" {
		t.Fatalf("ids = %v, err = %v, want reordered + tail fill", ids, err)
	}
	for _, bad := range []string{`{"ids":["C1","C1"]}`, `{"ids":["C9"]}`, "not json", "```json {\"ids\":[]} ```"} {
		if _, err := ai.ParseRerankIDs(bad, allowed); err == nil {
			t.Fatalf("parse %q should fail", bad)
		}
	}
}

// TestRerankCandidateIDs 重排候选输入仅为 ID + 短文本（无向量字段）。
func TestRerankCandidateIDs(t *testing.T) {
	candidates := []ai.RerankCandidate{{ID: "C1", Text: "短文本"}}
	if candidates[0].ID != "C1" || candidates[0].Text != "短文本" {
		t.Fatalf("candidate = %+v", candidates[0])
	}
}
