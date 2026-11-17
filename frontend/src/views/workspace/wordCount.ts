// 正文字数统计（与后端 pkg.CountWords 口径一致：CJK 逐字计数，其余按空白分词计数）

/** 汉字（Han 统一表意文字）判定 */
const NW_HAN_REGEXP = /\p{Script=Han}/u
/** 空白字符判定 */
const NW_SPACE_REGEXP = /\s/u

/**
 * 统计正文字数：CJK 字符逐字计数，其余按空白分词计数（行业通用约定）。
 * 与后端 pkg.CountWords 同口径，保证前端实时字数与落库字数一致。
 */
export function countWords(text: string): number {
  let wordCount = 0
  let inWord = false
  for (const charText of text) {
    if (NW_HAN_REGEXP.test(charText)) {
      wordCount += 1 // 汉字逐字计
      inWord = false
    } else if (NW_SPACE_REGEXP.test(charText)) {
      inWord = false
    } else if (!inWord) {
      wordCount += 1 // 连续拉丁字符算一个词
      inWord = true
    }
  }
  return wordCount
}
