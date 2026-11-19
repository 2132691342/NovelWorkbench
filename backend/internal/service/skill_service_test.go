// 本文件覆盖技能链路：编码解析（显式优先 / 正文首 token）→ 作用域优先级（项目覆盖全局）→
// 变量取值（显式 > 缺省 > 必填缺失）→ 模板占位符校验 → 参考资料关键词命中与预算截断 → 全局技能覆盖标记。
// 技能决定了 AI 看到什么提示词与哪些参考资料，任一环节偏差都会直接改变产出，因此合并为一条用例。

package service

import (
	"strings"
	"testing"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
)

// skillDO 构造测试用技能行。
func skillDO(ulid, scope, projectULID, code string, enabled bool) domain.SkillDO {
	return domain.SkillDO{
		SkillULID:      ulid,
		SkillCode:      code,
		SkillName:      code,
		ScopeCode:      scope,
		ProjectULID:    projectULID,
		EnabledFlag:    enabled,
		BuiltinFlag:    scope == domain.SkillScopeGlobal,
		PromptTemplate: "{{user_input}}",
	}
}

// TestSkillResolutionChain 覆盖技能解析与渲染链路。
func TestSkillResolutionChain(t *testing.T) {
	// ---- 环节一：技能编码解析（显式编码优先；否则取正文首 token 的 /code） ----
	code, remainder := resolveSkillCode("", "/continue 以林澈视角")
	if code != "continue" || remainder != "以林澈视角" {
		t.Fatalf("正文命令解析失败: %q %q", code, remainder)
	}
	code, remainder = resolveSkillCode("polish", "/continue 以林澈视角")
	if code != "polish" || remainder != "/continue 以林澈视角" {
		t.Fatalf("显式编码应优先且不剥离正文: %q %q", code, remainder)
	}
	code, remainder = resolveSkillCode("", "普通的正文开头")
	if code != "" || remainder != "普通的正文开头" {
		t.Fatalf("未命中技能时正文应原样保留: %q %q", code, remainder)
	}
	code, remainder = resolveSkillCode("", "续写 /continue")
	if code != "" || remainder != "续写 /continue" {
		t.Fatalf("命令不在首 token 时不应命中: %q %q", code, remainder)
	}

	// ---- 环节二：作用域优先级（启用项目技能 > 启用全局技能；停用一律不命中） ----
	projectSkill := skillDO("SKL_P", domain.SkillScopeProject, "PRJ_1", "polish", true)
	globalSkill := skillDO("SKL_G", domain.SkillScopeGlobal, "", "polish", true)
	if winner := pickSkillByScope(&projectSkill, &globalSkill); winner.SkillULID != "SKL_P" {
		t.Fatalf("项目技能应优先: %s", winner.SkillULID)
	}
	disabledProject := skillDO("SKL_P", domain.SkillScopeProject, "PRJ_1", "polish", false)
	if winner := pickSkillByScope(&disabledProject, &globalSkill); winner.SkillULID != "SKL_G" {
		t.Fatalf("停用的项目技能应回退全局技能: %s", winner.SkillULID)
	}
	if winner := pickSkillByScope(nil, &globalSkill); winner.SkillULID != "SKL_G" {
		t.Fatalf("无项目技能时应取全局技能: %s", winner.SkillULID)
	}
	if winner := pickSkillByScope(nil, nil); winner != nil {
		t.Fatalf("技能不存在时不应命中: %+v", winner)
	}
	disabledGlobal := skillDO("SKL_G", domain.SkillScopeGlobal, "", "polish", false)
	if winner := pickSkillByScope(nil, &disabledGlobal); winner != nil {
		t.Fatalf("停用的全局技能不应命中: %+v", winner)
	}

	// ---- 环节三：变量取值（显式传值 > 声明缺省值 > 必填缺失记入 missing） ----
	variableList := []domain.SkillVariableDO{
		{VariableName: "tone", VariableLabel: "语气", DefaultValue: "克制"},
		{VariableName: "scene", VariableLabel: "场景", RequiredFlag: true},
		{VariableName: "optional", VariableLabel: "可选"},
	}
	valueMap, missing := resolveSkillVariables("polish", variableList, map[string]string{"scene": "雨夜"}, "正文")
	if valueMap["tone"] != "克制" || valueMap["scene"] != "雨夜" || valueMap["user_input"] != "正文" {
		t.Fatalf("变量取值错误: %+v", valueMap)
	}
	if valueMap["skill_code"] != "polish" {
		t.Fatalf("内置变量 skill_code 未注入: %+v", valueMap)
	}
	if valueMap["optional"] != "" {
		t.Fatalf("非必填无值变量应为空串: %+v", valueMap)
	}
	if len(missing) != 0 {
		t.Fatalf("必填变量已提供时不应缺失: %v", missing)
	}
	_, missing = resolveSkillVariables("polish", variableList, nil, "正文")
	if len(missing) != 1 || missing[0] != "scene" {
		t.Fatalf("必填变量缺失应被记录: %v", missing)
	}
	// 显式空值不算已提供，回落到声明缺省值。
	valueMap, missing = resolveSkillVariables("polish", variableList, map[string]string{"tone": ""}, "正文")
	if valueMap["tone"] != "克制" {
		t.Fatalf("显式空值应回落缺省值: %+v", valueMap)
	}
	if len(missing) != 1 {
		t.Fatalf("必填变量仍应缺失: %v", missing)
	}

	// ---- 环节四：模板占位符校验（未声明变量必须拒绝） ----
	if err := validateSkillTemplate("润色 {{user_input}} 与 {{tone}}", variableList); err != nil {
		t.Fatalf("已声明占位符不应报错: %v", err)
	}
	if err := validateSkillTemplate("润色 {{unknown_var}}", variableList); err == nil {
		t.Fatal("未声明占位符必须被拒绝")
	}
	if err := validateSkillTemplate("润色 {{user_input}}", []domain.SkillVariableDO{{VariableName: ""}}); err == nil {
		t.Fatal("空变量名必须被拒绝")
	}

	// ---- 环节五：参考资料关键词命中（空关键词总是命中；任一关键词出现即命中） ----
	if !matchSkillReferenceKeywords("", []string{"任意正文"}) {
		t.Fatal("空关键词应总是命中")
	}
	if !matchSkillReferenceKeywords("雨夜, 追车", []string{"这是一个雨夜"}) {
		t.Fatal("关键词出现在正文中应命中")
	}
	if matchSkillReferenceKeywords("雨夜", []string{"晴天的午后"}) {
		t.Fatal("关键词未出现时不应命中")
	}
	if !matchSkillReferenceKeywords("雨夜,追车", []string{"晴天的午后", "追车开始"}) {
		t.Fatal("任一关键词命中即应命中")
	}

	// ---- 环节六：关键词列表归一（去空去重后以逗号拼接） ----
	if got := normalizeKeywordList(" 雨夜 , 追车 ,雨夜, "); got != "雨夜,追车" {
		t.Fatalf("关键词归一结果 = %q", got)
	}
	keywords := splitKeywordList("a,,b,a")
	if len(keywords) != 2 || keywords[0] != "a" || keywords[1] != "b" {
		t.Fatalf("关键词拆分结果 = %v", keywords)
	}

	// ---- 环节七：全局技能被同编码项目技能覆盖时须打标记 ----
	global := skillDO("SKL_G", domain.SkillScopeGlobal, "", "polish", true)
	project := skillDO("SKL_P", domain.SkillScopeProject, "PRJ_1", "polish", true)
	if !isOverriddenGlobalSkill(&global, []domain.SkillDO{global, project}) {
		t.Fatal("全局技能应被同编码项目技能标记为已覆盖")
	}
	if isOverriddenGlobalSkill(&project, []domain.SkillDO{global, project}) {
		t.Fatal("项目技能不应被标记为已覆盖")
	}
	disabledProjectOverride := skillDO("SKL_P", domain.SkillScopeProject, "PRJ_1", "polish", false)
	if isOverriddenGlobalSkill(&global, []domain.SkillDO{global, disabledProjectOverride}) {
		t.Fatal("停用的项目技能不应覆盖全局技能")
	}
	other := skillDO("SKL_O", domain.SkillScopeProject, "PRJ_1", "continue", true)
	if isOverriddenGlobalSkill(&global, []domain.SkillDO{global, other}) {
		t.Fatal("不同编码的项目技能不应覆盖全局技能")
	}

	// ---- 环节八：参考资料预算截断（命中后按字符预算截断，超出即停） ----
	longText := strings.Repeat("x", skillReferenceBudgetChars+100)
	dos := []domain.SkillReferenceDO{
		{RefULID: "REF_1", SkillULID: "SKL_G", RefName: "长文", RefText: longText, KeywordList: ""},
	}
	selected := selectSkillReferences(dos, []string{"任意正文"}, skillReferenceBudgetChars)
	if len(selected) != 0 {
		t.Fatalf("超预算资料不应被注入: %v", selected)
	}
	short := []domain.SkillReferenceDO{
		{RefULID: "REF_1", SkillULID: "SKL_G", RefName: "短文", RefText: "样例正文", KeywordList: ""},
	}
	selected = selectSkillReferences(short, []string{"任意正文"}, skillReferenceBudgetChars)
	if len(selected) != 1 || !strings.Contains(selected[0], "【短文】") || !strings.Contains(selected[0], "样例正文") {
		t.Fatalf("参考资料注入格式错误: %v", selected)
	}
}
