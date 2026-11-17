package pkg

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
)

// codeHTTPStatus 业务响应码 → HTTP 状态码的唯一映射点。
var codeHTTPStatus = map[string]int{
	domain.RespCodeSuccess:         http.StatusOK,                  // 成功
	domain.RespCodeInvalidParam:    http.StatusBadRequest,          // 参数错误
	domain.RespCodeUnauthorized:    http.StatusUnauthorized,        // 未登录
	domain.RespCodeForbidden:       http.StatusForbidden,           // 无权限
	domain.RespCodeNotFound:        http.StatusNotFound,            // 不存在
	domain.RespCodeConflict:        http.StatusConflict,            // 冲突
	domain.RespCodeBizFailed:       http.StatusBadRequest,          // 业务失败
	domain.RespCodeAIProviderError: http.StatusBadGateway,          // AI 供应商异常
	domain.RespCodeStorageError:    http.StatusServiceUnavailable,  // 基础设施异常
	domain.RespCodeInternalError:   http.StatusInternalServerError, // 内部错误
}

// HTTPStatusForCode 返回业务码对应的 HTTP 状态，未知码归为 500。
func HTTPStatusForCode(code string) int {
	if status, ok := codeHTTPStatus[code]; ok {
		return status
	}
	return http.StatusInternalServerError
}

// Ok 输出成功响应，data 可为任意业务数据（含 domain 的 RESP/VO 结构）。
func Ok(c *gin.Context, data any) {
	c.JSON(http.StatusOK, domain.DataRESP[any]{
		BaseRESP: domain.BaseRESP{RespCode: domain.RespCodeSuccess, RespMessage: "ok", TraceID: requestID(c)},
		Data:     data,
	})
}

// Fail 输出业务失败响应，HTTP 状态由业务码唯一映射。
func Fail(c *gin.Context, code string, message string) {
	c.JSON(HTTPStatusForCode(code), domain.DataRESP[any]{
		BaseRESP: domain.BaseRESP{RespCode: code, RespMessage: message, TraceID: requestID(c)},
		Data:     nil,
	})
}

// FailErr 将 error 转换为业务失败响应；BizError 保留业务码，其余归为内部错误。
func FailErr(c *gin.Context, err error) {
	var bizErr *BizError
	if errors.As(err, &bizErr) {
		Fail(c, bizErr.Code, bizErr.Message)
		return
	}
	Fail(c, domain.RespCodeInternalError, err.Error())
}

// requestID 取请求链路 ID（由中间件注入）。
func requestID(c *gin.Context) string {
	if value := c.GetString(ContextKeyRequestID); value != "" {
		return value
	}
	return ""
}
