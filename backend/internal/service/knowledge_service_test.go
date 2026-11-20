// 本文件覆盖知识库两条长链路：无向量检索与 Manticore 中文 ngram 表结构迁移。

package service

import (
	"context"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/2132691342/NovelWorkbench/backend/internal/ai"
	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/infra"
)

// searchHit 构造检索命中。
func searchHit(chunkULID string) domain.KnowledgeChunkVO {
	return domain.KnowledgeChunkVO{ChunkULID: chunkULID, ChunkText: chunkULID}
}

// TestKnowledgeRetrievalChain 覆盖检索链路：切片 → 查询词抽取 → RRF 双通道融合 → 单通道排序与 TopK → 重排解析。
func TestKnowledgeRetrievalChain(t *testing.T) {
	// ---- 环节一：切片（重叠共享、窗口内边界优先于硬切、非法参数收敛） ----
	if chunks := chunkText("  短文本  ", 100, 10); len(chunks) != 1 || chunks[0] != "短文本" {
		t.Fatalf("短文本应产出单切片: %v", chunks)
	}
	if chunks := chunkText("   \n  ", 100, 10); len(chunks) != 0 {
		t.Fatalf("空白文本不应产出切片: %v", chunks)
	}
	chunks := chunkText("abcdefghijklmnopqrstuvwxy", 10, 3)
	wantChunks := []string{"abcdefghij", "hijklmnopq", "opqrstuvwx", "vwxy"}
	if len(chunks) != len(wantChunks) {
		t.Fatalf("len(chunks) = %d, want %d: %v", len(chunks), len(wantChunks), chunks)
	}
	for i := range wantChunks {
		if chunks[i] != wantChunks[i] {
			t.Fatalf("chunks[%d] = %q, want %q", i, chunks[i], wantChunks[i])
		}
	}
	if chunks := chunkText("aaaaaaa\nbbbbbbbbbbb", 12, 0); len(chunks) != 2 || chunks[0] != "aaaaaaa" {
		t.Fatalf("窗口内边界应优先于硬切: %v", chunks)
	}
	// overlap ≥ chunkSize 与负 overlap 必须收敛，否则切片会死循环或切空。
	if chunks := chunkText(strings.Repeat("x", 10), 4, 99); len(chunks) < 3 {
		t.Fatalf("overlap ≥ chunkSize 应收敛为渐进切分: %v", chunks)
	}
	if chunks := chunkText("abcd", 2, -5); len(chunks) != 2 {
		t.Fatalf("负 overlap 应收敛: %v", chunks)
	}

	// ---- 环节二：查询词抽取（CJK 2-gram，拉丁词去重去标点） ----
	terms := strings.Join(extractQueryTerms("主角的名字与命运"), ",")
	if !strings.Contains(terms, "主角") || !strings.Contains(terms, "名字") {
		t.Fatalf("CJK 应切为 2-gram: %v", terms)
	}
	latinTerms := extractQueryTerms("Hello, world! hello")
	if len(latinTerms) != 2 || latinTerms[0] != "hello" || latinTerms[1] != "world" {
		t.Fatalf("拉丁词应去重去标点: %v", latinTerms)
	}

	// ---- 环节三：RRF 双通道融合（同切片跨通道得分累加并标 hybrid） ----
	results := fuseSearchResults(
		[]domain.KnowledgeChunkVO{searchHit("T1"), searchHit("T2")},
		[]domain.KnowledgeChunkVO{searchHit("T2"), searchHit("T3")},
		2,
	)
	if len(results) != 2 || results[0].ChunkULID != "T2" {
		t.Fatalf("跨通道命中的切片应排首位且截断到 TopK: %+v", results)
	}
	wantScore := 1.0/62.0 + 1.0/61.0
	if math.Abs(results[0].ScoreValue-wantScore) > 1e-12 || results[0].SourceMode != domain.SearchModeHybrid {
		t.Fatalf("T2 得分/来源错误: %v %q", results[0].ScoreValue, results[0].SourceMode)
	}
	if results[1].ChunkULID != "T1" || results[1].SourceMode != "term" {
		t.Fatalf("results[1] = %+v, want T1/term", results[1])
	}

	// ---- 环节四：单通道排序按 1/(60+rank) 降序 ----
	results = fuseSearchResults(
		[]domain.KnowledgeChunkVO{searchHit("T1"), searchHit("T2"), searchHit("T3")},
		nil, 2,
	)
	if len(results) != 2 || results[0].ChunkULID != "T1" || results[1].ChunkULID != "T2" {
		t.Fatalf("单通道应按 1/(60+rank) 降序: %+v", results)
	}
	if math.Abs(results[0].ScoreValue-1.0/61.0) > 1e-12 || math.Abs(results[1].ScoreValue-1.0/62.0) > 1e-12 {
		t.Fatalf("单通道得分不符: %v,%v", results[0].ScoreValue, results[1].ScoreValue)
	}

	// ---- 环节五：重排结果解析（严格 JSON + 候选校验 + 尾部补齐，非法即失败降级） ----
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

// TestManticoreSchemaMigrationChain 覆盖 Manticore 中文 ngram 表结构迁移：
// 旧全文表按新结构重建且仅迁移一次，新结构启动不重建。
func TestManticoreSchemaMigrationChain(t *testing.T) {
	var queries []string
	legacy := true
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.FormValue("query")
		queries = append(queries, query)
		switch {
		case strings.HasPrefix(query, "SHOW CREATE TABLE"):
			if legacy {
				_, _ = w.Write([]byte(`[{"Create Table":"min_infix_len='3' morphology='none'"}]`))
				return
			}
			_, _ = w.Write([]byte(`[{"Create Table":"ngram_len='1' ngram_chars='cjk'"}]`))
		case strings.HasPrefix(query, "DROP TABLE"):
			legacy = false
		}
		_, _ = w.Write([]byte(`[{"total":0}]`))
	}))
	defer server.Close()
	client := infra.NewManticore(&infra.ManticoreConfig{BaseURL: server.URL})

	rebuilt, err := ensureManticoreSchema(context.Background(), client, "nw_chunks")
	if err != nil {
		t.Fatalf("ensure schema: %v", err)
	}
	if !rebuilt {
		t.Fatal("legacy table should be rebuilt")
	}
	if len(queries) != 4 || !strings.HasPrefix(queries[1], "SHOW CREATE TABLE") ||
		!strings.HasPrefix(queries[2], "DROP TABLE") || !strings.Contains(queries[3], "ngram_len='1'") {
		t.Fatalf("unexpected migration sequence: %v", queries)
	}

	rebuilt, err = ensureManticoreSchema(context.Background(), client, "nw_chunks")
	if err != nil {
		t.Fatalf("ensure schema again: %v", err)
	}
	if rebuilt {
		t.Fatal("migrated table should not rebuild again")
	}
}
