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

// TestSlashParsingChain 覆盖斜杠命令解析链路：首 token 命中 → 命令与参数分离 → 占位符填充；
// 未命中（自由正文 / 未知命令 / 禁用命令 / 命令不在首 token）时正文原样保留。
// 命令识别错误会让用户输入被误当作技能执行，因此命中与未命中两条路径必须一起守住。
func TestSlashParsingChain(t *testing.T) {
	commands := testSlashCommands()

	// ---- 命中：命令 token 与参数分离，占位符同步填充 ----
	result := parseSlashContent("/continue 以林澈视角", commands)
	if result.SkillCode != "continue" {
		t.Fatalf("SkillCode = %q, want continue", result.SkillCode)
	}
	if result.RemainderText != "以林澈视角" {
		t.Fatalf("RemainderText = %q, want 以林澈视角", result.RemainderText)
	}
	if result.PlaceholderMap["user_input"] != "以林澈视角" {
		t.Fatalf("placeholder user_input = %q, want 以林澈视角", result.PlaceholderMap["user_input"])
	}

	// ---- 命中但无参数：占位符为空串 ----
	result = parseSlashContent("/outline", commands)
	if result.SkillCode != "outline" {
		t.Fatalf("SkillCode = %q, want outline", result.SkillCode)
	}
	if result.RemainderText != "" {
		t.Fatalf("RemainderText = %q, want empty", result.RemainderText)
	}
	if value, ok := result.PlaceholderMap["user_input"]; !ok || value != "" {
		t.Fatalf("placeholder user_input = %q (present=%v), want empty", value, ok)
	}

	// ---- 参数剥离：多空白/制表/换行分隔与首尾空白归一 ----
	result = parseSlashContent("  /polish \t 加强画面感  \n", commands)
	if result.SkillCode != "polish" || result.RemainderText != "加强画面感" {
		t.Fatalf("空白归一失败: %+v", result)
	}
	result = parseSlashContent("/review  逐段检查\n逻辑", commands)
	if result.RemainderText != "逐段检查\n逻辑" {
		t.Fatalf("参数内部换行应保留: %q", result.RemainderText)
	}

	// ---- 未命中：正文原样保留，不产出技能码与占位符 ----
	for _, contentText := range []string{
		"普通的正文开头，不该命中",
		"/unknown 一些参数",
		"/disabled_cmd 参数",
		"续写 /continue",
	} {
		result = parseSlashContent(contentText, commands)
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
