// 本文件实现审计日志的持久化仓储：落库在 Manticore 全文表，支持分页、全文检索、
// 链路聚合与按时间清理。PostgreSQL 不再保存审计行（历史表在启动时清理）。

package repo

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/infra"
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

// auditDDL 审计表结构：字符串列用于精确过滤，search_text 承载全文检索，
// created_at_ts 用毫秒时间戳做范围与排序（Manticore 不支持日期类型）。
func (r *AuditRepo) auditDDL() string {
	return fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s("+
		"audit_ulid string, log_level_code string, trace_id string, user_ulid string, user_name string, "+
		"action_code string, request_path string, request_method string, target_type string, target_ulid string, "+
		"resp_code string, resp_message text, http_status int, latency_ms int, ip_addr string, "+
		"user_agent_text text, request_body text, resp_body text, search_text text, created_at_ts bigint"+
		") charset_table='non_cjk,0..9,A..Z->a..z,a..z' ngram_len='1' ngram_chars='cjk'", r.table)
}

// EnsureSchema 确保审计表存在（启动期调用一次）。
func (r *AuditRepo) EnsureSchema(ctx context.Context) error {
	if r.manticore == nil {
		return fmt.Errorf("manticore 未配置")
	}
	return r.manticore.ExecWrite(ctx, r.auditDDL())
}

// Create 写入一条审计记录（search_text 由可检索字段拼接，供关键字命中）。
func (r *AuditRepo) Create(ctx context.Context, do *domain.AuditLogDO) error {
	if r.manticore == nil {
		return fmt.Errorf("manticore 未配置，审计写入跳过")
	}
	createdAtTs := do.CreatedAt.UnixMilli()
	if createdAtTs <= 0 {
		createdAtTs = time.Now().UnixMilli()
	}
	sql := fmt.Sprintf("INSERT INTO %s(audit_ulid, log_level_code, trace_id, user_ulid, user_name, "+
		"action_code, request_path, request_method, target_type, target_ulid, resp_code, resp_message, "+
		"http_status, latency_ms, ip_addr, user_agent_text, request_body, resp_body, search_text, created_at_ts) "+
		"VALUES ('%s','%s','%s','%s','%s','%s','%s','%s','%s','%s','%s','%s',%d,%d,'%s','%s','%s','%s','%s',%d)",
		r.table,
		escapeManticore(do.AuditULID), escapeManticore(do.LogLevelCode), escapeManticore(do.TraceID),
		escapeManticore(do.UserULID), escapeManticore(do.UserName), escapeManticore(do.ActionCode),
		escapeManticore(do.RequestPath), escapeManticore(do.RequestMethod), escapeManticore(do.TargetType),
		escapeManticore(do.TargetULID), escapeManticore(do.RespCode), escapeManticore(do.RespMessage),
		do.HTTPStatus, do.LatencyMs, escapeManticore(do.IPAddr), escapeManticore(do.UserAgentText),
		escapeManticore(do.RequestBody), escapeManticore(do.RespBody),
		escapeManticore(auditSearchText(do)), createdAtTs)
	return r.manticore.ExecWrite(ctx, sql)
}

// auditSearchText 拼接全文检索字段（动作码、操作人、路径、说明、目标与链路 ID）。
func auditSearchText(do *domain.AuditLogDO) string {
	return strings.Join([]string{
		do.ActionCode, do.UserName, do.RequestPath, do.RespMessage,
		do.TargetType, do.TargetULID, do.TraceID,
	}, " ")
}

// auditSelectColumns 列表查询列（不含请求/响应体，避免详情内容撑爆列表）。
const auditSelectColumns = "audit_ulid, log_level_code, trace_id, user_ulid, user_name, action_code, " +
	"request_path, request_method, target_type, target_ulid, resp_code, resp_message, http_status, " +
	"latency_ms, ip_addr, user_agent_text, created_at_ts"

// ListPage 分页查询审计日志（时间倒序），支持级别/动作/链路/用户/关键字/日期过滤。
func (r *AuditRepo) ListPage(ctx context.Context, req *domain.AuditListREQ) ([]domain.AuditLogDO, int64, error) {
	if r.manticore == nil {
		return nil, 0, fmt.Errorf("manticore 未配置")
	}
	condition := r.buildCondition(req)
	var total int64
	countRows, err := r.manticore.Query(ctx, fmt.Sprintf("SELECT COUNT(*) AS total FROM %s WHERE %s", r.table, condition))
	if err != nil {
		return nil, 0, err
	}
	if len(countRows) > 0 {
		total = int64Value(countRows[0]["total"])
	}
	if total == 0 {
		return []domain.AuditLogDO{}, 0, nil
	}
	sql := fmt.Sprintf("SELECT %s FROM %s WHERE %s ORDER BY created_at_ts DESC LIMIT %d OFFSET %d",
		auditSelectColumns, r.table, condition, req.PageSize, (req.PageNum-1)*req.PageSize)
	rows, err := r.manticore.Query(ctx, sql)
	if err != nil {
		return nil, 0, err
	}
	dos := make([]domain.AuditLogDO, 0, len(rows))
	for i := range rows {
		dos = append(dos, auditDOOf(rows[i]))
	}
	return dos, total, nil
}

// buildCondition 组装 WHERE 条件（Manticore 只支持一个 MATCH，关键字与动作模糊合并为一次匹配）。
func (r *AuditRepo) buildCondition(req *domain.AuditListREQ) string {
	clauses := make([]string, 0, 8)
	matchParts := make([]string, 0, 2)
	if keyword := strings.TrimSpace(req.KeywordText); keyword != "" {
		matchParts = append(matchParts, `"`+escapeManticore(keyword)+`"`)
	}
	if actionCode := strings.TrimSpace(req.ActionCode); actionCode != "" {
		matchParts = append(matchParts, `"`+escapeManticore(actionCode)+`"`)
	}
	if len(matchParts) > 0 {
		clauses = append(clauses, "MATCH('@search_text "+strings.Join(matchParts, " ")+"')")
	}
	if req.LogLevelCode != "" {
		clauses = append(clauses, "log_level_code = '"+escapeManticore(req.LogLevelCode)+"'")
	}
	if req.TraceID != "" {
		clauses = append(clauses, "trace_id = '"+escapeManticore(req.TraceID)+"'")
	}
	if req.UserULID != "" {
		clauses = append(clauses, "user_ulid = '"+escapeManticore(req.UserULID)+"'")
	}
	if req.TargetType != "" {
		clauses = append(clauses, "target_type = '"+escapeManticore(req.TargetType)+"'")
	}
	if fromTs, ok := dateStartMilli(req.DateFrom); ok {
		clauses = append(clauses, fmt.Sprintf("created_at_ts >= %d", fromTs))
	}
	if toTs, ok := dateEndMilli(req.DateTo); ok {
		clauses = append(clauses, fmt.Sprintf("created_at_ts <= %d", toTs))
	}
	if len(clauses) == 0 {
		return "created_at_ts > 0"
	}
	return strings.Join(clauses, " AND ")
}

// GetByULID 按 ULID 查询审计详情（含请求体与响应体），不存在时返回 (nil, nil)。
func (r *AuditRepo) GetByULID(ctx context.Context, auditULID string) (*domain.AuditLogDO, error) {
	if r.manticore == nil {
		return nil, fmt.Errorf("manticore 未配置")
	}
	sql := fmt.Sprintf("SELECT %s, request_body, resp_body FROM %s WHERE audit_ulid = '%s' LIMIT 1",
		auditSelectColumns, r.table, escapeManticore(auditULID))
	rows, err := r.manticore.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	do := auditDOOf(rows[0])
	do.RequestBody = stringValue(rows[0]["request_body"])
	do.RespBody = stringValue(rows[0]["resp_body"])
	return &do, nil
}

// ListByTrace 按链路 ID 查询同一次请求的全部审计行（时间升序，构成时间线）。
func (r *AuditRepo) ListByTrace(ctx context.Context, traceID string, limit int) ([]domain.AuditLogDO, error) {
	if r.manticore == nil {
		return nil, fmt.Errorf("manticore 未配置")
	}
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	sql := fmt.Sprintf("SELECT %s FROM %s WHERE trace_id = '%s' ORDER BY created_at_ts ASC LIMIT %d",
		auditSelectColumns, r.table, escapeManticore(traceID), limit)
	rows, err := r.manticore.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	dos := make([]domain.AuditLogDO, 0, len(rows))
	for i := range rows {
		dos = append(dos, auditDOOf(rows[i]))
	}
	return dos, nil
}

// ListActionOptions 汇总动作码及其条数（筛选下拉，按条数倒序）。
func (r *AuditRepo) ListActionOptions(ctx context.Context, limit int) ([]domain.AuditActionOptionVO, error) {
	if r.manticore == nil {
		return nil, fmt.Errorf("manticore 未配置")
	}
	if limit <= 0 || limit > 200 {
		limit = 60
	}
	sql := fmt.Sprintf("SELECT action_code, COUNT(*) AS total FROM %s GROUP BY action_code ORDER BY total DESC LIMIT %d",
		r.table, limit)
	rows, err := r.manticore.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	options := make([]domain.AuditActionOptionVO, 0, len(rows))
	for i := range rows {
		options = append(options, domain.AuditActionOptionVO{
			ActionCode: stringValue(rows[i]["action_code"]),
			Total:      int64Value(rows[i]["total"]),
		})
	}
	return options, nil
}

// DeleteBefore 删除指定时间之前的审计行（每日留存清理）。
func (r *AuditRepo) DeleteBefore(ctx context.Context, cutoff time.Time) (int, error) {
	if r.manticore == nil {
		return 0, fmt.Errorf("manticore 未配置")
	}
	cutoffTs := cutoff.UnixMilli()
	countRows, err := r.manticore.Query(ctx,
		fmt.Sprintf("SELECT COUNT(*) AS total FROM %s WHERE created_at_ts < %d", r.table, cutoffTs))
	if err != nil {
		return 0, err
	}
	affected := 0
	if len(countRows) > 0 {
		affected = int(int64Value(countRows[0]["total"]))
	}
	if affected == 0 {
		return 0, nil
	}
	if err := r.manticore.ExecWrite(ctx, fmt.Sprintf("DELETE FROM %s WHERE created_at_ts < %d", r.table, cutoffTs)); err != nil {
		return 0, err
	}
	return affected, nil
}

// CountAll 统计审计总条数（运行状态页展示留存规模）。
func (r *AuditRepo) CountAll(ctx context.Context) (int64, error) {
	if r.manticore == nil {
		return 0, fmt.Errorf("manticore 未配置")
	}
	rows, err := r.manticore.Query(ctx, fmt.Sprintf("SELECT COUNT(*) AS total FROM %s", r.table))
	if err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, nil
	}
	return int64Value(rows[0]["total"]), nil
}

// auditDOOf 把查询行映射为审计持久化对象（时间为毫秒时间戳还原）。
func auditDOOf(row map[string]any) domain.AuditLogDO {
	createdAtTs := int64Value(row["created_at_ts"])
	return domain.AuditLogDO{
		BaseDO:        domain.BaseDO{CreatedAt: time.UnixMilli(createdAtTs)},
		AuditULID:     stringValue(row["audit_ulid"]),
		LogLevelCode:  stringValue(row["log_level_code"]),
		TraceID:       stringValue(row["trace_id"]),
		UserULID:      stringValue(row["user_ulid"]),
		UserName:      stringValue(row["user_name"]),
		ActionCode:    stringValue(row["action_code"]),
		RequestPath:   stringValue(row["request_path"]),
		RequestMethod: stringValue(row["request_method"]),
		TargetType:    stringValue(row["target_type"]),
		TargetULID:    stringValue(row["target_ulid"]),
		RespCode:      stringValue(row["resp_code"]),
		RespMessage:   stringValue(row["resp_message"]),
		HTTPStatus:    int(int64Value(row["http_status"])),
		LatencyMs:     int64Value(row["latency_ms"]),
		IPAddr:        stringValue(row["ip_addr"]),
		UserAgentText: stringValue(row["user_agent_text"]),
	}
}

// escapeManticore 转义写入 SQL 字符串字面量的特殊字符。
var manticoreAuditEscaper = strings.NewReplacer(`\`, `\\`, `'`, `\'`, `"`, `\"`)

func escapeManticore(value string) string {
	return manticoreAuditEscaper.Replace(value)
}

// stringValue 读取字符串列（Manticore 可能把空串返回为缺失字段）。
func stringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case json.Number:
		return typed.String()
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	default:
		return ""
	}
}

// int64Value 读取整数列（JSON 数字统一按 float64 解码）。
func int64Value(value any) int64 {
	switch typed := value.(type) {
	case float64:
		return int64(typed)
	case int64:
		return typed
	case int:
		return int64(typed)
	case json.Number:
		parsed, _ := typed.Int64()
		return parsed
	case string:
		parsed, _ := strconv.ParseInt(typed, 10, 64)
		return parsed
	default:
		return 0
	}
}

// dateStartMilli 把 YYYY-MM-DD 解析为当日 00:00:00 的毫秒时间戳。
func dateStartMilli(dateText string) (int64, bool) {
	if strings.TrimSpace(dateText) == "" {
		return 0, false
	}
	parsed, err := time.ParseInLocation("2006-01-02", dateText, time.Local)
	if err != nil {
		return 0, false
	}
	return parsed.UnixMilli(), true
}

// dateEndMilli 把 YYYY-MM-DD 解析为当日 23:59:59.999 的毫秒时间戳。
func dateEndMilli(dateText string) (int64, bool) {
	startTs, ok := dateStartMilli(dateText)
	if !ok {
		return 0, false
	}
	return startTs + 24*60*60*1000 - 1, true
}
