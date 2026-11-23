// 本文件实现工具触发词的解析与选取：对话期既支持请求体显式指定（tool_code_list），
// 也支持正文内联 #code。两者按「显式优先、去重保序」合并，关键词自动触发只在还有余量时补位。

package service

import (
	"strings"
	"unicode"

	"github.com/2132691342/NovelWorkbench/backend/internal/ai"
	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
)

// 对话期工具选取上限（显式与自动共享额度，避免一次注入过多上下文）。
const (
	toolExplicitLimit = 3 // 显式 #code / tool_code_list 上限
	toolTotalLimit    = 3 // 单轮工具总数上限
)

// parseHashToolCodes 从正文中提取 #code 触发词，并返回去掉触发词后的正文。
// 触发词必须位于行首或空白之后、以空白或行尾结束，正文里的 #话题 之类不会被误判；
// 删除时连同其后的一个空白一并去掉，保留其余换行与缩进。
func parseHashToolCodes(contentText string) ([]string, string) {
	if !strings.Contains(contentText, "#") {
		return nil, contentText
	}
	var builder strings.Builder
	builder.Grow(len(contentText))
	codeList := make([]string, 0, 2)
	runes := []rune(contentText)
	for i := 0; i < len(runes); i++ {
		if runes[i] != '#' || (i > 0 && !unicode.IsSpace(runes[i-1])) {
			builder.WriteRune(runes[i])
			continue
		}
		end := i + 1
		for end < len(runes) && isToolCodeRune(runes[end]) {
			end++
		}
		code := string(runes[i+1 : end])
		atBoundary := end == len(runes) || unicode.IsSpace(runes[end])
		if code == "" || len(code) > 32 || !atBoundary {
			builder.WriteRune(runes[i])
			continue
		}
		codeList = append(codeList, code)
		i = end - 1
		if end < len(runes) && unicode.IsSpace(runes[end]) {
			i = end // 连同触发词后的一个空白一起吞掉
		}
	}
	if len(codeList) == 0 {
		return nil, contentText
	}
	return codeList, strings.TrimSpace(builder.String())
}

// isToolCodeRune 判断是否为合法工具编码字符（小写字母、数字与下划线）。
func isToolCodeRune(char rune) bool {
	return (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '_'
}

// mergeToolCodes 合并显式与内联编码：去重保序，超出上限截断。
func mergeToolCodes(explicitList, parsedList []string) []string {
	seen := make(map[string]bool, len(explicitList)+len(parsedList))
	merged := make([]string, 0, len(explicitList)+len(parsedList))
	for _, code := range append(append([]string{}, explicitList...), parsedList...) {
		trimmed := strings.TrimSpace(code)
		if trimmed == "" || seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		merged = append(merged, trimmed)
		if len(merged) >= toolExplicitLimit {
			break
		}
	}
	return merged
}

// selectToolsByCode 按编码挑选工具：不存在或已停用的编码记入 missingList。
// 显式触发必须报错而不是静默跳过，否则用户以为体检跑过了。
func selectToolsByCode(dos []domain.ToolDO, codeList []string) ([]domain.ToolVO, []string) {
	byCode := make(map[string]*domain.ToolDO, len(dos))
	for i := range dos {
		byCode[dos[i].ToolCode] = &dos[i]
	}
	voList := make([]domain.ToolVO, 0, len(codeList))
	missingList := make([]string, 0, 1)
	for _, code := range codeList {
		do := byCode[code]
		if do == nil || !do.EnabledFlag {
			missingList = append(missingList, code)
			continue
		}
		voList = append(voList, toToolVO(do))
	}
	return voList, missingList
}

// matchToolsByKeyword 按关键词匹配自动触发工具：跳过已在显式列表中的编码，最多补到 limit 个。
func matchToolsByKeyword(dos []domain.ToolDO, contentText string, excludeSet map[string]bool,
	limit int) []domain.ToolVO {
	if strings.TrimSpace(contentText) == "" || limit <= 0 {
		return nil
	}
	voList := make([]domain.ToolVO, 0, limit)
	for i := range dos {
		if excludeSet[dos[i].ToolCode] || !dos[i].EnabledFlag {
			continue
		}
		for _, keyword := range ai.ToolKeywordsOf(dos[i].TriggerKeywords) {
			if strings.Contains(contentText, keyword) {
				voList = append(voList, toToolVO(&dos[i]))
				break
			}
		}
		if len(voList) >= limit {
			break
		}
	}
	return voList
}
