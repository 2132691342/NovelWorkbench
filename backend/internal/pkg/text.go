package pkg

import "unicode"

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
