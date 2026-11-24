// 本文件实现工具模块的视图组装、参数序列化与文本辅助函数（工具服务主体见 tool_service.go）。

package service

import (
	"encoding/json"
	"strings"

	"github.com/2132691342/NovelWorkbench/backend/internal/ai"
	"github.com/2132691342/NovelWorkbench/backend/internal/ai/check"
	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
)

// toToolVO 组装工具视图对象。
func toToolVO(do *domain.ToolDO) domain.ToolVO {
	return domain.ToolVO{
		ToolULID:        do.ToolULID,
		ToolCode:        do.ToolCode,
		ToolName:        do.ToolName,
		ToolTypeCode:    do.ToolTypeCode,
		DescriptionText: do.DescriptionText,
		TriggerKeywords: do.TriggerKeywords,
		ParamList:       unmarshalToolParams(do.ParamSchemaJSON),
		EnabledFlag:     do.EnabledFlag,
		BuiltinFlag:     do.BuiltinFlag,
		CustomizedFlag:  do.CustomizedFlag,
		SortOrder:       do.SortOrder,
		CreatedAt:       pkg.FormatTime(do.CreatedAt),
		UpdatedAt:       pkg.FormatTime(do.UpdatedAt),
	}
}

// marshalToolParams 序列化参数声明（空列表存 "[]"）。
func marshalToolParams(paramList []domain.ToolParamDO) string {
	if len(paramList) == 0 {
		return "[]"
	}
	data, err := json.Marshal(paramList)
	if err != nil {
		return "[]"
	}
	return string(data)
}

// unmarshalToolParams 反序列化参数声明（损坏数据按空列表处理）。
func unmarshalToolParams(paramJSON string) []domain.ToolParamDO {
	paramList := make([]domain.ToolParamDO, 0, 4)
	if paramJSON == "" {
		return paramList
	}
	if err := json.Unmarshal([]byte(paramJSON), &paramList); err != nil {
		return []domain.ToolParamDO{}
	}
	return paramList
}

// toToolParamList 内置参数声明转领域结构。
func toToolParamList(paramList []ai.ToolParam) []domain.ToolParamDO {
	result := make([]domain.ToolParamDO, 0, len(paramList))
	for _, param := range paramList {
		result = append(result, domain.ToolParamDO{
			ParamName: param.ParamName, ParamLabel: param.ParamLabel,
			RequiredFlag: param.RequiredFlag, DefaultValue: param.DefaultValue,
			DescriptionText: param.DescriptionText,
		})
	}
	return result
}

// hookRelatedToContent 判断伏笔描述里的关键词是否出现在正文（简易分词：中文双字片段）。
func hookRelatedToContent(hook *domain.ForeshadowHookDO, contentText string) bool {
	if strings.TrimSpace(contentText) == "" {
		return false
	}
	text := hook.DescriptionText + " " + hook.ExpectedPayoffText
	runes := []rune(text)
	for i := 0; i+1 < len(runes); i++ {
		pair := string(runes[i : i+2])
		if isHanPair(pair) && strings.Contains(contentText, pair) {
			return true
		}
	}
	return false
}

// isHanPair 判断两字符是否都是汉字（用于粗粒度关键词命中）。
func isHanPair(pair string) bool {
	runes := []rune(pair)
	if len(runes) != 2 {
		return false
	}
	for _, runeValue := range runes {
		if runeValue < 0x4E00 || runeValue > 0x9FFF {
			return false
		}
	}
	return true
}

// splitNonEmptyLines 按行切分并去掉空行。
func splitNonEmptyLines(text string) []string {
	rawList := strings.FieldsFunc(text, func(r rune) bool { return r == '\n' || r == '\r' })
	result := make([]string, 0, len(rawList))
	for _, item := range rawList {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// firstSentenceOf 取段落首句（到第一个句末标点为止）。
func firstSentenceOf(paragraph string) string {
	runes := []rune(strings.TrimSpace(paragraph))
	for index, runeValue := range runes {
		if strings.ContainsRune("。！？!?…", runeValue) && index >= 8 {
			return string(runes[:index+1])
		}
	}
	if len(runes) <= 60 {
		return string(runes)
	}
	return string(runes[:60]) + "…"
}

// lastSentenceOf 取末句（最后一个句末标点之后到结尾，过长则截断）。
func lastSentenceOf(text string) string {
	trimmed := strings.TrimSpace(text)
	runes := []rune(trimmed)
	start := len(runes) - 60
	if start < 0 {
		start = 0
	}
	return strings.TrimSpace(string(runes[start:]))
}

// stripHTMLTags 去掉富文本标签（设定正文可能是 HTML）。
func stripHTMLTags(text string) string {
	var builder strings.Builder
	inTag := false
	for _, runeValue := range text {
		switch {
		case runeValue == '<':
			inTag = true
		case runeValue == '>':
			inTag = false
			builder.WriteRune(' ')
		case !inTag:
			builder.WriteRune(runeValue)
		}
	}
	return strings.Join(strings.Fields(builder.String()), " ")
}

// intToText 整数转字符串。
func intToText(value int64) string {
	if value == 0 {
		return "0"
	}
	negative := value < 0
	if negative {
		value = -value
	}
	digits := make([]byte, 0, 12)
	for value > 0 {
		digits = append([]byte{byte('0' + value%10)}, digits...)
		value /= 10
	}
	if negative {
		return "-" + string(digits)
	}
	return string(digits)
}

// parseIntText 解析正整数文本。
func parseIntText(text string) (int, bool) {
	value := 0
	if text == "" {
		return 0, false
	}
	for _, runeValue := range text {
		if runeValue < '0' || runeValue > '9' {
			return 0, false
		}
		value = value*10 + int(runeValue-'0')
		if value > 100000 {
			return 0, false
		}
	}
	return value, true
}

// percentText 把 0-1 的比例格式化为百分比文本。
func percentText(ratio float64) string {
	return intToText(int64(ratio*100+0.5)) + "%"
}

// boolText 布尔转「是/否」。
func boolText(value bool) string {
	if value {
		return "是"
	}
	return "否"
}

// verdictLabel 体检结论中文标签。
func verdictLabel(verdictCode string) string {
	switch verdictCode {
	case check.VerdictPass:
		return "可发"
	case check.VerdictRevise:
		return "需修"
	default:
		return "不可发"
	}
}

// levelLabel 问题级别中文标签。
func levelLabel(levelCode string) string {
	switch levelCode {
	case "block":
		return "阻断"
	case "warn":
		return "警示"
	default:
		return "建议"
	}
}
