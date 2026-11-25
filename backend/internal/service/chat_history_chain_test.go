// 历史链路的回归：会话历史注入（条数不截取、单条超长首尾收敛）→ 历史归档（逐条编号、角色与工具标签）。
// 两条链路都在「把作者说过的话完整交给模型」这一目标上，任一环节出错都表现为模型「忘记」前文，
// 因此合并在本文件按链路组织。

package service

import (
	"strconv"
	"strings"
	"testing"

	"github.com/2132691342/NovelWorkbench/backend/internal/ai"
	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
)

// TestHistoryChain 覆盖历史注入与历史归档的完整链路。
func TestHistoryChain(t *testing.T) {
	// 环节1：历史注入。条数一条不删，短正文原样返回。
	messages := []domain.ChatMessageDO{
		chatMessageOf("MSG_1", domain.ChatRoleUser, "一"),
		chatMessageOf("MSG_2", domain.ChatRoleAssistant, "二"),
		chatMessageOf("MSG_3", domain.ChatRoleUser, "三"),
		chatMessageOf("MSG_4", domain.ChatRoleAssistant, "四"),
		chatMessageOf("MSG_5", domain.ChatRoleUser, "五"),
	}
	injected, condensedCount := condenseHistory(messages)
	if len(injected) != len(messages) || injected[0].MessageULID != "MSG_1" ||
		injected[4].MessageULID != "MSG_5" {
		t.Fatalf("历史应全量旧到新注入: %v", injected)
	}
	if condensedCount != 0 || injected[0].ContentText != "一" {
		t.Fatalf("短正文不应被改写: %+v", injected[0])
	}

	// 环节2：单条超长只保留首尾，条数仍然一条不删，并标注省略字数。
	longList := []domain.ChatMessageDO{
		chatMessageOf("MSG_L1", domain.ChatRoleUser, strings.Repeat("甲", 5000)),
		chatMessageOf("MSG_L2", domain.ChatRoleAssistant, "短回复"),
	}
	injected, condensedCount = condenseHistory(longList)
	if len(injected) != 2 || condensedCount != 1 {
		t.Fatalf("只应收敛超长的那一条: %d, %d", len(injected), condensedCount)
	}
	if !strings.HasPrefix(injected[0].ContentText, "甲甲") ||
		!strings.Contains(injected[0].ContentText, "此处省略") ||
		!strings.HasSuffix(injected[0].ContentText, "甲甲") {
		t.Fatalf("超长正文应保留首尾并标注省略: %q", injected[0].ContentText)
	}
	if longList[0].ContentText != strings.Repeat("甲", 5000) {
		t.Fatal("收敛不得改写原消息")
	}

	// 环节3：归档全量。条数一条不删，逐条带序号与角色标签。
	archiveList := make([]ai.ArchiveMessage, 0, 6)
	for i := range 6 {
		roleCode := "user"
		if i%2 == 1 {
			roleCode = "assistant"
		}
		archiveList = append(archiveList, ai.ArchiveMessage{
			Seq: i + 1, RoleCode: roleCode, ContentText: "第" + strconv.Itoa(i+1) + "条内容",
			ToolCodeCSV: "chapter_check", CreatedAt: "2026-11-25T10:00:00+08:00",
		})
	}
	archiveText := ai.ArchiveHistory(archiveList, ai.ArchiveConfig{WithHeader: true})
	if !strings.Contains(archiveText, "共 6 条") {
		t.Fatalf("归档应标明总条数: %q", archiveText)
	}
	for i := range 6 {
		marker := strconv.Itoa(i+1) + ". "
		if !strings.Contains(archiveText, marker) {
			t.Fatalf("归档应逐条编号，缺少 %q: %q", marker, archiveText)
		}
	}
	if !strings.Contains(archiveText, "作者") || !strings.Contains(archiveText, "AI") {
		t.Fatalf("归档应带角色标签: %q", archiveText)
	}
	if !strings.Contains(archiveText, "［工具：chapter_check］") {
		t.Fatalf("归档应保留工具标签: %q", archiveText)
	}
	if !strings.Contains(archiveText, "2026-11-25T10:00:00+08:00") {
		t.Fatalf("归档应保留时间: %q", archiveText)
	}

	// 环节4：单条超长只收敛那一条，其余条数与内容不变。
	mixedList := []ai.ArchiveMessage{
		{Seq: 1, RoleCode: "user", ContentText: strings.Repeat("乙", 6000)},
		{Seq: 2, RoleCode: "assistant", ContentText: "短回答"},
	}
	mixedText := ai.ArchiveHistory(mixedList, ai.ArchiveConfig{})
	if strings.Count(mixedText, "1. 作者") != 1 || strings.Count(mixedText, "2. AI") != 1 {
		t.Fatalf("归档不得丢条: %q", mixedText)
	}
	if !strings.Contains(mixedText, "短回答") || !strings.Contains(mixedText, "此处省略") {
		t.Fatalf("应保留短正文并收敛超长正文: %q", mixedText)
	}
	if ai.ArchiveHistory(nil, ai.ArchiveConfig{}) != "" {
		t.Fatal("空历史应返回空串")
	}
}
