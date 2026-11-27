// 出厂提示词资产与开书方案解析的回归：资产形状与编码契约 → 任务块结构 → 人格底线 → 方案解析。
// 这几件事决定「开箱即用」是否成立：资产形状错会让播种出的提示词无法触发，
// 任务块缺段落会让模型自行发挥格式，人格底线缺失会让精简后的任务提示词失去约束，
// 方案解析错会让整份开书结果作废。

package ai

import (
	"strings"
	"testing"
)

// builtinFlowCodeList 期望的内置流程编码（顺序即创作作业顺序）。
var builtinFlowCodeList = []string{"plan", "draft", "revise", "check"}

// builtinCommandCodeMap 期望的内置任务编码：流程编码 → 该流程的两条任务编码。
var builtinCommandCodeMap = map[string][]string{
	"plan":   {"outline", "chapter_plan"},
	"draft":  {"chapter", "continue"},
	"revise": {"polish", "deai"},
	"check":  {"review", "summary"},
}

// TestBuiltinAssetChain 覆盖出厂提示词从定义到可播种的完整链路。
func TestBuiltinAssetChain(t *testing.T) {
	flowList := BuiltinDomains()
	commandList := BuiltinCommandPrompts()

	// 环节1：形状。恰好 4 个流程，每个流程恰好 2 条任务，共 8 条，编码唯一。
	if len(flowList) != len(builtinFlowCodeList) {
		t.Fatalf("内置流程应为 %d 个，实际 %d", len(builtinFlowCodeList), len(flowList))
	}
	if len(commandList) != 8 {
		t.Fatalf("内置任务提示词应为 8 条，实际 %d", len(commandList))
	}
	codeSet := map[string]bool{}
	for _, flow := range flowList {
		if flow.FlowCode == "" || flow.FlowName == "" || flow.SortOrder <= 0 {
			t.Fatalf("流程形状不合法：%+v", flow)
		}
		if len(flow.CommandList) != 2 {
			t.Fatalf("流程 %s 应恰好 2 条任务，实际 %d", flow.FlowCode, len(flow.CommandList))
		}
		for _, command := range flow.CommandList {
			if command.PromptCode == "" || command.PromptName == "" ||
				command.DescriptionText == "" || command.CategoryCode == "" {
				t.Fatalf("提示词字段缺失：%+v", command)
			}
			if codeSet[command.PromptCode] {
				t.Fatalf("提示词编码重复：%s", command.PromptCode)
			}
			codeSet[command.PromptCode] = true
		}
	}

	// 环节2：编码契约。流程与任务编码都是硬契约，改名会让前端入口与工作流步骤失效。
	for i, flowCode := range builtinFlowCodeList {
		if flowList[i].FlowCode != flowCode {
			t.Fatalf("第 %d 个流程应为 %s，实际 %s", i, flowCode, flowList[i].FlowCode)
		}
		actual := map[string]bool{}
		for _, command := range flowList[i].CommandList {
			actual[command.PromptCode] = true
		}
		for _, commandCode := range builtinCommandCodeMap[flowCode] {
			if !actual[commandCode] {
				t.Fatalf("流程 %s 缺少任务 %s", flowCode, commandCode)
			}
		}
	}
	// 下线集合与现行清单必须同源，两者不一致会让清理逻辑误删正在使用的提示词。
	for code := range BuiltinCommandCodeSet() {
		if !codeSet[code] {
			t.Fatalf("下线集合含未知编码：%s", code)
		}
	}

	// 环节3：任务块结构。提示词只写「这一单活怎么干」，通用文风规则在人格里。
	// 每份必须声明输入与输出格式，缺失就会让模型自行发挥格式。
	for _, command := range commandList {
		for _, section := range []string{"【输入】", "【输出格式】"} {
			if !strings.Contains(command.PromptText, section) {
				t.Fatalf("提示词 %s 缺少段落 %s", command.PromptCode, section)
			}
		}
		// 三到九百字：够说清一件事，又不至于让作者翻十屏才知道这条干什么。
		if runes := []rune(command.PromptText); len(runes) < 300 || len(runes) > 900 {
			t.Fatalf("提示词 %s 长度 %d 字，应在 300-900 之间", command.PromptCode, len(runes))
		}
	}

	// 环节4：占位符可渲染。未登记占位符会被渲染为空串，等于静默丢上下文。
	for _, command := range commandList {
		if unknown := TemplatePlaceholders(command.PromptText, BuiltinPlaceholders); len(unknown) > 0 {
			t.Fatalf("提示词 %s 使用了未登记的占位符：%v", command.PromptCode, unknown)
		}
	}

	// 环节5：作者人格兜底。文风与结构规则全部收敛在这里，因此它必须够长够全。
	if strings.TrimSpace(BaseSystemPrompt) == "" {
		t.Fatal("BaseSystemPrompt 不能为空")
	}
	if runes := []rune(BaseSystemPrompt); len(runes) < 600 || len(runes) > 1400 {
		t.Fatalf("BaseSystemPrompt 长度 %d 字，应在 600-1400 之间", len(runes))
	}
	for _, ruleText := range []string{"不写定性", "对称排比", "万能词", "章末", "能力边界"} {
		if !strings.Contains(BaseSystemPrompt, ruleText) {
			t.Fatalf("BaseSystemPrompt 缺少关键规则：%s", ruleText)
		}
	}
	if unknown := TemplatePlaceholders(BaseSystemPrompt, BuiltinPlaceholders); len(unknown) > 0 {
		t.Fatalf("BaseSystemPrompt 使用了未登记的占位符：%v", unknown)
	}
	if !strings.Contains(ComposeSystemPrompt("", ""), BaseSystemPrompt) {
		t.Fatal("无身份提示词时应回退到 BaseSystemPrompt")
	}
}

// TestOpenBookPlanChain 覆盖开书主链入口的方案解析：模型输出 → 可落库方案。
func TestOpenBookPlanChain(t *testing.T) {
	// 环节1：容错。模型常在 JSON 前后加解释文字或代码围栏，解析必须仍能取出完整结构。
	content := "好的，方案如下：\n```json\n" +
		`{"project_name":"长夜行","genre_code":"fantasy","selling_point_text":"废柴弟子必须在三年内筑基",
"synopsis_text":"简介","style_text":"冷峻",
"volume_list":[{"volume_name":"第一卷","summary_text":"入门","chapter_list":[{"chapter_name":"第一章 钥匙","summary_text":"捡到小瓶"}]}],
"character_list":[{"character_name":"韩立","role_type_code":"protagonist","personality_text":"谨慎","goal_text":"筑基","background_text":"外门弟子"}],
"lore_list":[{"lore_name":"催熟小瓶","lore_type_code":"rule","content_text":"规则/边界/代价"}]}` +
		"\n```\n希望有帮助。"
	plan, err := ParseBootstrapPlan(content)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if plan.ProjectName != "长夜行" || len(plan.VolumeList) != 1 ||
		len(plan.VolumeList[0].ChapterList) != 1 ||
		plan.VolumeList[0].ChapterList[0].ChapterName != "第一章 钥匙" ||
		len(plan.CharacterList) != 1 || plan.CharacterList[0].RoleTypeCode != "protagonist" {
		t.Fatalf("解析结果不符: %+v", plan)
	}

	// 环节2：拒绝。无 JSON、缺书名、缺卷章与非法 JSON 一律报错，不产出半成品方案。
	badCases := map[string]string{
		"无 JSON":  "抱歉，我无法完成。",
		"缺书名":     `{"project_name":"","volume_list":[{"volume_name":"第一卷","chapter_list":[]}]}`,
		"缺卷章结构":   `{"project_name":"长夜行","volume_list":[]}`,
		"非法 JSON": `{"project_name":"长夜行","volume_list":[}`,
	}
	for name, bad := range badCases {
		if _, err := ParseBootstrapPlan(bad); err == nil {
			t.Fatalf("%s：期望解析失败，实际成功", name)
		}
	}
}
