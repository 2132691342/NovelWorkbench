// 章节体检链路的回归：同一正文的门禁结论必须稳定，且问题分级与综合分可复现。
// 这条用例覆盖「词面指标 → 分级 → 结论」三步，任何一步漂移都会让写作台与对话期工具给出不一致的判断。

package check

import (
	"strings"
	"testing"
)

// TestChapterCheckChain 覆盖体检的完整判定链。
func TestChapterCheckChain(t *testing.T) {
	// 步骤1：空正文直接判不可发，且只有一个阻断问题。
	emptyReport := Analyze("   ")
	if emptyReport.VerdictCode != VerdictBlock || len(emptyReport.IssueList) != 1 {
		t.Fatalf("空正文应判 block 且只有一条问题，实际 verdict=%s issues=%d",
			emptyReport.VerdictCode, len(emptyReport.IssueList))
	}

	// 步骤2：命中零容忍词 + 缺钩子 + 字数不足 → 不可发，且命中次数与示例可核对。
	dirty := "他深吸一口气，不禁握紧了拳头。命运的齿轮开始转动，他内心深处仿佛有火在烧。"
	dirtyReport := Analyze(dirty)
	if dirtyReport.VerdictCode != VerdictBlock {
		t.Fatalf("含零容忍词应判 block，实际 %s", dirtyReport.VerdictCode)
	}
	if dirtyReport.BannedWordCount != 3 {
		t.Fatalf("零容忍词命中数 = %d，want 3", dirtyReport.BannedWordCount)
	}
	if dirtyReport.HookPresent {
		t.Fatal("该正文没有章末钩子")
	}
	if !hasIssue(dirtyReport, "banned_word", "block") {
		t.Fatal("缺少 banned_word 阻断项")
	}
	if !hasIssue(dirtyReport, "hook_missing", "warn") {
		t.Fatal("缺少 hook_missing 警示项")
	}
	if dirtyReport.ScoreValue >= 100 {
		t.Fatalf("有问题时综合分必须扣分，实际 %d", dirtyReport.ScoreValue)
	}

	// 步骤3：干净正文（有钩子、无套路词、字数足够）判可发或至多建议级。
	clean := strings.Repeat("雨停之后他推开邮站的门，铜锈味顺着台阶往上爬。", 60) +
		"\n\n他摸到信封右下角那道裂口时，手指停住了——这道裂口是谁划的？"
	cleanReport := Analyze(clean)
	if cleanReport.BannedWordCount != 0 {
		t.Fatalf("干净正文不应命中零容忍词，实际 %d", cleanReport.BannedWordCount)
	}
	if cleanReport.VerdictCode == VerdictBlock {
		t.Fatalf("干净正文不应判 block，问题：%+v", cleanReport.IssueList)
	}
	if !cleanReport.HookPresent {
		t.Fatal("末句是悬念问句，应识别为有钩子")
	}

	// 步骤4：确定性。同一输入两次体检必须得到完全相同的分数与结论。
	again := Analyze(dirty)
	if again.ScoreValue != dirtyReport.ScoreValue || again.VerdictCode != dirtyReport.VerdictCode {
		t.Fatalf("同一正文两次体检结果不一致：%d/%s vs %d/%s",
			dirtyReport.ScoreValue, dirtyReport.VerdictCode, again.ScoreValue, again.VerdictCode)
	}
}

// hasIssue 判断报告中是否存在指定编码与级别的问题。
func hasIssue(report Report, issueCode, levelCode string) bool {
	for _, issue := range report.IssueList {
		if issue.IssueCode == issueCode && issue.LevelCode == levelCode {
			return true
		}
	}
	return false
}
