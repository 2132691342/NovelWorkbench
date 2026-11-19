// 本文件实现正文字数统计（汉字逐字，其余按空白分词）。

package pkg

import "unicode"

// TruncateText 按字符数截断文本（多字节安全），用于日志/通知等定长字段。
func TruncateText(text string, maxLength int) string {
	if maxLength <= 0 {
		return ""
	}
	runes := []rune(text)
	if len(runes) <= maxLength {
		return text
	}
	return string(runes[:maxLength])
}

// CountWords 统计正文字数：CJK 字符逐字计数，其余按空白分词计数（行业通用约定）。
func CountWords(text string) int64 {
	var wordCount int64
	inWord := false
	for _, runeValue := range text {
		switch {
		case unicode.Is(unicode.Han, runeValue):
			wordCount++ // 汉字逐字计
			inWord = false
		case unicode.IsSpace(runeValue):
			inWord = false
		default:
			if !inWord {
				wordCount++ // 连续拉丁字符算一个词
				inWord = true
			}
		}
	}
	return wordCount
}
