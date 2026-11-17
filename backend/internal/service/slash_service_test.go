package service

import (
	"testing"

	"github.com/2132691342/NovelWorkbench/backend/internal/ai"
	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
)

// testSlashCommands 基于内置技能构造测试命令表（含一条禁用命令）。
func testSlashCommands() []domain.SlashCommandDO {
	commands := ai.BuiltinSkills()
	for i := range commands {
		commands[i].EnabledFlag = true
	}
	commands = append(commands, domain.SlashCommandDO{
		CommandCode: "disabled_cmd",
		CommandName: "禁用命令",
		EnabledFlag: false,
	})
	return commands
}

// TestParseSlashContentHit 斜杠命令命中：剥离命令 token，剩余正文入 remainder 与占位符。
func TestParseSlashContentHit(t *testing.T) {
	result := parseSlashContent("/continue 以林澈视角", testSlashCommands())
	if result.SkillCode != "continue" {
		t.Fatalf("SkillCode = %q, want continue", result.SkillCode)
	}
	if result.RemainderText != "以林澈视角" {
		t.Fatalf("RemainderText = %q, want 以林澈视角", result.RemainderText)
	}
	if result.PlaceholderMap["user_input"] != "以林澈视角" {
		t.Fatalf("placeholder user_input = %q, want 以林澈视角", result.PlaceholderMap["user_input"])
	}
}

// TestParseSlashContentHitNoArgs 命中但无参数：remainder 与占位符为空串。
func TestParseSlashContentHitNoArgs(t *testing.T) {
	result := parseSlashContent("/outline", testSlashCommands())
	if result.SkillCode != "outline" {
		t.Fatalf("SkillCode = %q, want outline", result.SkillCode)
	}
	if result.RemainderText != "" {
		t.Fatalf("RemainderText = %q, want empty", result.RemainderText)
	}
	if value, ok := result.PlaceholderMap["user_input"]; !ok || value != "" {
		t.Fatalf("placeholder user_input = %q (present=%v), want empty", value, ok)
	}
}

// TestParseSlashContentParamStrip 参数剥离：多空白/制表/换行分隔与首尾空白归一。
func TestParseSlashContentParamStrip(t *testing.T) {
	result := parseSlashContent("  /polish \t 加强画面感  \n", testSlashCommands())
	if result.SkillCode != "polish" {
		t.Fatalf("SkillCode = %q, want polish", result.SkillCode)
	}
	if result.RemainderText != "加强画面感" {
		t.Fatalf("RemainderText = %q, want 加强画面感", result.RemainderText)
	}
	result = parseSlashContent("/review  逐段检查\n逻辑", testSlashCommands())
	if result.RemainderText != "逐段检查\n逻辑" {
		t.Fatalf("RemainderText = %q, want 逐段检查\\n逻辑", result.RemainderText)
	}
}

// TestParseSlashContentMiss 未命中：未知命令/自由正文/禁用命令均返回空 skill_code，正文原样保留。
func TestParseSlashContentMiss(t *testing.T) {
	cases := []string{
		"普通的正文开头，不该命中",
		"/unknown 一些参数",
		"/disabled_cmd 参数",
		"续写 /continue",
	}
	for _, contentText := range cases {
		result := parseSlashContent(contentText, testSlashCommands())
		if result.SkillCode != "" {
			t.Fatalf("content %q: SkillCode = %q, want empty", contentText, result.SkillCode)
		}
		if result.RemainderText != contentText {
			t.Fatalf("content %q: RemainderText = %q, want original", contentText, result.RemainderText)
		}
		if len(result.PlaceholderMap) != 0 {
			t.Fatalf("content %q: placeholder_map not empty: %v", contentText, result.PlaceholderMap)
		}
	}
}
