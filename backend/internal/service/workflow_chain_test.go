// 工作流的完整链路回归：模板形状 → 运行参数 → 目标章节解析 → 状态机 → 就绪度 → 上下文收敛 → 审核门禁。
// 这些环节共同决定「点一下就能跑」是否成立：模板形状错会让运行立即失败，
// 参数越界会让模型跑飞，章节范围选错会覆盖作者已写好的正文，
// 就绪度算错会让用户看到「素材齐备」却一开跑就失败，审核门禁收口错会让审核按钮点了没反应。

package service

import (
	"context"
	"strings"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/2132691342/NovelWorkbench/backend/internal/ai"
	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/repo"
)

// draftOutputPromptCodeSet 输出会被直接写入章节正文的提示词编码。
// 这些提示词一旦要求模型输出清单或说明，那些文字就会变成正文。
var draftOutputPromptCodeSet = map[string]bool{
	"chapter":  true,
	"continue": true,
	"polish":   true,
	"deai":     true,
}

// TestWorkflowTemplateChain 覆盖内置模板从定义到可执行的完整链路。
func TestWorkflowTemplateChain(t *testing.T) {
	// 环节1：模板形状。每个模板必须有步骤、有可读名称与说明，编码唯一。
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

		// 环节2：每个模板都必须有「出稿」与「入库」，且入库排在出稿之后，
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

		// 环节3：AI 步骤必须引用提示词，否则运行期取不到正文。
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

	// 环节4：序列化往返。模板以 JSON 落库，往返丢失字段会让运行期拿到残缺步骤。
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

// TestWorkflowReadinessChain 覆盖开跑前的就绪度体检：评分口径、检查项文案与阻塞标记。
func TestWorkflowReadinessChain(t *testing.T) {
	// 环节1：评分边界。没有检查项、全就绪、部分就绪。
	if got := readinessScoreOf(0, 0); got != 0 {
		t.Fatalf("无检查项应为 0 分，实际 %d", got)
	}
	if got := readinessScoreOf(9, 9); got != 100 {
		t.Fatalf("全部就绪应为 100 分，实际 %d", got)
	}
	// 4/9 ≈ 44.4，整数除法截断到 44（宁可低报不虚报）。
	if got := readinessScoreOf(4, 9); got != 44 {
		t.Fatalf("4/9 应为 44 分，实际 %d", got)
	}

	// 环节2：数量类检查项。未达标带补齐建议，达标只给确认文案。
	notReady := readinessCountItem(readinessItemCharacter, "角色卡", 2, readinessCharacterMin, true,
		"先补角色卡", "/material/P1?tab=character")
	if notReady.ReadyFlag || !notReady.BlockingFlag {
		t.Fatalf("2 张角色卡应为未就绪且阻塞：%+v", notReady)
	}
	if notReady.HintText != "先补角色卡" {
		t.Fatalf("未就绪应保留补齐建议，实际 %q", notReady.HintText)
	}
	if notReady.CurrentValue != "2 条" || notReady.TargetValue != "≥3 条" {
		t.Fatalf("当前值/目标值不符：%q / %q", notReady.CurrentValue, notReady.TargetValue)
	}
	ready := readinessCountItem(readinessItemCharacter, "角色卡", readinessCharacterMin, readinessCharacterMin, true,
		"先补角色卡", "/material/P1?tab=character")
	if !ready.ReadyFlag || ready.HintText != "已就绪" {
		t.Fatalf("达到下限应就绪且不再提示补齐：%+v", ready)
	}

	// 环节3：文本类检查项。空白串视为未就绪，非阻塞项不参与阻塞计数。
	blankItem := readinessTextItem(readinessItemProjectStyle, "文风设定", "   ", "补一句文风", "/dashboard")
	if blankItem.ReadyFlag || blankItem.BlockingFlag {
		t.Fatalf("空白文风应为未就绪且不阻塞：%+v", blankItem)
	}
	filledItem := readinessTextItem(readinessItemProjectSynopsis, "项目简介", "一个关于退稿的故事", "补一句简介", "/dashboard")
	if !filledItem.ReadyFlag || filledItem.CurrentValue != "已就绪" {
		t.Fatalf("非空简介应就绪：%+v", filledItem)
	}

	// 环节4：提示词缺口文案点名具体编码（编码比数量更能指导补齐）。
	hintText := readinessPromptHint(false)
	for _, promptCode := range readinessPromptCodeList {
		if !strings.Contains(hintText, promptCode) {
			t.Fatalf("提示文案应点名编码 %s，实际 %q", promptCode, hintText)
		}
	}
	if readinessPromptHint(true) != "已就绪" {
		t.Fatal("就绪时不应再提示补齐")
	}
}

// TestWorkflowContextChain 覆盖工作流上下文装配的总量收敛：各块限长之外还要兜住总量，
// 否则素材多的项目会把提示词顶爆。
func TestWorkflowContextChain(t *testing.T) {
	// 情况1：短文本原样返回（只去首尾空白）。
	shortText := "【本章】第 1 章 开端"
	if got := condenseContextText("  " + shortText + "\n"); got != shortText {
		t.Fatalf("短上下文应原样返回，实际：%q", got)
	}

	// 情况2：恰好等于上限，不触发省略。
	exactText := strings.Repeat("字", workflowContextTotalMaxRunes)
	if got := condenseContextText(exactText); got != exactText {
		t.Fatalf("等于上限时不应省略，实际长度：%d", len([]rune(got)))
	}

	// 情况3：超出上限时保留首尾并标注省略字数，总长不超过上限加标注。
	longText := strings.Repeat("甲", workflowContextTotalMaxRunes-100) +
		strings.Repeat("乙", 400) + strings.Repeat("丙", workflowContextTotalMaxRunes-100)
	got := condenseContextText(longText)
	if !strings.Contains(got, "上下文过长") {
		t.Fatalf("超长上下文应带省略标注，实际：%q", got[:60])
	}
	if !strings.HasPrefix(got, "甲") || !strings.HasSuffix(got, "丙") {
		t.Fatalf("超长上下文应保留首尾，实际首尾：%q / %q", got[:6], got[len(got)-6:])
	}
	if len([]rune(got)) > workflowContextTotalMaxRunes+60 {
		t.Fatalf("收敛后仍过长：%d 字", len([]rune(got)))
	}
}

// TestWorkflowBodyOnlyPromptChain 守住「起草类步骤的输出会被原样写进章节」这条契约：
// 提示词正文里不能出现要求模型输出分析/清单的字样，否则那些文字会变成章节正文。
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

// TestWorkflowGateStepChain 覆盖门禁步骤的收口：审核通过/退回必须原地更新进入门禁时写的那一行，
// 不能再插一行终态。多插一行会永久残留一条 running 僵尸步骤，
// 前端时间线同时显示「等待审核（进行中）」和「人工审核（通过）」，看起来像卡住。
func TestWorkflowGateStepChain(t *testing.T) {
	// 环节1：收口分支选择。命中 running 门禁行走更新，缺失才回退插入。
	if got := gateStepCloseAction(nil); got != gateStepActionInsert {
		t.Fatalf("找不到门禁行应回退插入，实际 %q", got)
	}
	gateStep := &domain.WorkflowStepDO{
		StepULID:       "WFS_TEST",
		StepTypeCode:   domain.WorkflowStepTypeHumanReview,
		StepStatusCode: domain.WorkflowStepStatusRunning,
	}
	if got := gateStepCloseAction(gateStep); got != gateStepActionUpdate {
		t.Fatalf("命中门禁行应原地更新，实际 %q", got)
	}

	// 环节2：门禁定位条件。类型与状态都必须锁死，否则会误更新普通步骤行或已收口的门禁行。
	capture := &sqlCapture{}
	db, err := gorm.Open(postgres.New(postgres.Config{DSN: "host=127.0.0.1 port=1 user=x dbname=x"}), &gorm.Config{
		DryRun:                 true,
		SkipDefaultTransaction: true,
		DisableAutomaticPing:   true,
		Logger:                 logger.New(capture, logger.Config{LogLevel: logger.Info}),
	})
	if err != nil {
		t.Fatalf("打开 DryRun 数据库失败: %v", err)
	}
	workflowRepo := repo.NewWorkflowRepo(db)
	// DryRun 下 Find 不返回行，这里只关心生成的 SQL 条件。
	if _, err := workflowRepo.FindRunningGateStep(context.Background(), "WFR_TEST", "CH_TEST", 4); err != nil {
		t.Fatalf("查询不应报错: %v", err)
	}
	sqlText := capture.joined()
	for _, expected := range []string{
		"run_ulid", "chapter_ulid", "step_index", "human_review", "running",
	} {
		if !strings.Contains(sqlText, expected) {
			t.Fatalf("门禁定位 SQL 缺少条件 %q：\n%s", expected, sqlText)
		}
	}
	if strings.Contains(sqlText, "INSERT") {
		t.Fatalf("门禁定位不应产生 INSERT：\n%s", sqlText)
	}

	// 环节3：找不到门禁行时回退插入一行终态（历史数据不报错）。
	fallbackCapture := &sqlCapture{}
	fallbackDB, err := gorm.Open(postgres.New(postgres.Config{DSN: "host=127.0.0.1 port=1 user=x dbname=x"}), &gorm.Config{
		DryRun:                 true,
		SkipDefaultTransaction: true,
		DisableAutomaticPing:   true,
		Logger:                 logger.New(fallbackCapture, logger.Config{LogLevel: logger.Info}),
	})
	if err != nil {
		t.Fatalf("打开 DryRun 数据库失败: %v", err)
	}
	fallbackRepo := repo.NewWorkflowRepo(fallbackDB)
	fallbackSvc := NewWorkflowService(fallbackRepo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	runDO := &domain.WorkflowRunDO{
		RunULID:             "WFR_TEST",
		ChapterULIDListJSON: `["CH_TEST"]`,
		CurrentIndex:        0,
		CurrentStepIndex:    4,
	}
	if err := fallbackSvc.closeGateStep(context.Background(), runDO, 4, domain.WorkflowStepTypeHumanReview,
		"人工审核", domain.WorkflowStepStatusSuccess, "审核通过"); err != nil {
		t.Fatalf("回退插入不应报错: %v", err)
	}
	if !strings.Contains(fallbackCapture.joined(), "INSERT") {
		t.Fatalf("找不到门禁行时应回退插入：\n%s", fallbackCapture.joined())
	}
}
