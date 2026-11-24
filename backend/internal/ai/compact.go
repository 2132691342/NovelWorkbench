// 本文件实现会话历史的本地压缩：不调用模型、不做向量检索，按确定性规则把旧消息
// 折叠成一行一条的摘要，只保留最近若干条原文。
// 用途有三处：对话注入历史前的预算收缩、@ 引用历史会话时的整段压缩、/compact 动作的持久压缩点。

package ai

import (
	"strings"
	"unicode/utf8"
)

// 压缩默认参数。
const (
	compactKeepRecentCount = 8    // 原样保留的最近消息条数
	compactPerMessageRunes = 60   // 被折叠消息每条保留的字符数
	compactBudgetChars     = 3000 // 压缩后摘要的字符预算
	compactMinTotalChars   = 6000 // 历史总字符低于该值不压缩：压了反而丢失细节
)

// CompactMessage 一条待压缩的历史消息。
type CompactMessage struct {
	RoleCode    string // 角色：user/assistant/system
	ContentText string // 消息正文
}

// CompactConfig 压缩参数（零值字段取默认值）。
type CompactConfig struct {
	KeepRecentCount int  // 原样保留的最近条数
	PerMessageRunes int  // 每条被折叠消息保留的字符数
	BudgetChars     int  // 摘要字符预算
	MinTotalChars   int  // 触发压缩的总字符门槛
	FoldAll         bool // 全部折叠（@ 引用整段历史会话时用），忽略 KeepRecentCount
}

// CompactResult 压缩结果。
type CompactResult struct {
	DigestText   string           // 折叠出的摘要（未压缩时为空）
	KeptList     []CompactMessage // 原样保留的最近消息（旧→新）
	FoldedCount  int              // 被折叠进摘要的消息条数
	DroppedCount int              // 因摘要超预算而被丢弃的折叠条数
	Compacted    bool             // 是否实际发生压缩
}

// DefaultCompactConfig 返回默认压缩参数。
func DefaultCompactConfig() CompactConfig {
	return CompactConfig{
		KeepRecentCount: compactKeepRecentCount,
		PerMessageRunes: compactPerMessageRunes,
		BudgetChars:     compactBudgetChars,
		MinTotalChars:   compactMinTotalChars,
	}
}

// CompactHistory 本地压缩会话历史。
// 规则：总字符未过门槛时原样保留；否则最近 KeepRecentCount 条保留原文，
// 更早的消息逐条折叠为「角色: 首段摘要」，摘要超出预算时从最旧一条开始丢弃。
// 全程无模型调用，O(n) 单趟完成，同一输入必然得到同一输出。
func CompactHistory(messages []CompactMessage, cfg CompactConfig) CompactResult {
	if cfg.KeepRecentCount <= 0 {
		cfg.KeepRecentCount = compactKeepRecentCount
	}
	if cfg.PerMessageRunes <= 0 {
		cfg.PerMessageRunes = compactPerMessageRunes
	}
	if cfg.BudgetChars <= 0 {
		cfg.BudgetChars = compactBudgetChars
	}
	if cfg.MinTotalChars <= 0 {
		cfg.MinTotalChars = compactMinTotalChars
	}
	total := 0
	for i := range messages {
		total += utf8.RuneCountInString(messages[i].ContentText)
	}
	if total <= cfg.MinTotalChars {
		return CompactResult{KeptList: messages}
	}
	foldEnd := len(messages) - cfg.KeepRecentCount
	if cfg.FoldAll {
		foldEnd = len(messages)
	}
	if foldEnd <= 0 {
		return CompactResult{KeptList: messages}
	}
	lineList := make([]string, 0, foldEnd)
	for i := range foldEnd {
		lineList = append(lineList, compactLine(messages[i], cfg.PerMessageRunes))
	}
	used := 0
	keepFrom := 0
	for i := len(lineList) - 1; i >= 0; i-- {
		lineRunes := utf8.RuneCountInString(lineList[i]) + 1
		if used+lineRunes > cfg.BudgetChars {
			keepFrom = i + 1
			break
		}
		used += lineRunes
	}
	dropped := keepFrom
	keptLines := lineList[keepFrom:]
	var builder strings.Builder
	builder.WriteString("【历史摘要（本地压缩）】\n")
	for _, line := range keptLines {
		builder.WriteString(line)
		builder.WriteString("\n")
	}
	return CompactResult{
		DigestText:   strings.TrimSpace(builder.String()),
		KeptList:     messages[foldEnd:],
		FoldedCount:  len(keptLines),
		DroppedCount: dropped,
		Compacted:    true,
	}
}

// compactLine 把一条消息折叠成单行摘要：角色标签 + 去空白后的首段内容。
func compactLine(message CompactMessage, maxRunes int) string {
	label := "用户"
	switch message.RoleCode {
	case "assistant":
		label = "AI"
	case "system":
		label = "系统"
	}
	return label + ": " + compactText(message.ContentText, maxRunes)
}

// compactText 折叠空白并按字符截断（保留完整 UTF-8 边界）。
func compactText(contentText string, maxRunes int) string {
	collapsed := strings.Join(strings.Fields(contentText), " ")
	runes := []rune(collapsed)
	if len(runes) <= maxRunes {
		return string(runes)
	}
	return string(runes[:maxRunes]) + "…"
}

// JoinDigest 合并两段历史摘要（旧的在前），空串自动忽略。
// 会话已有持久压缩点时，本轮新折叠的摘要接在其后，避免新摘要覆盖旧结论。
func JoinDigest(baseDigest, appendDigest string) string {
	base := strings.TrimSpace(baseDigest)
	appended := strings.TrimSpace(appendDigest)
	switch {
	case base == "":
		return appended
	case appended == "":
		return base
	default:
		return base + "\n" + appended
	}
}
