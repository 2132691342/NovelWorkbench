// 章节体检的场景回归：词面指标 → 问题分级 → 综合分与结论。
// 写作台与对话期工具共用同一套门禁，任何一步漂移都会让两处给出不一致的判断。

package check

import (
	"strings"
	"testing"
)

// TestChapterCheckChain 覆盖体检的完整判定链。
func TestChapterCheckChain(t *testing.T) {
	// 步骤1：空正文直接判不可发。
	if report := Analyze("   "); report.VerdictCode != VerdictBlock || len(report.IssueList) != 1 {
		t.Fatalf("空正文应判 block 且只有一条问题，实际 verdict=%s issues=%d",
			report.VerdictCode, len(report.IssueList))
	}

	// 步骤2：命中零容忍词且缺钩子的正文判不可发，问题按级别分级登记，综合分扣分。
	dirty := "他深吸一口气，不禁握紧了拳头。命运的齿轮开始转动，他内心深处仿佛有火在烧。"
	dirtyReport := Analyze(dirty)
	if dirtyReport.VerdictCode != VerdictBlock || dirtyReport.BannedWordCount != 3 {
		t.Fatalf("含零容忍词应判 block，命中数 = %d", dirtyReport.BannedWordCount)
	}
	if dirtyReport.HookPresent {
		t.Fatal("该正文没有章末钩子")
	}
	if !hasIssue(dirtyReport, "banned_word", "block") || !hasIssue(dirtyReport, "hook_missing", "warn") {
		t.Fatalf("问题分级登记错误: %+v", dirtyReport.IssueList)
	}
	if dirtyReport.ScoreValue >= 100 {
		t.Fatalf("有问题时综合分必须扣分，实际 %d", dirtyReport.ScoreValue)
	}

	// 步骤3：干净正文（有钩子、无套路词）不判不可发，末句悬念问句识别为有钩子。
	clean := strings.Repeat("雨停之后他推开邮站的门，铜锈味顺着台阶往上爬。", 60) +
		"\n\n他摸到信封右下角那道裂口时，手指停住了——这道裂口是谁划的？"
	cleanReport := Analyze(clean)
	if cleanReport.BannedWordCount != 0 || cleanReport.VerdictCode == VerdictBlock || !cleanReport.HookPresent {
		t.Fatalf("干净正文判定错误: verdict=%s banned=%d hook=%v",
			cleanReport.VerdictCode, cleanReport.BannedWordCount, cleanReport.HookPresent)
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
