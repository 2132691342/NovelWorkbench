// 工作流链路的回归：模板可播种 → 运行参数收敛 → 目标章节解析与用途匹配 → 审核状态机。
// 这几件事决定「点一下就能跑」是否成立：模板形状错会让运行立即失败，
// 参数越界会让模型跑飞，章节范围选错会覆盖作者已写好的正文，状态机错会让审核按钮点了没反应。

package service

import (
	"strings"
	"testing"

	"github.com/2132691342/NovelWorkbench/backend/internal/ai"
	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
)

// draftOutputPromptCodeSet 输出会被直接写入章节正文的提示词编码。
// 这些提示词一旦要求模型输出清单或说明，那些文字就会变成正文。
var draftOutputPromptCodeSet = map[string]bool{
	"chapter": true,
	"polish":  true,
	"deai":    true,
}

// TestWorkflowTemplateChain 覆盖内置模板从定义到可执行的完整链路。
func TestWorkflowTemplateChain(t *testing.T) {
	// 步骤1：模板形状。每个模板必须有步骤、有可读名称与说明，编码唯一。
	codeSet := map[string]bool{}
	for _, seed := range workflowTemplateSeedList {
		if seed.TemplateCode == "" || seed.TemplateName == "" || seed.DescriptionText == "" {
			t.Fatalf("模板缺少必填字段：%+v", seed)
		}
		if codeSet[seed.TemplateCode] {
			t.Fatalf("模板编码重复：%s", seed.TemplateCode)
		}
		codeSet[seed.TemplateCode] = true
		if len(seed.StepList) == 0 {
			t.Fatalf("模板 %s 没有步骤", seed.TemplateCode)
		}

		// 步骤2：每个模板都必须有「出稿」与「入库」，且入库排在出稿之后，
		// 否则要么没有产出，要么产出无处落地。
		firstIndex, lastIndex := -1, -1
		for i, step := range seed.StepList {
			if step.StepName == "" || step.StepTypeCode == "" {
				t.Fatalf("模板 %s 的步骤缺少名称或类型：%+v", seed.TemplateCode, step)
			}
			switch step.StepTypeCode {
			case domain.WorkflowStepTypeDraft:
				if firstIndex < 0 {
					firstIndex = i
				}
			case domain.WorkflowStepTypeApply:
				lastIndex = i
			}
		}
		if firstIndex < 0 || lastIndex < 0 || lastIndex < firstIndex {
			t.Fatalf("模板 %s 步骤顺序不合法：出稿 %d，入库 %d", seed.TemplateCode, firstIndex, lastIndex)
		}

		// 步骤3：AI 步骤必须引用提示词，否则运行期取不到正文。
		for _, step := range seed.StepList {
			if step.StepTypeCode == domain.WorkflowStepTypeDraft ||
				step.StepTypeCode == domain.WorkflowStepTypeReview ||
				step.StepTypeCode == domain.WorkflowStepTypeSummary {
				if step.PromptCode == "" {
					t.Fatalf("模板 %s 的 AI 步骤 %s 缺少提示词编码", seed.TemplateCode, step.StepName)
				}
			}
		}
	}

	// 步骤4：序列化往返。模板以 JSON 落库，往返丢失字段会让运行期拿到残缺步骤。
	for _, seed := range workflowTemplateSeedList {
		roundTrip := unmarshalWorkflowSteps(marshalWorkflowSteps(seed.StepList))
		if len(roundTrip) != len(seed.StepList) {
			t.Fatalf("模板 %s 步骤序列化往返丢失：%d → %d", seed.TemplateCode, len(seed.StepList), len(roundTrip))
		}
		for i := range roundTrip {
			if roundTrip[i].StepTypeCode != seed.StepList[i].StepTypeCode ||
				roundTrip[i].PromptCode != seed.StepList[i].PromptCode ||
				roundTrip[i].SkipWhenPass != seed.StepList[i].SkipWhenPass {
				t.Fatalf("模板 %s 步骤 %d 往返字段不一致：%+v", seed.TemplateCode, i, roundTrip[i])
			}
		}
	}
}

// TestWorkflowRunChain 覆盖一次运行从参数到状态机的完整链路。
func TestWorkflowRunChain(t *testing.T) {
	// 环节1：默认值。作者不填也要能跑起来。
	defaults := domain.WorkflowRunParamsDTO{}
	if err := normalizeWorkflowParams(&defaults); err != nil {
		t.Fatalf("默认参数应通过校验: %v", err)
	}
	if defaults.TargetWordCount != workflowDefaultTargetWords || defaults.GateScore != workflowDefaultGateScore {
		t.Fatalf("默认参数未补齐：%+v", defaults)
	}
	if defaults.ReviewGateFlag == nil || !*defaults.ReviewGateFlag {
		t.Fatal("人工审核门禁应默认开启")
	}

	// 环节2：越界拒绝。字数与分数直接决定模型输出规模，不能放行极端值。
	badCases := []domain.WorkflowRunParamsDTO{
		{TargetWordCount: 10},
		{TargetWordCount: 999999},
		{GateScore: 10},
		{GateScore: 200},
		{NewChapterCount: 999},
	}
	for i := range badCases {
		if err := normalizeWorkflowParams(&badCases[i]); err == nil {
			t.Fatalf("越界参数应被拒绝：%+v", badCases[i])
		}
	}

	// 环节3：显式关闭审核门禁必须被保留，不能被默认值覆盖。
	disabledFlag := false
	params := domain.WorkflowRunParamsDTO{ReviewGateFlag: &disabledFlag}
	if err := normalizeWorkflowParams(&params); err != nil {
		t.Fatalf("合法参数应通过: %v", err)
	}
	if params.ReviewGateFlag == nil || *params.ReviewGateFlag {
		t.Fatal("显式关闭的审核门禁被默认值覆盖")
	}

	// 环节4：状态机。只有待审核能审核，只有失败能重试，只有暂停能继续。
	if !isWorkflowRunActive(domain.WorkflowRunStatusRunning) ||
		!isWorkflowRunActive(domain.WorkflowRunStatusWaitingReview) {
		t.Fatal("执行中与待审核都应视为活跃运行")
	}
	if isWorkflowRunActive(domain.WorkflowRunStatusSucceeded) ||
		isWorkflowRunActive(domain.WorkflowRunStatusFailed) {
		t.Fatal("已结束的运行不应视为活跃")
	}

	// 环节5：运行与章节的对应关系，越界与空列表都要安全返回。
	run := &domain.WorkflowRunDO{ChapterULIDListJSON: `["CHP_1","CHP_2"]`}
	if got := currentChapterULIDOfRun(run); got != "CHP_1" {
		t.Fatalf("首页应指向第一章，实际 %q", got)
	}
	run.CurrentIndex = 1
	if got := currentChapterULIDOfRun(run); got != "CHP_2" {
		t.Fatalf("第二页应指向第二章，实际 %q", got)
	}
	// 越界不能 panic：审核通过后下标会前移到 len，此时不应有当前章。
	run.CurrentIndex = 2
	if got := currentChapterULIDOfRun(run); got != "" {
		t.Fatalf("越界下标应返回空章节，实际 %q", got)
	}
	if got := currentChapterULIDOfRun(&domain.WorkflowRunDO{}); got != "" {
		t.Fatalf("空章节列表应返回空串，实际 %q", got)
	}

	// 环节6：模板用途决定目标章节范围。
	// 起草类拿空白章，打磨/去 AI 味类拿已有正文的章；规则错了会「静默做错事」，
	// 要么空跑一遍，要么覆盖作者已写好的正文。
	if !templateNeedsWrittenChapter(domain.WorkflowTemplateChapterPolish) ||
		!templateNeedsWrittenChapter(domain.WorkflowTemplateChapterDeai) {
		t.Fatal("打磨与去 AI 味模板应处理已有正文的章节")
	}
	if templateNeedsWrittenChapter(domain.WorkflowTemplateChapterDraft) {
		t.Fatal("逐章成稿模板应处理空白章节，不能拿已有正文")
	}
	// 未知编码按起草类处理：宁可用空白章，也不覆盖已有正文。
	if templateNeedsWrittenChapter("unknown_template") {
		t.Fatal("未知模板不应默认按已有正文处理")
	}
}

// TestWorkflowBodyOnlyPromptChain 守住「起草类步骤的输出会被原样写进章节」这条契约：
// 提示词正文里不能出现要求模型输出分析/清单的字样，否则那些文字会变成章节正文。
// 这条回归来自一次真实事故：去 AI 味提示词要求「先列命中清单，再输出正文」，
// 结果整份清单被当作正文写入了章节。
func TestWorkflowBodyOnlyPromptChain(t *testing.T) {
	// 环节1：系统提示词必须明确禁止过程文字。
	if !strings.Contains(workflowBodyOnlySystemPrompt, "只输出") ||
		!strings.Contains(workflowBodyOnlySystemPrompt, "不要输出") {
		t.Fatalf("起草类系统提示词约束不足：%s", workflowBodyOnlySystemPrompt)
	}

	// 环节2：所有会被写入正文的提示词，正文里不得要求先输出清单或说明。
	for _, domainItem := range ai.BuiltinDomains() {
		for _, command := range domainItem.CommandList {
			if !draftOutputPromptCodeSet[command.PromptCode] {
				continue
			}
			// 只拦「要求先输出过程」的正面指令；「不写改动说明」这类反向约束是合规的。
			for _, forbidden := range []string{"先列", "再输出改写", "输出格式：", "1. 先"} {
				if strings.Contains(command.PromptText, forbidden) {
					t.Fatalf("提示词 %s 要求输出过程文字（%q），其输出会被当作正文入库", command.PromptCode, forbidden)
				}
			}
		}
	}
}
