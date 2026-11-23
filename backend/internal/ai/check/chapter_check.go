// 本文件实现章节量化体检：把可机检的门禁项做成纯函数，供对话期工具调用与
// 管理端体检面板共用。覆盖零容忍词、套路句式、情绪直述、段落均匀度与章末钩子，
// 按 block / warn 分级输出并给出综合分与结论。

package check

import (
	"sort"
	"strings"
	"unicode"
)

// 门禁阈值（每章基准，可按章节字数比例放宽由调用方处理）。
const (
	thresholdDashMax      = 12 // 破折号上限
	thresholdBannedZero   = 0  // 零容忍词
	thresholdWarnMax      = 3  // 警示词上限
	thresholdEmotionMax   = 2  // 情绪命名（感到/觉得）上限
	thresholdHookMin      = 1  // 章末钩子最少一种
	thresholdDialogueBare = 30 // 裸对话标签占比上限（%）
)

// bannedWordList 零容忍词（出现即判定 AI 味或套路化）。
var bannedWordList = []string{
	"不禁", "与此同时", "然而", "内心深处", "缓缓说道", "命运的齿轮", "宿命感",
	"忍不住", "下意识地", "不由自主地", "嘴角勾起", "深邃的眼眸", "一抹弧度",
	"仿佛整个世界", "心中暗道",
}

// warnWordList 警示词（每章出现次数受限）。
var warnWordList = []string{
	"深吸一口气", "时间来到了", "另一边", "仿佛", "似乎", "不由得", "淡淡地",
	"微微", "轻轻", "顿了顿",
}

// emotionWordList 情绪命名（直接命名情绪而非外化）。
var emotionWordList = []string{"感到", "觉得", "心中一阵", "涌上心头"}

// bareTagList 裸对话标签（"他说""我说"式，占比过高即提示改为动作标签）。
var bareTagList = []string{"他说", "她说", "我说", "说道", "问道", "答道", "开口道", "回答道"}

// hookPatternList 章末钩子的可识别形态（问题/危险/误会/决定/代价）。
var hookPatternList = []string{"？", "?", "！", "!", "……", "..."}

// Issue 一条体检问题（level: block 阻断 / warn 警示 / info 建议）。
type Issue struct {
	IssueCode  string `json:"issue_code"`  // 问题编码（稳定标识，前端按码着色）
	LevelCode  string `json:"level_code"`  // 级别：block/warn/info
	TitleText  string `json:"title_text"`  // 问题标题
	DetailText string `json:"detail_text"` // 说明与改法
	HitCount   int    `json:"hit_count"`   // 命中次数
	SampleText string `json:"sample_text"` // 命中示例（截断）
}

// Report 体检报告。
type Report struct {
	WordCount        int64   `json:"word_count"`         // 字数
	ParagraphCount   int     `json:"paragraph_count"`    // 段落数
	DialogueRatio    float64 `json:"dialogue_ratio"`     // 对话行占比（0-1）
	DashCount        int     `json:"dash_count"`         // 破折号数量
	BannedWordCount  int     `json:"banned_word_count"`  // 零容忍词命中数
	WarnWordCount    int     `json:"warn_word_count"`    // 警示词命中数
	EmotionWordCount int     `json:"emotion_word_count"` // 情绪命名命中数
	BareTagCount     int     `json:"bare_tag_count"`     // 裸对话标签数
	HookPresent      bool    `json:"hook_present"`       // 章末是否有钩子
	ScoreValue       int     `json:"score_value"`        // 综合分（0-100）
	VerdictCode      string  `json:"verdict_code"`       // 结论：pass/revise/block
	IssueList        []Issue `json:"issue_list"`         // 问题清单
}

// verdict 结论编码。
const (
	VerdictPass   = "pass"   // 可发
	VerdictRevise = "revise" // 需修
	VerdictBlock  = "block"  // 不可发
)

// Analyze 对正文执行量化体检（纯函数，无外部依赖）。
func Analyze(contentText string) Report {
	text := strings.TrimSpace(contentText)
	report := Report{IssueList: make([]Issue, 0, 8)}
	if text == "" {
		report.VerdictCode = VerdictBlock
		report.IssueList = append(report.IssueList, Issue{
			IssueCode: "empty", LevelCode: "block", TitleText: "正文为空",
			DetailText: "没有可体检的正文，请先写入章节内容。",
		})
		return report
	}
	report.WordCount = countWords(text)
	paragraphList := splitParagraphs(text)
	report.ParagraphCount = len(paragraphList)
	report.DialogueRatio = dialogueRatio(paragraphList)
	report.DashCount = strings.Count(text, "——") + strings.Count(text, "—")
	bannedHits := countOccurrences(text, bannedWordList)
	warnHits := countOccurrences(text, warnWordList)
	emotionHits := countOccurrences(text, emotionWordList)
	bareTagHits := countOccurrences(text, bareTagList)
	report.BannedWordCount = totalHits(bannedHits)
	report.WarnWordCount = totalHits(warnHits)
	report.EmotionWordCount = totalHits(emotionHits)
	report.BareTagCount = totalHits(bareTagHits)
	report.HookPresent = hookPresent(text)

	appendIssue := func(code, level, title, detail string, hitCount int, sample string) {
		report.IssueList = append(report.IssueList, Issue{
			IssueCode: code, LevelCode: level, TitleText: title,
			DetailText: detail, HitCount: hitCount, SampleText: sample,
		})
	}
	if report.WordCount < 1500 {
		appendIssue("word_count", "warn", "字数偏少",
			"常规连载章建议 2000-3500 字；低于 1500 字读者会觉得这一章没发生什么。", int(report.WordCount), "")
	}
	if report.DashCount > thresholdDashMax {
		appendIssue("dash", "warn", "破折号过多",
			"解释用冒号、转折用句号、补充用分号、强调用动作打断；破折号每章控制在 12 处以内。", report.DashCount, "")
	}
	if report.BannedWordCount > thresholdBannedZero {
		appendIssue("banned_word", "block", "命中零容忍词",
			"这些词是 AI 味与套路的强信号，逐个替换成具体动作或事实："+joinTopHits(bannedHits, 6), report.BannedWordCount, "")
	}
	if report.WarnWordCount > thresholdWarnMax {
		appendIssue("warn_word", "warn", "套路词偏多",
			"警示词每章不超过 3 次，优先删掉而不是换同义词："+joinTopHits(warnHits, 6), report.WarnWordCount, "")
	}
	if report.EmotionWordCount > thresholdEmotionMax {
		appendIssue("emotion_named", "warn", "情绪被直接命名",
			"「感到/觉得」这类词每章不超过 2 次；把情绪写成动作、生理反应与旁人可见的细节："+joinTopHits(emotionHits, 4),
			report.EmotionWordCount, "")
	}
	if report.BareTagCount > 0 && report.DialogueRatio > 0 {
		bareRatio := float64(report.BareTagCount) / float64(maxInt(1, int(float64(report.ParagraphCount)*report.DialogueRatio)))
		if bareRatio > float64(thresholdDialogueBare)/100 {
			appendIssue("bare_tag", "warn", "对话标签过于单调",
				"七成以上对白应带动作或神态，让读者知道谁在说、说话时手在做什么："+joinTopHits(bareTagHits, 4),
				report.BareTagCount, "")
		}
	}
	if !report.HookPresent {
		appendIssue("hook_missing", "warn", "章末缺少钩子",
			"章末停在问题、危险、误会、决定或代价上；不要用反问、感叹或总结收尾。", 0, tailSample(text))
	}
	if hasUniformParagraphs(paragraphList) {
		appendIssue("paragraph_uniform", "info", "段落长度过于均匀",
			"长短句交替、紧张处用短段、铺陈处用长段；均匀段落读起来像机器在排比。", report.ParagraphCount, "")
	}

	report.ScoreValue = scoreOf(report)
	report.VerdictCode = verdictOf(report)
	return report
}

// countWords 统计字数：CJK 逐字，其余按空白分词（与正文编辑器口径一致）。
func countWords(text string) int64 {
	var wordCount int64
	inWord := false
	for _, runeValue := range text {
		switch {
		case unicode.Is(unicode.Han, runeValue):
			wordCount++
			inWord = false
		case unicode.IsSpace(runeValue):
			inWord = false
		default:
			if !inWord {
				wordCount++
				inWord = true
			}
		}
	}
	return wordCount
}

// splitParagraphs 按空行或换行切分非空段落。
func splitParagraphs(text string) []string {
	rawList := strings.FieldsFunc(text, func(r rune) bool { return r == '\n' || r == '\r' })
	paragraphList := make([]string, 0, len(rawList))
	for _, item := range rawList {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			paragraphList = append(paragraphList, trimmed)
		}
	}
	return paragraphList
}

// dialogueRatio 计算含引号对白的段落占比。
func dialogueRatio(paragraphList []string) float64 {
	if len(paragraphList) == 0 {
		return 0
	}
	dialogueCount := 0
	for _, paragraph := range paragraphList {
		if strings.ContainsAny(paragraph, "「」“”\"") {
			dialogueCount++
		}
	}
	return float64(dialogueCount) / float64(len(paragraphList))
}

// countOccurrences 统计词表中各词的命中次数（返回 词 → 次数）。
func countOccurrences(text string, wordList []string) map[string]int {
	hits := make(map[string]int, len(wordList))
	for _, word := range wordList {
		if count := strings.Count(text, word); count > 0 {
			hits[word] = count
		}
	}
	return hits
}

// totalHits 汇总命中总数。
func totalHits(hits map[string]int) int {
	total := 0
	for _, count := range hits {
		total += count
	}
	return total
}

// joinTopHits 取命中次数最高的若干词，拼成「词×次数」。
func joinTopHits(hits map[string]int, limit int) string {
	type pair struct {
		word  string
		count int
	}
	pairList := make([]pair, 0, len(hits))
	for word, count := range hits {
		pairList = append(pairList, pair{word: word, count: count})
	}
	sort.Slice(pairList, func(i, j int) bool {
		if pairList[i].count != pairList[j].count {
			return pairList[i].count > pairList[j].count
		}
		return pairList[i].word < pairList[j].word
	})
	if len(pairList) > limit {
		pairList = pairList[:limit]
	}
	parts := make([]string, 0, len(pairList))
	for _, item := range pairList {
		parts = append(parts, item.word+"×"+itoa(item.count))
	}
	return strings.Join(parts, "、")
}

// hookPresent 判断章末是否有钩子（末 60 字内出现悬念标点或未闭合语气）。
func hookPresent(text string) bool {
	runes := []rune(strings.TrimSpace(text))
	if len(runes) == 0 {
		return false
	}
	start := len(runes) - 60
	if start < 0 {
		start = 0
	}
	tail := string(runes[start:])
	for _, pattern := range hookPatternList {
		if strings.Contains(tail, pattern) {
			return true
		}
	}
	return false
}

// tailSample 取章末 40 字作为示例。
func tailSample(text string) string {
	runes := []rune(strings.TrimSpace(text))
	if len(runes) <= 40 {
		return string(runes)
	}
	return string(runes[len(runes)-40:])
}

// hasUniformParagraphs 判断段落长度是否过于均匀（标准差过小且段数足够）。
func hasUniformParagraphs(paragraphList []string) bool {
	if len(paragraphList) < 6 {
		return false
	}
	lengthList := make([]float64, 0, len(paragraphList))
	for _, paragraph := range paragraphList {
		lengthList = append(lengthList, float64(len([]rune(paragraph))))
	}
	mean := 0.0
	for _, length := range lengthList {
		mean += length
	}
	mean /= float64(len(lengthList))
	if mean <= 0 {
		return false
	}
	variance := 0.0
	for _, length := range lengthList {
		delta := length - mean
		variance += delta * delta
	}
	variance /= float64(len(lengthList))
	deviation := variance / (mean * mean)
	return deviation < 0.08
}

// scoreOf 按问题级别扣分（block 每个 25，warn 每个 8，info 每个 2）。
func scoreOf(report Report) int {
	score := 100
	for _, issue := range report.IssueList {
		switch issue.LevelCode {
		case "block":
			score -= 25
		case "warn":
			score -= 8
		default:
			score -= 2
		}
	}
	if score < 0 {
		score = 0
	}
	return score
}

// verdictOf 由问题级别推导结论。
func verdictOf(report Report) string {
	hasBlock, warnCount := false, 0
	for _, issue := range report.IssueList {
		switch issue.LevelCode {
		case "block":
			hasBlock = true
		case "warn":
			warnCount++
		}
	}
	if hasBlock {
		return VerdictBlock
	}
	if warnCount >= 3 {
		return VerdictRevise
	}
	if warnCount > 0 {
		return VerdictRevise
	}
	return VerdictPass
}

// itoa 整数转字符串（避免为此引入 strconv 之外的分支）。
func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	digits := make([]byte, 0, 8)
	for value > 0 {
		digits = append([]byte{byte('0' + value%10)}, digits...)
		value /= 10
	}
	return string(digits)
}

// maxInt 取较大值。
func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}
