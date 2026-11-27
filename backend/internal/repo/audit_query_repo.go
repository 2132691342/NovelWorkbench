// 本文件实现审计行的写入与查询：落库、分页与全文检索、单条与链路明细、
// 动作候选、按时间清理、条数统计，以及 Manticore 行到 DO 的转换。

package repo

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
)

func (r *AuditRepo) Create(ctx context.Context, do *domain.AuditLogDO) error {
	if r.manticore == nil {
		return fmt.Errorf("manticore 未配置，审计写入跳过")
	}
	createdAtTs := do.CreatedAt.UnixMilli()
	if createdAtTs <= 0 {
		createdAtTs = time.Now().UnixMilli()
	}
	sql := fmt.Sprintf("INSERT INTO %s(%s) VALUES (%s)",
		r.table, auditInsertColumns(), auditInsertPlaceholders())
	values := []any{
		escapeManticore(do.AuditULID), escapeManticore(do.LogLevelCode), escapeManticore(do.SpanTypeCode),
		escapeManticore(do.TraceID), escapeManticore(do.ParentSpanCode),
		boolInt(do.ParentSpanCode == ""), boolInt(do.LogLevelCode == domain.AuditLevelError),
		boolInt(do.LogLevelCode == domain.AuditLevelWarn),
		boolInt(do.SpanTypeCode == domain.AuditSpanTypeAICall),
		escapeManticore(do.UserULID), escapeManticore(do.UserName), escapeManticore(do.ActionCode),
		escapeManticore(do.ActionName), escapeManticore(do.ModelName), do.TokenCount,
		escapeManticore(do.RequestPath), escapeManticore(do.RequestMethod), escapeManticore(do.TargetType),
		escapeManticore(do.TargetULID), escapeManticore(do.RespCode), escapeManticore(do.RespMessage),
		do.HTTPStatus, do.LatencyMs, escapeManticore(do.IPAddr), escapeManticore(do.UserAgentText),
		escapeManticore(do.RequestBody), escapeManticore(do.RespBody),
		escapeManticore(auditSearchText(do)), createdAtTs,
	}
	return r.manticore.ExecWrite(ctx, fmt.Sprintf(sql, values...))
}

// auditSearchText 拼接全文检索字段（动作码、动作名、操作人、路径、说明、目标、模型与链路 ID）。
func auditSearchText(do *domain.AuditLogDO) string {
	return strings.Join([]string{
		do.ActionCode, do.ActionName, do.UserName, do.RequestPath, do.RespMessage,
		do.TargetType, do.TargetULID, do.TraceID, do.ModelName,
	}, " ")
}

// ListPage 分页查询审计记录（时间倒序），支持级别/span/动作/链路/用户/关键字/时间过滤。
func (r *AuditRepo) ListPage(ctx context.Context, req *domain.AuditListREQ) ([]domain.AuditLogDO, int64, error) {
	if r.manticore == nil {
		return nil, 0, fmt.Errorf("manticore 未配置")
	}
	condition := auditConditionOf(&auditCondition{
		KeywordText: req.KeywordText, ActionCode: req.ActionCode, LogLevelCode: req.LogLevelCode,
		SpanTypeCode: req.SpanTypeCode, TraceID: req.TraceID, UserULID: req.UserULID,
		TargetType: req.TargetType, TimeFrom: req.TimeFrom, TimeTo: req.TimeTo,
		DateFrom: req.DateFrom, DateTo: req.DateTo,
	})
	var total int64
	countRows, err := r.manticore.Query(ctx,
		fmt.Sprintf("SELECT COUNT(*) AS total FROM %s WHERE %s", r.table, condition))
	if err != nil {
		return nil, 0, err
	}
	if len(countRows) > 0 {
		total = int64Value(countRows[0]["total"])
	}
	if total == 0 {
		return []domain.AuditLogDO{}, 0, nil
	}
	sql := fmt.Sprintf("SELECT %s FROM %s WHERE %s ORDER BY `created_at_ts` DESC LIMIT %d OFFSET %d",
		auditSelectColumns(), r.table, condition, req.PageSize, (req.PageNum-1)*req.PageSize)
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

// auditCondition 审计筛选条件的中间表示（记录视图与链路视图共用同一套拼接逻辑）。
type auditCondition struct {
	KeywordText  string // 全文关键字
	ActionCode   string // 动作码
	LogLevelCode string // 级别
	SpanTypeCode string // span 类型
	TraceID      string // 链路 ID 精确匹配
	UserULID     string // 操作人
	TargetType   string // 目标类型
	TimeFrom     string // 起始时刻 RFC3339
	TimeTo       string // 结束时刻 RFC3339
	DateFrom     string // 起始日期 YYYY-MM-DD
	DateTo       string // 结束日期 YYYY-MM-DD
}

// auditConditionOf 组装 WHERE 条件（Manticore 只支持一个 MATCH，关键字与动作模糊合并为一次匹配）。
func auditConditionOf(cond *auditCondition) string {
	clauses := make([]string, 0, 8)
	matchParts := make([]string, 0, 2)
	if keyword := strings.TrimSpace(cond.KeywordText); keyword != "" {
		matchParts = append(matchParts, `"`+escapeManticore(keyword)+`"`)
	}
	if actionCode := strings.TrimSpace(cond.ActionCode); actionCode != "" {
		matchParts = append(matchParts, `"`+escapeManticore(actionCode)+`"`)
	}
	if len(matchParts) > 0 {
		// @search_text 是全文匹配的字段引用语法，不走标识符反引号
		clauses = append(clauses, "MATCH('@search_text "+strings.Join(matchParts, " ")+"')")
	}
	if cond.LogLevelCode != "" {
		clauses = append(clauses, quoteIdent("log_level_code")+" = '"+escapeManticore(cond.LogLevelCode)+"'")
	}
	if cond.SpanTypeCode != "" {
		clauses = append(clauses, quoteIdent("span_type_code")+" = '"+escapeManticore(cond.SpanTypeCode)+"'")
	}
	if cond.TraceID != "" {
		clauses = append(clauses, quoteIdent("trace_id")+" = '"+escapeManticore(cond.TraceID)+"'")
	}
	if cond.UserULID != "" {
		clauses = append(clauses, quoteIdent("user_ulid")+" = '"+escapeManticore(cond.UserULID)+"'")
	}
	if cond.TargetType != "" {
		clauses = append(clauses, quoteIdent("target_type")+" = '"+escapeManticore(cond.TargetType)+"'")
	}
	// 精确时刻优先于日期（快捷范围「近 1 小时」需要分钟级精度）
	fromTs, fromOK := timestampMilli(cond.TimeFrom)
	if !fromOK {
		fromTs, fromOK = dateStartMilli(cond.DateFrom)
	}
	if fromOK {
		clauses = append(clauses, fmt.Sprintf("%s >= %d", quoteIdent("created_at_ts"), fromTs))
	}
	toTs, toOK := timestampMilli(cond.TimeTo)
	if !toOK {
		toTs, toOK = dateEndMilli(cond.DateTo)
	}
	if toOK {
		clauses = append(clauses, fmt.Sprintf("%s <= %d", quoteIdent("created_at_ts"), toTs))
	}
	if len(clauses) == 0 {
		return quoteIdent("created_at_ts") + " > 0"
	}
	return strings.Join(clauses, " AND ")
}

// GetByULID 按 ULID 查询审计详情（含请求体与响应体），不存在时返回 (nil, nil)。
func (r *AuditRepo) GetByULID(ctx context.Context, auditULID string) (*domain.AuditLogDO, error) {
	if r.manticore == nil {
		return nil, fmt.Errorf("manticore 未配置")
	}
	sql := fmt.Sprintf("SELECT %s, `request_body`, `resp_body` FROM %s WHERE `audit_ulid` = '%s' LIMIT 1",
		auditSelectColumns(), r.table, escapeManticore(auditULID))
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

// ListByTrace 按链路 ID 查询同一次请求的全部 span（时间升序，含请求/响应摘要）。
func (r *AuditRepo) ListByTrace(ctx context.Context, traceID string, limit int) ([]domain.AuditLogDO, error) {
	if r.manticore == nil {
		return nil, fmt.Errorf("manticore 未配置")
	}
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	sql := fmt.Sprintf("SELECT %s, `request_body`, `resp_body` FROM %s WHERE `trace_id` = '%s' ORDER BY `created_at_ts` ASC LIMIT %d",
		auditSelectColumns(), r.table, escapeManticore(traceID), limit)
	rows, err := r.manticore.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	dos := make([]domain.AuditLogDO, 0, len(rows))
	for i := range rows {
		do := auditDOOf(rows[i])
		do.RequestBody = stringValue(rows[i]["request_body"])
		do.RespBody = stringValue(rows[i]["resp_body"])
		dos = append(dos, do)
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
	sql := fmt.Sprintf("SELECT %s, COUNT(*) AS total FROM %s "+
		"GROUP BY %s ORDER BY total DESC LIMIT %d",
		quoteIdent("action_code"), r.table, quoteIdent("action_code"), limit)
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
		fmt.Sprintf("SELECT COUNT(*) AS total FROM %s WHERE `created_at_ts` < %d", r.table, cutoffTs))
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
	if err := r.manticore.ExecWrite(ctx, fmt.Sprintf("DELETE FROM %s WHERE `created_at_ts` < %d", r.table, cutoffTs)); err != nil {
		return 0, err
	}
	return affected, nil
}

// CountAll 统计审计总条数（运行状态页与留存摘要展示）。
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

// ListTracePage 按链路分页查询：先用聚合拿到当前页的 trace_id 与链路指标，
// 再取这些链路的全部 span 供服务层汇总（Manticore 不支持 MAX(string)，
// 最慢 span 名称、根 span 字段只能在拿到行之后在内存里算）。
func auditDOOf(row map[string]any) domain.AuditLogDO {
	createdAtTs := int64Value(row["created_at_ts"])
	return domain.AuditLogDO{
		BaseDO:         domain.BaseDO{CreatedAt: time.UnixMilli(createdAtTs)},
		AuditULID:      stringValue(row["audit_ulid"]),
		LogLevelCode:   stringValue(row["log_level_code"]),
		SpanTypeCode:   stringValue(row["span_type_code"]),
		TraceID:        stringValue(row["trace_id"]),
		ParentSpanCode: stringValue(row["parent_span_code"]),
		UserULID:       stringValue(row["user_ulid"]),
		UserName:       stringValue(row["user_name"]),
		ActionCode:     stringValue(row["action_code"]),
		ActionName:     stringValue(row["action_name"]),
		ModelName:      stringValue(row["model_name"]),
		TokenCount:     int(int64Value(row["token_count"])),
		RequestPath:    stringValue(row["request_path"]),
		RequestMethod:  stringValue(row["request_method"]),
		TargetType:     stringValue(row["target_type"]),
		TargetULID:     stringValue(row["target_ulid"]),
		RespCode:       stringValue(row["resp_code"]),
		RespMessage:    stringValue(row["resp_message"]),
		HTTPStatus:     int(int64Value(row["http_status"])),
		LatencyMs:      int64Value(row["latency_ms"]),
		IPAddr:         stringValue(row["ip_addr"]),
		UserAgentText:  stringValue(row["user_agent_text"]),
	}
}

// escapeManticore 转义写入 SQL 字符串字面量的特殊字符。
var manticoreAuditEscaper = strings.NewReplacer(`\`, `\\`, `'`, `\'`, `"`, `\"`)

func escapeManticore(value string) string {
	return manticoreAuditEscaper.Replace(value)
}

// boolInt 把布尔派生标记转成 Manticore 可聚合的 0/1 整数。
func boolInt(flag bool) int {
	if flag {
		return 1
	}
	return 0
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

// timestampMilli 把 RFC3339 时刻解析为毫秒时间戳（空值或非法值返回 false）。
func timestampMilli(timeText string) (int64, bool) {
	if strings.TrimSpace(timeText) == "" {
		return 0, false
	}
	parsed, err := time.Parse(time.RFC3339, timeText)
	if err != nil {
		return 0, false
	}
	return parsed.UnixMilli(), true
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
