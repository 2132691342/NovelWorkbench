// 工具触发链路的场景回归：#code 解析 → 编码合并 → 显式选取与缺失 → 关键词补位。
// 这四步决定「用户点名的工具是否真的跑了」，任一环节偏移都会让体检静默不执行。

package service

import (
	"testing"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
)

// toolDO 构造测试用工具行。
func toolDO(code string, enabled bool, keywords string) domain.ToolDO {
	return domain.ToolDO{
		ToolULID: "TOL_" + code, ToolCode: code, ToolName: code + "工具",
		TriggerKeywords: keywords, EnabledFlag: enabled, BuiltinFlag: true,
	}
}

// TestToolTriggerChain 覆盖一次「正文带 #code 的对话」里工具从解析到执行的选取链路。
func TestToolTriggerChain(t *testing.T) {
	// 环节1：正文解析。行首/空白后的 #code 命中，正文中间粘连、大写、含连字符的不命中；
	// 触发词被剥离且不破坏换行与缩进。
	cases := []struct {
		caseName    string
		content     string
		wantCodes   int
		wantRemain  string
		wantFirstOf string
	}{
		{"行首触发", "#chapter_check 看看这章", 1, "看看这章", "chapter_check"},
		{"行中触发", "帮我 #deai_scan 扫一下", 1, "帮我 扫一下", "deai_scan"},
		{"多个触发", "#chapter_check #deai_scan 检查", 2, "检查", "chapter_check"},
		{"保留换行", "#chapter_check\n第一段\n第二段", 1, "第一段\n第二段", "chapter_check"},
		{"词内井号不命中", "C# 是语言", 0, "C# 是语言", ""},
		{"大写不命中", "#Chapter_Check 检查", 0, "#Chapter_Check 检查", ""},
		{"无井号", "普通正文", 0, "普通正文", ""},
	}
	for _, testCase := range cases {
		codeList, remainText := parseHashToolCodes(testCase.content)
		if len(codeList) != testCase.wantCodes {
			t.Fatalf("%s: 命中数 = %d, want %d (%v)", testCase.caseName, len(codeList), testCase.wantCodes, codeList)
		}
		if testCase.wantCodes > 0 && codeList[0] != testCase.wantFirstOf {
			t.Fatalf("%s: 首个编码 = %q, want %q", testCase.caseName, codeList[0], testCase.wantFirstOf)
		}
		if remainText != testCase.wantRemain {
			t.Fatalf("%s: 剩余正文 = %q, want %q", testCase.caseName, remainText, testCase.wantRemain)
		}
	}

	// 环节2：编码合并。请求体显式编码在前、内联编码在后，去重保序并截断到上限。
	merged := mergeToolCodes([]string{"chapter_check", "chapter_check"}, []string{"deai_scan", "chapter_check"})
	if len(merged) != 2 || merged[0] != "chapter_check" || merged[1] != "deai_scan" {
		t.Fatalf("合并结果错误: %v", merged)
	}
	overflow := mergeToolCodes([]string{"a", "b", "c", "d"}, nil)
	if len(overflow) != toolExplicitLimit {
		t.Fatalf("超出上限未截断: %v", overflow)
	}

	// 环节3：显式选取。编码不存在或已停用记入 missing，不能静默跳过。
	fixtures := []domain.ToolDO{
		toolDO("chapter_check", true, "体检,检查这章"),
		toolDO("deai_scan", true, "去ai味"),
		toolDO("lore_lookup", false, "设定"),
	}
	voList, missingList := selectToolsByCode(fixtures, []string{"chapter_check", "lore_lookup", "not_exist"})
	if len(voList) != 1 || voList[0].ToolCode != "chapter_check" {
		t.Fatalf("显式选取错误: %+v", voList)
	}
	if len(missingList) != 2 {
		t.Fatalf("缺失列表错误: %v", missingList)
	}

	// 环节4：关键词补位。已显式选中的不重复补，停用的不参与，补到余量为止。
	excludeSet := map[string]bool{"chapter_check": true}
	autoList := matchToolsByKeyword(fixtures, "这章有没有去ai味", excludeSet, 2)
	if len(autoList) != 1 || autoList[0].ToolCode != "deai_scan" {
		t.Fatalf("关键词补位错误: %+v", autoList)
	}
	if got := matchToolsByKeyword(fixtures, "设定是什么", excludeSet, 2); len(got) != 0 {
		t.Fatalf("停用工具不应自动触发: %+v", got)
	}
	if got := matchToolsByKeyword(fixtures, "这章有没有去ai味", excludeSet, 0); len(got) != 0 {
		t.Fatalf("余量为 0 时不应补位: %+v", got)
	}
}
