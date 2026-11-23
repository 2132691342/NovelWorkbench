// 知识库链路的场景回归：文档切片 → 查询词抽取 → 双通道融合 → 重排解析；索引结构迁移单列一条。
// 切片与融合是同一条检索链路的上下游，合并为一条用例；
// 索引迁移发生在每次启动，会重建已有索引，必须单独守住。

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

// TestKnowledgeRetrievalChain 覆盖一次检索从「原文」到「排序结果」的完整链路：
// 切片 → 查询词抽取 → RRF 双通道融合 → 单通道排序 → 重排解析与 fail-open。
func TestKnowledgeRetrievalChain(t *testing.T) {
	// 环节1：切片。重叠窗口保留上下文，非法 overlap 收敛防死循环，空白不产出切片。
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
			t.Fatalf("切片[%d] = %q, want %q", i, chunks[i], want[i])
		}
	}
	if chunks := chunkText(strings.Repeat("x", 10), 4, 99); len(chunks) < 3 {
		t.Fatalf("overlap ≥ chunkSize 应收敛: %v", chunks)
	}

	// 环节2：查询词抽取。CJK 切 2-gram；拉丁词去重去标点。
	terms := strings.Join(extractQueryTerms("主角的名字与命运"), ",")
	if !strings.Contains(terms, "主角") || !strings.Contains(terms, "名字") {
		t.Fatalf("CJK 应切为 2-gram: %v", terms)
	}
	latin := extractQueryTerms("Hello, world! hello")
	if len(latin) != 2 || latin[0] != "hello" || latin[1] != "world" {
		t.Fatalf("拉丁词应去重去标点: %v", latin)
	}

	// 环节3：双通道 RRF 融合。跨通道命中累加得分并标 hybrid，TopK 截断。
	results := fuseSearchResults(
		[]domain.KnowledgeChunkVO{searchHit("T1"), searchHit("T2")},
		[]domain.KnowledgeChunkVO{searchHit("T2"), searchHit("T3")},
		2,
	)
	wantScore := 1.0/62.0 + 1.0/61.0
	if len(results) != 2 || results[0].ChunkULID != "T2" ||
		math.Abs(results[0].ScoreValue-wantScore) > 1e-12 || results[0].SourceMode != domain.SearchModeHybrid {
		t.Fatalf("跨通道命中应排首位且标 hybrid: %+v", results[0])
	}
	if results[1].ChunkULID != "T1" || results[1].SourceMode != "term" {
		t.Fatalf("results[1] = %+v, want T1/term", results[1])
	}

	// 环节4：单通道按 1/(60+rank) 降序并截断 TopK。
	results = fuseSearchResults(
		[]domain.KnowledgeChunkVO{searchHit("T1"), searchHit("T2"), searchHit("T3")},
		nil, 2,
	)
	if len(results) != 2 || results[0].ChunkULID != "T1" || results[1].ChunkULID != "T2" {
		t.Fatalf("单通道应按 1/(60+rank) 降序: %+v", results)
	}

	// 环节5：重排解析。合法结果重排并补齐尾部；重复、越界、非 JSON、代码围栏一律失败降级。
	allowed := []string{"C1", "C2", "C3"}
	ids, err := ai.ParseRerankIDs(`{"ids":["C2","C1"]}`, allowed)
	if err != nil || len(ids) != 3 || ids[0] != "C2" || ids[1] != "C1" || ids[2] != "C3" {
		t.Fatalf("重排应重排并补齐尾部: %v, err=%v", ids, err)
	}
	for _, bad := range []string{`{"ids":["C1","C1"]}`, `{"ids":["C9"]}`, "not json", "```json {\"ids\":[]} ```"} {
		if _, err := ai.ParseRerankIDs(bad, allowed); err == nil {
			t.Fatalf("非法重排结果应报错: %q", bad)
		}
	}
}

// TestManticoreSchemaMigrationChain 覆盖中文 ngram 索引的建表结构迁移：识别旧表 → 重建 → 回灌只执行一次；
// 新结构启动不重建。
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
