// 本文件实现斜杠命令解析与提示词占位符校验的纯函数。

package service

import (
	"strings"

	"github.com/2132691342/NovelWorkbench/backend/internal/ai"
	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
)

// resolveSlashCode 解析提示词编码与剩余正文（纯函数）：
// 显式传入的 prompt_code 优先；否则解析正文首 token 的 /code，命中才剥离命令；
// 未命中提示词时正文原样返回。
func resolveSlashCode(explicitCode, contentText string) (string, string) {
	if explicitCode != "" {
		return explicitCode, contentText
	}
	parsedCode, parsedRemainder := parseSlashToken(contentText)
	return parsedCode, parsedRemainder
}

// parseSlashToken 解析正文首 token 的 /code：返回编码与去掉命令后的正文。
func parseSlashToken(contentText string) (string, string) {
	trimmed := strings.TrimSpace(contentText)
	if !strings.HasPrefix(trimmed, "/") {
		return "", trimmed
	}
	end := strings.IndexAny(trimmed, " \t\r\n")
	if end < 0 {
		return "", ""
	}
	code := strings.TrimPrefix(trimmed[:end], "/")
	if code == "" {
		return "", trimmed
	}
	return code, strings.TrimSpace(trimmed[end:])
}

// validatePromptPlaceholders 校验提示词中的占位符都在内置白名单内：
// 未登记的占位符会被渲染为空串，等于静默丢上下文，必须在这里拦下。
func validatePromptPlaceholders(promptText string) error {
	unknown := ai.TemplatePlaceholders(promptText, ai.BuiltinPlaceholders)
	if len(unknown) > 0 {
		return pkg.NewBizError(domain.RespCodeInvalidParam,
			"提示词中的占位符未登记: "+strings.Join(unknown, ","))
	}
	return nil
}
