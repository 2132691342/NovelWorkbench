// 开书方案解析的回归：围栏与前后缀容错 → 结构完整性 → 缺字段拒绝。
// 解析是开书主链的入口，模型输出格式稍有偏差就会让整份方案作废或落库半成品，
// 因此把容错与拒绝两条路径合并为一条用例。

package ai

import "testing"

// TestParseBootstrapPlanChain 覆盖模型输出到可落库方案的完整解析链路。
func TestParseBootstrapPlanChain(t *testing.T) {
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
	if plan.ProjectName != "长夜行" || len(plan.VolumeList) != 1 {
		t.Fatalf("解析结果不符: %+v", plan)
	}
	if len(plan.VolumeList[0].ChapterList) != 1 || plan.VolumeList[0].ChapterList[0].ChapterName != "第一章 钥匙" {
		t.Fatalf("章节解析不符: %+v", plan.VolumeList[0].ChapterList)
	}
	if len(plan.CharacterList) != 1 || plan.CharacterList[0].RoleTypeCode != "protagonist" {
		t.Fatalf("角色解析不符: %+v", plan.CharacterList)
	}

	// 步骤2：拒绝。无 JSON、缺书名、缺卷章与非法 JSON 一律报错，不产出半成品方案。
	cases := map[string]string{
		"无 JSON":  "抱歉，我无法完成。",
		"缺书名":     `{"project_name":"","volume_list":[{"volume_name":"第一卷","chapter_list":[]}]}`,
		"缺卷章结构":   `{"project_name":"长夜行","volume_list":[]}`,
		"非法 JSON": `{"project_name":"长夜行","volume_list":[}`,
	}
	for name, bad := range cases {
		if _, err := ParseBootstrapPlan(bad); err == nil {
			t.Fatalf("%s：期望解析失败，实际成功", name)
		}
	}
}
