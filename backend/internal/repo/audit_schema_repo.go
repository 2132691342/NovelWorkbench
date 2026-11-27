// 本文件实现审计表的结构管理与仓储类型：列定义（唯一事实源）、建表与补列、
// 结构退化检测与自动重建。列名一律加反引号（model_name 是 Manticore 保留字）。
// PostgreSQL 不再保存审计行。

package repo

import (
	"context"
	"fmt"
	"strings"

	"github.com/2132691342/NovelWorkbench/backend/internal/infra"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
)

// AuditRepo 审计日志仓储（Manticore 全文表）。
type AuditRepo struct {
	manticore *infra.ManticoreClient // 全文检索客户端
	table     string                 // 审计表名
}

// NewAuditRepo 构造审计仓储；manticore 为 nil 时所有写入降级为丢弃（启动期已告警）。
func NewAuditRepo(manticore *infra.ManticoreClient, table string) *AuditRepo {
	if strings.TrimSpace(table) == "" {
		table = "nw_audit"
	}
	return &AuditRepo{manticore: manticore, table: table}
}

// TableName 返回审计表名（清理 CLI 打印目标与建表复用）。
func (r *AuditRepo) TableName() string { return r.table }

// auditColumnSpec 审计表的一列（列名 + Manticore 类型）。
// 列名一律经 quoteIdent 加反引号：model_name 是 Manticore 25.0 的保留字，
// 裸写会让 CREATE/ALTER/INSERT 全部语法报错（P03 unexpected MODEL_NAME）。
type auditColumnSpec struct {
	name     string // 列名（写入 SQL 时统一加反引号）
	typeCode string // Manticore 列类型：string/int 建为 attribute（可过滤），text 建为全文字段
}

// auditColumnSpecList 审计表的全部列，顺序即 INSERT 与 SELECT 的列序（唯一事实源）。
// 末尾四个 *_flag 是行级派生标记：Manticore 不支持 CASE，链路级别/模型调用/根节点
// 这类「按 trace 聚合」的筛选只能靠预先算好的 0/1 属性做 SUM，因此必须在写入时定好。
var auditColumnSpecList = []auditColumnSpec{
	{"audit_ulid", "string"},
	{"log_level_code", "string"},
	{"span_type_code", "string"},
	{"trace_id", "string"},
	{"parent_span_code", "string"},
	{"root_span_flag", "int"},
	{"error_span_flag", "int"},
	{"warn_span_flag", "int"},
	{"ai_span_flag", "int"},
	{"user_ulid", "string"},
	{"user_name", "string"},
	{"action_code", "string"},
	{"action_name", "string"},
	{"model_name", "string"},
	{"token_count", "int"},
	{"request_path", "string"},
	{"request_method", "string"},
	{"target_type", "string"},
	{"target_ulid", "string"},
	{"resp_code", "string"},
	{"resp_message", "text"},
	{"http_status", "int"},
	{"latency_ms", "int"},
	{"ip_addr", "string"},
	{"user_agent_text", "text"},
	{"request_body", "text"},
	{"resp_body", "text"},
	{"search_text", "text"},
	{"created_at_ts", "bigint"},
}

// auditTextColumnSet 建为 text 字段（全文检索/报文，不参与过滤）的列。
var auditTextColumnSet = map[string]bool{
	"resp_message":    true,
	"user_agent_text": true,
	"request_body":    true,
	"resp_body":       true,
	"search_text":     true,
}

// auditDetailOnlyColumnSet 只在详情/链路查询里读取的大字段（列表不取，避免撑爆响应）。
var auditDetailOnlyColumnSet = map[string]bool{
	"request_body": true,
	"resp_body":    true,
	"search_text":  true,
}

// auditAddedColumnNames 后续新增列（表已存在时逐列补齐，老部署无需重建索引）。
var auditAddedColumnNames = []string{
	"span_type_code", "parent_span_code", "action_name", "model_name", "token_count",
	"root_span_flag", "error_span_flag", "warn_span_flag", "ai_span_flag",
}

// quoteIdent 用反引号包裹 Manticore 标识符，规避保留字（如 model_name）导致的语法错误。
func quoteIdent(name string) string {
	return "`" + strings.ReplaceAll(name, "`", "``") + "`"
}

// auditColumnList 返回全部列名（INSERT 与测试比对用）。
func auditColumnList() []string {
	names := make([]string, 0, len(auditColumnSpecList))
	for _, column := range auditColumnSpecList {
		names = append(names, column.name)
	}
	return names
}

// auditInsertColumns 返回 INSERT 的列清单（与 auditColumnSpecList 同序，全部加反引号）。
func auditInsertColumns() string {
	quoted := make([]string, 0, len(auditColumnSpecList))
	for _, column := range auditColumnSpecList {
		quoted = append(quoted, quoteIdent(column.name))
	}
	return strings.Join(quoted, ", ")
}

// auditInsertPlaceholders 返回 INSERT 的占位符序列（整型列用 %d，其余用 '%s'），
// 与 auditColumnSpecList 同序，避免列与值错位。
func auditInsertPlaceholders() string {
	parts := make([]string, 0, len(auditColumnSpecList))
	for _, column := range auditColumnSpecList {
		switch column.typeCode {
		case "int", "bigint":
			parts = append(parts, "%d")
		default:
			parts = append(parts, "'%s'")
		}
	}
	return strings.Join(parts, ",")
}

// auditSelectColumns 列表查询列（不含请求/响应体与 search_text，避免详情内容撑爆列表）。
func auditSelectColumns() string {
	quoted := make([]string, 0, len(auditColumnSpecList))
	for _, column := range auditColumnSpecList {
		if auditDetailOnlyColumnSet[column.name] {
			continue
		}
		quoted = append(quoted, quoteIdent(column.name))
	}
	return strings.Join(quoted, ", ")
}

// auditDDL 审计表结构：attribute 列用于精确过滤，search_text 承载全文检索，
// created_at_ts 用毫秒时间戳做范围与排序（Manticore 不支持日期类型）。
func (r *AuditRepo) auditDDL() string {
	definitions := make([]string, 0, len(auditColumnSpecList))
	for _, column := range auditColumnSpecList {
		definitions = append(definitions, quoteIdent(column.name)+" "+column.typeCode)
	}
	return fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s(%s)"+
		" charset_table='non_cjk,0..9,A..Z->a..z,a..z' ngram_len='1' ngram_chars='cjk'",
		r.table, strings.Join(definitions, ", "))
}

// EnsureSchema 确保审计表存在、补齐新增列，并在检测到结构退化时自动重建（启动期调用一次）。
func (r *AuditRepo) EnsureSchema(ctx context.Context) error {
	if r.manticore == nil {
		return fmt.Errorf("manticore 未配置")
	}
	if err := r.manticore.ExecWrite(ctx, r.auditDDL()); err != nil {
		return err
	}
	if err := r.ensureColumns(ctx); err != nil {
		return err
	}
	return r.rebuildIfDegraded(ctx)
}

// ensureColumns 逐列补齐历史表缺失的新列（列已存在时忽略报错，保证可重复执行）。
func (r *AuditRepo) ensureColumns(ctx context.Context) error {
	existing, err := r.columnSet(ctx)
	if err != nil {
		return err
	}
	for _, name := range auditAddedColumnNames {
		if existing[name] {
			continue
		}
		sql := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", r.table, quoteIdent(name), auditColumnType(name))
		if err := r.manticore.ExecWrite(ctx, sql); err != nil {
			// 并发启动或列刚被其他实例补上：只记日志，不阻断启动
			if strings.Contains(strings.ToLower(err.Error()), "duplicate") ||
				strings.Contains(strings.ToLower(err.Error()), "exists") {
				continue
			}
			return fmt.Errorf("补齐审计列 %s: %w", name, err)
		}
	}
	return nil
}

// auditColumnType 返回列的 Manticore 类型（未登记时回落 string）。
func auditColumnType(name string) string {
	for _, column := range auditColumnSpecList {
		if column.name == name {
			return column.typeCode
		}
	}
	return "string"
}

// auditFilterCriticalColumnSet 过滤与排序所依赖的核心列：只要其中一列退化为 text，
// 列表筛选就必然失效（Manticore 只允许对 attribute 做过滤），值得重建。
// 用「核心列」而非「任意列」作为触发条件，避免个别列的类型误报导致误删历史审计。
var auditFilterCriticalColumnSet = map[string]bool{
	"log_level_code":  true,
	"span_type_code":  true,
	"trace_id":        true,
	"action_code":     true,
	"created_at_ts":   true,
	"root_span_flag":  true,
	"error_span_flag": true,
	"ai_span_flag":    true,
}

// rebuildIfDegraded 检测并修复退化表：EnsureSchema 失败后 INSERT 会触发 Manticore 自动建表，
// 把所有列建成 text（stored field），此时按 span_type_code / log_level_code 过滤会报
// "filters are supported only on attributes"，span 与级别筛选全部失效。
// 字段→attribute 的转换 Manticore 不支持，DROP + 重建是唯一修复手段，因此这里自动重建并明确告警。
func (r *AuditRepo) rebuildIfDegraded(ctx context.Context) error {
	degraded, err := r.degradedColumns(ctx)
	if err != nil {
		// 自愈是兜底手段：DESC 失败不应让建表成功的启动流程变成失败
		pkg.SLogger().Warnw("审计表结构退化检测失败，跳过自愈", "table", r.table, "error", err)
		return nil
	}
	if !hasCriticalDegradedColumn(degraded) {
		return nil
	}
	var dropped int64
	if total, countErr := r.CountAll(ctx); countErr == nil {
		dropped = total
	}
	if _, err := r.Rebuild(ctx); err != nil {
		return fmt.Errorf("重建退化审计表: %w", err)
	}
	pkg.SLogger().Errorw("检测到审计表结构退化，已重建",
		"table", r.table, "degraded_columns", degraded, "dropped_rows", dropped,
		"reason", "关键列被建成 text 字段，WHERE 过滤会报 filters are supported only on attributes")
	return nil
}

// hasCriticalDegradedColumn 判断退化列表里是否包含过滤核心列。
func hasCriticalDegradedColumn(degraded []string) bool {
	for _, name := range degraded {
		if auditFilterCriticalColumnSet[name] {
			return true
		}
	}
	return false
}

// degradedColumns 返回被建成 text 字段、本应为 attribute 的列（空表示结构正常）。
func (r *AuditRepo) degradedColumns(ctx context.Context) ([]string, error) {
	rows, err := r.manticore.Query(ctx, "DESC "+r.table)
	if err != nil {
		return nil, err
	}
	degraded := make([]string, 0, 4)
	for i := range rows {
		name := stringValue(rows[i]["Field"])
		if name == "" {
			name = stringValue(rows[i]["field"])
		}
		if name == "" || auditTextColumnSet[name] {
			continue
		}
		typeCode := strings.ToLower(stringValue(rows[i]["Type"]))
		if typeCode == "" {
			typeCode = strings.ToLower(stringValue(rows[i]["type"]))
		}
		// Manticore 各版本把全文字段报成 text 或 field，两者都算退化
		if typeCode == "text" || typeCode == "field" {
			degraded = append(degraded, name)
		}
	}
	return degraded, nil
}

// columnSet 读取当前表的列名集合（DESC 输出的 Field 列）。
func (r *AuditRepo) columnSet(ctx context.Context) (map[string]bool, error) {
	rows, err := r.manticore.Query(ctx, "DESC "+r.table)
	if err != nil {
		return nil, err
	}
	existing := make(map[string]bool, len(rows))
	for i := range rows {
		name := stringValue(rows[i]["Field"])
		if name == "" {
			name = stringValue(rows[i]["field"])
		}
		if name != "" {
			existing[name] = true
		}
	}
	return existing, nil
}

// Rebuild 删除并重建审计表（清理 CLI 使用，返回删除前条数）。
func (r *AuditRepo) Rebuild(ctx context.Context) (int64, error) {
	if r.manticore == nil {
		return 0, fmt.Errorf("manticore 未配置")
	}
	var before int64
	if total, err := r.CountAll(ctx); err == nil {
		before = total
	}
	if err := r.manticore.ExecWrite(ctx, "DROP TABLE IF EXISTS "+r.table); err != nil {
		return before, err
	}
	if err := r.manticore.ExecWrite(ctx, r.auditDDL()); err != nil {
		return before, err
	}
	return before, r.ensureColumns(ctx)
}

// Create 写入一条审计记录（search_text 由可检索字段拼接，供关键字命中）。
