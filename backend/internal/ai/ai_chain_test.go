// AI 基础能力的长链路回归：出厂资产 → 开书方案解析 → 模型网关实例构造。
// 三条链路的共同点是「配错才在生产暴露」：资产形状错会让播种出的技能无法触发，
// 方案解析错会让整份开书结果作废，缓存键漏维度会让换密钥后继续用旧凭据。

package ai

import (
	"net/http"
	"testing"
	"time"
)

// TestBuiltinAssetChain 覆盖出厂资产从定义到可播种的完整链路：
// 域形状（每域一份档案两个技能）→ 编码全局唯一 → 模板占位符全部登记 → 指纹稳定。
func TestBuiltinAssetChain(t *testing.T) {
	domainList := BuiltinDomains()
	if len(domainList) == 0 {
		t.Fatal("内置流程域为空")
	}

	// 步骤1：形状与编码唯一。档案与技能编码重复会在播种时互相覆盖。
	profileCodeSet := map[string]bool{}
	skillCodeSet := map[string]bool{}
	for _, item := range domainList {
		if item.DomainCode == "" || item.DomainName == "" || item.Profile.ProfileCode == "" ||
			item.Profile.SystemPrompt == "" || len(item.SkillList) != 2 {
			t.Fatalf("流程域形状不合法：%+v", item)
		}
		if profileCodeSet[item.Profile.ProfileCode] {
			t.Fatalf("档案编码重复：%s", item.Profile.ProfileCode)
		}
		profileCodeSet[item.Profile.ProfileCode] = true
		for _, skill := range item.SkillList {
			if skillCodeSet[skill.CommandCode] {
				t.Fatalf("技能编码重复：%s", skill.CommandCode)
			}
			skillCodeSet[skill.CommandCode] = true
		}
	}

	// 步骤2：模板可渲染。占位符未登记会让渲染后变量为空，技能在对话中无法产出可用提示词。
	for _, item := range domainList {
		for _, skill := range item.SkillList {
			if unknown := TemplatePlaceholders(skill.PromptTemplate, BuiltinPlaceholders); len(unknown) > 0 {
				t.Fatalf("技能 %s 使用了未登记的占位符：%v", skill.CommandCode, unknown)
			}
		}
	}

	// 步骤3：指纹稳定。格式或算法变化会让历史资产被误判为「内容已变」而全量重播种。
	for _, item := range domainList {
		for _, skill := range item.SkillList {
			if len(skill.ContentHash()) != 64 {
				t.Fatalf("技能 %s 指纹异常", skill.CommandCode)
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

// TestModelGatewayChain 覆盖模型网关的实例构造：缓存键维度 → 流式客户端超时策略。
func TestModelGatewayChain(t *testing.T) {
	// 步骤1：缓存键。密钥、协议、地址、模型、温度、输出上限、超时任一变化都必须换键，
	// 否则换密钥后实例被复用，请求会继续带旧凭据。
	base := ProviderSpec{
		ProviderULID: "PRV_1", ProtocolCode: ProtocolOpenAI, BaseURL: "https://gw.example.com",
		APIKey: "key-a", ChatModelName: "m1", Temperature: 0.7, MaxTokens: 2048, Timeout: 120 * time.Second,
	}
	mutations := []func(spec *ProviderSpec){
		func(spec *ProviderSpec) { spec.APIKey = "key-b" },
		func(spec *ProviderSpec) { spec.ProtocolCode = ProtocolAnthropic },
		func(spec *ProviderSpec) { spec.BaseURL = "https://gw2.example.com" },
		func(spec *ProviderSpec) { spec.ChatModelName = "m2" },
		func(spec *ProviderSpec) { spec.Temperature = 0.9 },
		func(spec *ProviderSpec) { spec.MaxTokens = 4096 },
		func(spec *ProviderSpec) { spec.Timeout = 60 * time.Second },
	}
	for _, mutate := range mutations {
		changed := base
		mutate(&changed)
		if base.cacheKey() == changed.cacheKey() {
			t.Fatalf("配置变更后缓存键未变化，实例会被错误复用：%+v", changed)
		}
	}

	// 步骤2：流式客户端。整体超时会切断长流，只能靠首包超时与调用方 ctx 控制。
	client := newStreamHTTPClient(45 * time.Second)
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("应使用 *http.Transport，实际 %T", client.Transport)
	}
	if client.Timeout != 0 || transport.ResponseHeaderTimeout != 45*time.Second {
		t.Fatalf("流式客户端超时策略错误：整体 %v，首包 %v", client.Timeout, transport.ResponseHeaderTimeout)
	}
}
