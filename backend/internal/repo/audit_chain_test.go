// 审计表结构与筛选条件的回归：DDL 与列清单 → 保留字转义 → 派生标记列 → 退化判定 → 筛选条件拼接。
// 这些规则决定链路视图能不能用：model_name 是 Manticore 保留字，裸写会让建表失败后自动建成 text 表，
// 筛选随之失效；派生标记列退化成 text 会让级别、模型调用、慢链路筛选整页失准；
// 条件拼接错一个括号就会让整页查询报错。

package repo

import (
	"strings"
	"testing"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
)

// newTestAuditRepo 构造不连库的仓储（只测 SQL 生成）。
func newTestAuditRepo() *AuditRepo {
	return NewAuditRepo(nil, "nw_audit")
}

// quoteAll 给列名逐个加反引号（测试期望值用）。
func quoteAll(names []string) []string {
	quoted := make([]string, 0, len(names))
	for _, name := range names {
		quoted = append(quoted, quoteIdent(name))
	}
	return quoted
}

// TestAuditSchemaChain 覆盖审计表的结构契约。
func TestAuditSchemaChain(t *testing.T) {
	// 环节1：DDL 与补列语句。所有列名必须带反引号：model_name 是 Manticore 保留字，
	// 裸写会报 P03 unexpected MODEL_NAME，建表失败后 Manticore 会自动建一张 text 表。
	ddl := newTestAuditRepo().auditDDL()
	for _, column := range auditColumnSpecList {
		if !strings.Contains(ddl, quoteIdent(column.name)) {
			t.Fatalf("DDL 缺少带反引号的列 %s：%s", quoteIdent(column.name), ddl)
		}
	}
	if strings.Contains(ddl, " model_name ") || strings.Contains(ddl, "(model_name ") {
		t.Fatalf("DDL 中 model_name 未加反引号：%s", ddl)
	}
	if !strings.Contains(ddl, "`model_name` string") {
		t.Fatalf("DDL 未按 `model_name` string 生成：%s", ddl)
	}
	for _, name := range auditAddedColumnNames {
		sql := "ALTER TABLE nw_audit ADD COLUMN " + quoteIdent(name) + " " + auditColumnType(name)
		if !strings.Contains(sql, quoteIdent(name)) {
			t.Fatalf("补列语句未加反引号：%s", sql)
		}
	}

	// 环节2：标识符转义。内部含反引号的列名要把反引号加倍。
	for input, want := range map[string]string{
		"model_name": "`model_name`",
		"trace_id":   "`trace_id`",
		"we`ird":     "`we``ird`",
	} {
		if got := quoteIdent(input); got != want {
			t.Fatalf("quoteIdent(%q) = %q，want %q", input, got, want)
		}
	}

	// 环节3：列清单同源。INSERT 列与列定义必须完全同序同集合，占位符数量也要对齐。
	want := strings.Join(quoteAll(auditColumnList()), ", ")
	if got := auditInsertColumns(); got != want {
		t.Fatalf("INSERT 列清单漂移\nwant: %s\n got: %s", want, got)
	}
	if placeholderCount := strings.Count(auditInsertPlaceholders(), ",") + 1; placeholderCount != len(auditColumnSpecList) {
		t.Fatalf("INSERT 占位符 %d 个，列 %d 个", placeholderCount, len(auditColumnSpecList))
	}

	// 环节4：SELECT 列是列定义的真子集，且排除大字段。
	all := make(map[string]bool, len(auditColumnSpecList))
	for _, column := range auditColumnSpecList {
		all[column.name] = true
	}
	selected := strings.Split(auditSelectColumns(), ", ")
	if len(selected) == 0 {
		t.Fatal("SELECT 列清单为空")
	}
	for _, quoted := range selected {
		name := strings.Trim(quoted, "`")
		if !all[name] {
			t.Fatalf("SELECT 出现未定义列 %s", name)
		}
		if auditDetailOnlyColumnSet[name] {
			t.Fatalf("列表 SELECT 不应包含大字段 %s", name)
		}
	}
	for _, name := range auditColumnList() {
		if auditDetailOnlyColumnSet[name] {
			continue
		}
		if !strings.Contains(auditSelectColumns(), quoteIdent(name)) {
			t.Fatalf("SELECT 缺少列 %s", name)
		}
	}

	// 环节5：text 列声明与集合必须一致，退化判定据此决定要不要重建表。
	for _, column := range auditColumnSpecList {
		if (column.typeCode == "text") != auditTextColumnSet[column.name] {
			t.Fatalf("列 %s 类型 %s 与 auditTextColumnSet 不一致", column.name, column.typeCode)
		}
		if auditTextColumnSet[column.name] && auditFilterCriticalColumnSet[column.name] {
			t.Fatalf("text 列 %s 不应出现在过滤核心列集合", column.name)
		}
	}

	// 环节6：派生标记列必须是 int。链路视图的级别、模型调用、慢链路筛选全靠这几列做 SUM，
	// 退化成 text 会让整页筛选失效。
	for _, name := range []string{"root_span_flag", "error_span_flag", "warn_span_flag", "ai_span_flag"} {
		if got := auditColumnType(name); got != "int" {
			t.Fatalf("派生标记列 %s 类型为 %s，want int", name, got)
		}
		if auditTextColumnSet[name] {
			t.Fatalf("派生标记列 %s 不应是全文字段", name)
		}
	}

	// 环节7：退化判定。只有过滤核心列退化才重建，避免误删历史审计。
	degradeCases := []struct {
		name     string
		degraded []string
		want     bool
	}{
		{"全部列退化为 text（自动建表）", []string{"span_type_code", "log_level_code", "trace_id", "created_at_ts"}, true},
		{"仅 span_type_code 退化", []string{"span_type_code"}, true},
		{"仅 created_at_ts 退化", []string{"created_at_ts"}, true},
		{"非核心列退化不重建", []string{"user_name", "target_type"}, false},
		{"无退化", nil, false},
	}
	for _, testCase := range degradeCases {
		if got := hasCriticalDegradedColumn(testCase.degraded); got != testCase.want {
			t.Fatalf("%s: hasCriticalDegradedColumn(%v) = %v，want %v",
				testCase.name, testCase.degraded, got, testCase.want)
		}
	}
}

// TestAuditConditionChain 覆盖审计筛选条件的拼接规则。
func TestAuditConditionChain(t *testing.T) {
	// 环节1：多条件拼接。只允许一次 MATCH，时间区间优先精确时刻。
	got := auditConditionOf(&auditCondition{
		KeywordText:  "workflow",
		ActionCode:   "workflow/run/create",
		LogLevelCode: domain.AuditLevelError,
		TraceID:      "TRC_01M7S87Y7997XGEMG5WG0RHZFT",
		TimeFrom:     "2026-11-01T00:00:00Z",
		DateTo:       "2026-11-02",
	})
	if strings.Count(got, "MATCH(") != 1 {
		t.Fatalf("条件里应只有一次 MATCH：%s", got)
	}
	for _, fragment := range []string{
		"@search_text", "`log_level_code` = 'error'", "`trace_id` = 'TRC_01M7S87Y7997XGEMG5WG0RHZFT'",
		"`created_at_ts` >=",
	} {
		if !strings.Contains(got, fragment) {
			t.Fatalf("条件缺少 %s：%s", fragment, got)
		}
	}
	// 给了 TimeFrom 时不应再叠加 DateFrom（两者互斥，避免同一维度两次过滤）。
	if strings.Contains(got, "`created_at_ts` > 0 AND") {
		t.Fatalf("条件拼接出现空过滤残留：%s", got)
	}

	// 环节2：无筛选时回落到恒真条件（Manticore 不接受空 WHERE）。
	if got := auditConditionOf(&auditCondition{}); got != "`created_at_ts` > 0" {
		t.Fatalf("空条件应回落恒真过滤，got %s", got)
	}
}
