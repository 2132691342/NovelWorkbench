// 本文件实现密码强度规则的唯一来源：新建用户、管理员重置密码、本人改密共用同一套判定。
// 前端各表单的提示文案与此处规则必须一致，否则会出现「前端提示合格、后端仍拒绝」的割裂。

package service

import (
	"strings"
	"unicode"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
)

// passwordMinLength 密码最小长度（与前端强度提示一致）。
const passwordMinLength = 8

// validatePasswordStrength 校验密码强度：≥8 位，且同时包含字母与数字。
// 只用于设置新密码的入口（创建用户、重置密码、本人改密）；登录比对不经过这里。
func validatePasswordStrength(passwordText string) error {
	if len([]rune(passwordText)) < passwordMinLength {
		return pkg.NewBizError(domain.RespCodeInvalidParam, "密码至少 8 位")
	}
	hasLetter, hasDigit := false, false
	for _, char := range passwordText {
		switch {
		case unicode.IsLetter(char):
			hasLetter = true
		case unicode.IsDigit(char):
			hasDigit = true
		}
		if hasLetter && hasDigit {
			break
		}
	}
	if !hasLetter || !hasDigit {
		return pkg.NewBizError(domain.RespCodeInvalidParam, "密码需同时包含字母与数字")
	}
	if strings.TrimSpace(passwordText) == "" {
		return pkg.NewBizError(domain.RespCodeInvalidParam, "密码不能全为空白")
	}
	return nil
}
