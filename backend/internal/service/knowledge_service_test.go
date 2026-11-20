// 本文件覆盖知识库链路的场景回归：文本切片、查询词抽取、RRF 融合排序、重排解析与索引结构迁移。

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

// TestChunkText 覆盖切片：空白处理、重叠窗口、窗口内边界优先、overlap 非法收敛防死循环。
func TestChunkText(t *testing.T) {
	if chunks := chunkText("  短文本  ", 100, 10); len(chunks) != 1 || chunks[0] != "短文本" {
		t.Fatalf("短文本应产出单切片: %v", chunks)
	}
	if chunks := chunkText("   \n  ", 100, 10); len(chunks) != 0 {
		t.Fatalf("空白文本不应产出切片: %v", chunks)
	}
	chunks := chunkText("abcdefghijklmnopqrstuvwxy", 10, 3)
	want := []string{"abcdefghij", "hijklmnopq", "opqrstuvwx", "vwxy"}
	if len(chunks) != len(want) {
		t.Fatalf("len = %d, want %d: %v", len(chunks), len(want), chunks)
	}
	for i := range want {
		if chunks[i] != want[i] {
			t.Fatalf("chunks[%d] = %q, want %q", i, chunks[i], want[i])
		}
	}
	if chunks := chunkText("aaaaaaa\nbbbbbbbbbbb", 12, 0); len(chunks) != 2 || chunks[0] != "aaaaaaa" {
		t.Fatalf("窗口内边界应优先于硬切: %v", chunks)
	}
	if chunks := chunkText(strings.Repeat("x", 10), 4, 99); len(chunks) < 3 {
		t.Fatalf("overlap ≥ chunkSize 应收敛: %v", chunks)
	}
	if chunks := chunkText("abcd", 2, -5); len(chunks) != 2 {
		t.Fatalf("负 overlap 应收敛: %v", chunks)
	}
}

// TestExtractQueryTerms 覆盖查询词抽取：CJK 2-gram、拉丁词去重去标点。
func TestExtractQueryTerms(t *testing.T) {
	terms := strings.Join(extractQueryTerms("主角的名字与命运"), ",")
	if !strings.Contains(terms, "主角") || !strings.Contains(terms, "名字") {
		t.Fatalf("CJK 应切为 2-gram: %v", terms)
	}
	latin := extractQueryTerms("Hello, world! hello")
	if len(latin) != 2 || latin[0] != "hello" || latin[1] != "world" {
		t.Fatalf("拉丁词应去重去标点: %v", latin)
	}
}

// TestFuseSearchResults 覆盖双通道 RRF 融合：跨通道命中累加得分并标 hybrid、TopK 截断、
// 单通道按 1/(60+rank) 降序。
func TestFuseSearchResults(t *testing.T) {
	results := fuseSearchResults(
		[]domain.KnowledgeChunkVO{searchHit("T1"), searchHit("T2")},
		[]domain.KnowledgeChunkVO{searchHit("T2"), searchHit("T3")},
		2,
	)
	if len(results) != 2 || results[0].ChunkULID != "T2" {
		t.Fatalf("跨通道命中应排首位: %+v", results)
	}
	wantScore := 1.0/62.0 + 1.0/61.0
	if math.Abs(results[0].ScoreValue-wantScore) > 1e-12 || results[0].SourceMode != domain.SearchModeHybrid {
		t.Fatalf("T2 得分/来源错误: %v %q", results[0].ScoreValue, results[0].SourceMode)
	}
	if results[1].ChunkULID != "T1" || results[1].SourceMode != "term" {
		t.Fatalf("results[1] = %+v, want T1/term", results[1])
	}

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
}

// TestParseRerankIDs 覆盖重排解析：合法结果重排并补齐尾部，重复/越界/非 JSON/代码围栏一律失败降级。
func TestParseRerankIDs(t *testing.T) {
	allowed := []string{"C1", "C2", "C3"}
	ids, err := ai.ParseRerankIDs(`{"ids":["C2","C1"]}`, allowed)
	if err != nil || len(ids) != 3 || ids[0] != "C2" || ids[1] != "C1" || ids[2] != "C3" {
		t.Fatalf("重排应重排并补齐尾部: %v, err=%v", ids, err)
	}
	for _, bad := range []string{`{"ids":["C1","C1"]}`, `{"ids":["C9"]}`, "not json", "```json {\"ids\":[]} ```"} {
		if _, err := ai.ParseRerankIDs(bad, allowed); err == nil {
			t.Fatalf("重排解析 %q 应失败降级", bad)
		}
	}
}

// TestManticoreSchemaMigration 覆盖中文 ngram 表迁移：旧表重建一次且语句序列正确，新表不重建。
func TestManticoreSchemaMigration(t *testing.T) {
	var queries []string
	legacy := true
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.FormValue("query")
		queries = append(queries, query)
		switch {
		case strings.HasPrefix(query, "SHOW CREATE TABLE"):
			if legacy {
				_, _ = w.Write([]byte(`[{"Create Table":"min_infix_len='3' morphology='none'"}]`))
			} else {
				_, _ = w.Write([]byte(`[{"Create Table":"ngram_len='1' ngram_chars='cjk'"}]`))
			}
		case strings.HasPrefix(query, "DROP TABLE"):
			legacy = false
			_, _ = w.Write([]byte(`[{"total":0}]`))
		default:
			_, _ = w.Write([]byte(`[{"total":0}]`))
		}
	}))
	defer server.Close()
	client := infra.NewManticore(&infra.ManticoreConfig{BaseURL: server.URL})

	rebuilt, err := ensureManticoreSchema(context.Background(), client, "nw_chunks")
	if err != nil || !rebuilt {
		t.Fatalf("legacy 表应重建: rebuilt=%v err=%v", rebuilt, err)
	}
	if len(queries) != 4 || !strings.HasPrefix(queries[1], "SHOW CREATE TABLE") ||
		!strings.HasPrefix(queries[2], "DROP TABLE") || !strings.Contains(queries[3], "ngram_len='1'") {
		t.Fatalf("迁移语句序列错误: %v", queries)
	}
	queries = nil
	rebuilt, err = ensureManticoreSchema(context.Background(), client, "nw_chunks")
	if err != nil || rebuilt {
		t.Fatalf("新结构不应重建: rebuilt=%v err=%v", rebuilt, err)
	}
}
