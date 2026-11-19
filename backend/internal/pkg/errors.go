// 本文件定义业务错误类型与构造、转换函数。

package pkg

import (
	"fmt"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
)

// BizError 业务错误：携带字符串业务响应码，API 层据此映射 HTTP 状态并输出 RESP 包络。
type BizError struct {
	Code    string // 业务响应码（domain.RespCode* 族）
	Message string // 人类可读错误信息
}

// Error 实现 error 接口。
func (e *BizError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// NewBizError 构造业务错误。
func NewBizError(code string, message string) *BizError {
	return &BizError{Code: code, Message: message}
}

// WrapBizError 将任意 error 包装为业务错误；nil 入参返回 nil。
func WrapBizError(code string, message string, err error) *BizError {
	if err == nil {
		return nil
	}
	return &BizError{Code: code, Message: fmt.Sprintf("%s: %v", message, err)}
}

// AsBizError 将 error 转换为 BizError；非 BizError 统一归类为内部错误。
func AsBizError(err error) *BizError {
	if err == nil {
		return nil
	}
	if bizErr, ok := err.(*BizError); ok {
		return bizErr
	}
	return NewBizError(domain.RespCodeInternalError, err.Error())
}
