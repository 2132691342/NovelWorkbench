package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/2132691342/NovelWorkbench/backend/internal/infra"
)

// TestEnsureManticoreSchemaMigratesLegacyTable 验证旧全文表按 CJK ngram 结构重建且仅迁移一次。
func TestEnsureManticoreSchemaMigratesLegacyTable(t *testing.T) {
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
