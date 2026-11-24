// 本文件实现会话历史的本地归档：不调用模型、不滑动窗口、不按 token 预算裁剪。
// 两条纪律：注入历史时一条不丢；单条正文过长时保留首尾并标注省略字数（压缩率刻意做低）。
// 归档文本用于 @ 引用历史会话——把整段对话导出成逐条编号的记录。

package ai

import (
	"fmt"
	"strings"
)

// 单条正文的收敛参数：合计 4000 字以内原样保留，超出的保留首尾。
const (
	condenseHeadRunes = 2400 // 超长正文保留的开头字符数
	condenseTailRunes = 1600 // 超长正文保留的结尾字符数
)

// CondenseText 收敛单条超长正文：未超长原样返回，超长保留首尾并标注省略字数。
// 保留量刻意做大（合计 4000 字），宁可上下文长一些，也不把成段正文压成摘要句。
func CondenseText(contentText string) string {
	return CondenseRunes(contentText, condenseHeadRunes, condenseTailRunes)
}

// CondenseRunes 按指定首尾字符数收敛正文（首尾都放不下时才收敛）。
func CondenseRunes(contentText string, headRunes, tailRunes int) string {
	if headRunes <= 0 || tailRunes <= 0 {
		return contentText
	}
	runes := []rune(contentText)
	if len(runes) <= headRunes+tailRunes {
		return contentText
	}
	omitted := len(runes) - headRunes - tailRunes
	return string(runes[:headRunes]) +
		fmt.Sprintf("\n……（此处省略 %d 字）……\n", omitted) +
		string(runes[len(runes)-tailRunes:])
}

// ArchiveMessage 一条待归档的历史消息。
type ArchiveMessage struct {
	Seq         int    // 会话内序号（1 起）
	RoleCode    string // 角色：user/assistant/system
	ContentText string // 消息正文
	ToolCodeCSV string // 本轮执行的工具编码，逗号分隔（可空）
	RefNameCSV  string // 本轮引用的素材名，逗号分隔（可空）
	CreatedAt   string // 创建时间（RFC3339，可空）
}

// ArchiveConfig 归档参数（零值取默认）。
type ArchiveConfig struct {
	HeadRunes  int  // 单条正文保留的开头字符数
	TailRunes  int  // 单条正文保留的结尾字符数
	WithHeader bool // 是否输出标题行
}

// ArchiveHistory 把整段会话导出成逐条编号的归档文本。
// 条数一条不删；只有单条正文超长时保留首尾，并在记录里直接标注省略了多少字。
// 同一输入必然得到同一输出，O(n) 单趟完成。
func ArchiveHistory(messages []ArchiveMessage, cfg ArchiveConfig) string {
	if cfg.HeadRunes <= 0 {
		cfg.HeadRunes = condenseHeadRunes
	}
	if cfg.TailRunes <= 0 {
		cfg.TailRunes = condenseTailRunes
	}
	if len(messages) == 0 {
		return ""
	}
	var builder strings.Builder
	if cfg.WithHeader {
		builder.WriteString(fmt.Sprintf("【历史会话归档】共 %d 条，按时间顺序完整保留\n", len(messages)))
	}
	for i := range messages {
		builder.WriteString(archiveRecord(messages[i], i+1, cfg))
		builder.WriteString("\n")
	}
	return strings.TrimSpace(builder.String())
}

// archiveRecord 把一条消息整理成「序号. 角色（时间）［标签］\n正文」。
func archiveRecord(message ArchiveMessage, fallbackSeq int, cfg ArchiveConfig) string {
	seq := message.Seq
	if seq <= 0 {
		seq = fallbackSeq
	}
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("%d. %s", seq, archiveRoleLabel(message.RoleCode)))
	if message.CreatedAt != "" {
		builder.WriteString("（" + message.CreatedAt + "）")
	}
	if message.ToolCodeCSV != "" {
		builder.WriteString("［工具：" + message.ToolCodeCSV + "］")
	}
	if message.RefNameCSV != "" {
		builder.WriteString("［引用：" + message.RefNameCSV + "］")
	}
	builder.WriteString("\n")
	builder.WriteString(CondenseRunes(message.ContentText, cfg.HeadRunes, cfg.TailRunes))
	return builder.String()
}

// archiveRoleLabel 归档文本里的角色标签（给作者看的，不是模型角色）。
func archiveRoleLabel(roleCode string) string {
	switch roleCode {
	case "assistant":
		return "AI"
	case "system":
		return "系统"
	default:
		return "作者"
	}
}
