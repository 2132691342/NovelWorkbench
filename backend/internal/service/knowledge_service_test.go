package service

import (
	"math"
	"strings"
	"testing"

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
	vectorHits := []domain.KnowledgeChunkVO{searchHit("V1"), searchHit("V2")}
	keywordHits := []domain.KnowledgeChunkVO{searchHit("V2"), searchHit("V3")}
	results := fuseSearchResults(vectorHits, keywordHits, 2)
	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(results))
	}
	if results[0].ChunkULID != "V2" {
		t.Fatalf("results[0] = %q, want V2", results[0].ChunkULID)
	}
	wantV2 := 1.0/62.0 + 1.0/61.0
	if math.Abs(results[0].ScoreValue-wantV2) > 1e-12 {
		t.Fatalf("V2 score = %v, want %v", results[0].ScoreValue, wantV2)
	}
	if results[0].SourceMode != "hybrid" {
		t.Fatalf("V2 source_mode = %q, want hybrid", results[0].SourceMode)
	}
	if results[1].ChunkULID != "V1" {
		t.Fatalf("results[1] = %q, want V1", results[1].ChunkULID)
	}
	if math.Abs(results[1].ScoreValue-1.0/61.0) > 1e-12 {
		t.Fatalf("V1 score = %v, want %v", results[1].ScoreValue, 1.0/61.0)
	}
	if results[1].SourceMode != "vector" {
		t.Fatalf("V1 source_mode = %q, want vector", results[1].SourceMode)
	}
}

// TestFuseSearchResultsSingleChannel RRF 单通道：得分按 1/(60+rank) 递减，source_mode 单通道，topK 截断。
func TestFuseSearchResultsSingleChannel(t *testing.T) {
	keywordHits := []domain.KnowledgeChunkVO{searchHit("K1"), searchHit("K2"), searchHit("K3")}
	results := fuseSearchResults(nil, keywordHits, 2)
	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(results))
	}
	for i := range results {
		if results[i].SourceMode != "keyword" {
			t.Fatalf("results[%d] source_mode = %q, want keyword", i, results[i].SourceMode)
		}
	}
	if results[0].ChunkULID != "K1" || results[1].ChunkULID != "K2" {
		t.Fatalf("order = %q,%q, want K1,K2", results[0].ChunkULID, results[1].ChunkULID)
	}
	if math.Abs(results[0].ScoreValue-1.0/61.0) > 1e-12 || math.Abs(results[1].ScoreValue-1.0/62.0) > 1e-12 {
		t.Fatalf("scores = %v,%v, want %v,%v", results[0].ScoreValue, results[1].ScoreValue, 1.0/61.0, 1.0/62.0)
	}
	vectorOnly := fuseSearchResults([]domain.KnowledgeChunkVO{searchHit("V1")}, nil, 0)
	if len(vectorOnly) != 1 || vectorOnly[0].SourceMode != "vector" {
		t.Fatalf("vectorOnly = %v, want single vector hit", vectorOnly)
	}
}
