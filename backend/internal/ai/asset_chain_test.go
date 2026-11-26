// 出厂提示词资产与开书方案解析的回归：资产形状与占位符 → 模型输出解析成可落库方案。
// 这两件事决定「开箱即用」是否成立：资产形状错会让播种出的提示词无法触发，
// 方案解析错会让整份开书结果作废。

package ai

import "testing"

// TestBuiltinAssetChain 覆盖出厂提示词从定义到可播种的完整链路。
func TestBuiltinAssetChain(t *testing.T) {
	domainList := BuiltinDomains()
	if len(domainList) == 0 {
		t.Fatal("内置流程域为空")
	}

	// 步骤1：形状与编码唯一。身份与指令编码重复会在播种时互相覆盖。
	codeSet := map[string]bool{}
	for _, item := range domainList {
		if item.DomainCode == "" || item.DomainName == "" || item.SystemPromptCode == "" ||
			item.SystemPromptText == "" || len(item.CommandList) < 1 || len(item.CommandList) > 3 {
			t.Fatalf("流程域形状不合法：%+v", item)
		}
		if codeSet[item.SystemPromptCode] {
			t.Fatalf("身份提示词编码重复：%s", item.SystemPromptCode)
		}
		codeSet[item.SystemPromptCode] = true
		for _, command := range item.CommandList {
			if codeSet[command.PromptCode] {
				t.Fatalf("指令提示词编码重复：%s", command.PromptCode)
			}
			codeSet[command.PromptCode] = true
		}
	}

	// 步骤2：占位符可渲染。未登记占位符会被渲染为空串，等于静默丢上下文。
	for _, item := range domainList {
		if unknown := TemplatePlaceholders(item.SystemPromptText, BuiltinPlaceholders); len(unknown) > 0 {
			t.Fatalf("身份提示词 %s 使用了未登记的占位符：%v", item.SystemPromptCode, unknown)
		}
		for _, command := range item.CommandList {
			if unknown := TemplatePlaceholders(command.PromptText, BuiltinPlaceholders); len(unknown) > 0 {
				t.Fatalf("提示词 %s 使用了未登记的占位符：%v", command.PromptCode, unknown)
			}
		}
	}
}

// TestOpenBookPlanChain 覆盖开书主链入口的方案解析：模型输出 → 可落库方案。
func TestOpenBookPlanChain(t *testing.T) {
	// 步骤1：容错。模型常在 JSON 前后加解释文字或代码围栏，解析必须仍能取出完整结构。
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

	// 步骤2：拒绝。无 JSON、缺书名、缺卷章与非法 JSON 一律报错，不产出半成品方案。
	badCases := map[string]string{
		"无 JSON":    "抱歉，我无法完成。",
		"缺书名":       `{"project_name":"","volume_list":[{"volume_name":"第一卷","chapter_list":[]}]}`,
		"缺卷章结构":     `{"project_name":"长夜行","volume_list":[]}`,
		"非法 JSON":   `{"project_name":"长夜行","volume_list":[}`,
	}
	for name, bad := range badCases {
		if _, err := ParseBootstrapPlan(bad); err == nil {
			t.Fatalf("%s：期望解析失败，实际成功", name)
		}
	}
}
