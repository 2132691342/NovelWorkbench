// 提示词资产的服务侧链路回归：播种形状与编码契约 → 下线清理 → 身份提示词兜底。
// 这几件事决定「开箱即用」是否成立：播种少一条会让工作流步骤取不到正文，
// 下线清理漏掉身份类会让库里残留点不动的僵尸条目，
// 兜底失效会让系统消息整段缺失，模型直接失去作者人格约束。

package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/2132691342/NovelWorkbench/backend/internal/ai"
	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/repo"
)

// errTestQuery 模拟仓储查询失败，用于验证兜底不被错误打断。
var errTestQuery = errors.New("查询失败")

// TestPromptAssetChain 覆盖提示词资产从播种到下线的完整链路。
func TestPromptAssetChain(t *testing.T) {
	flowList := ai.BuiltinDomains()
	commandList := ai.BuiltinCommandPrompts()

	// 环节1：播种形状。4 个流程、每个恰好 2 条、共 8 条，且全部是 command 类。
	if len(flowList) != 4 {
		t.Fatalf("内置流程应为 4 个，实际 %d", len(flowList))
	}
	if len(commandList) != 8 {
		t.Fatalf("内置任务提示词应为 8 条，实际 %d", len(commandList))
	}
	expectedCodeSet := map[string]bool{
		"outline": true, "chapter_plan": true,
		"chapter": true, "continue": true,
		"polish": true, "deai": true,
		"review": true, "summary": true,
	}
	for _, command := range commandList {
		if !expectedCodeSet[command.PromptCode] {
			t.Fatalf("出现未知编码：%s", command.PromptCode)
		}
		delete(expectedCodeSet, command.PromptCode)
	}
	if len(expectedCodeSet) > 0 {
		t.Fatalf("缺少内置编码：%v", expectedCodeSet)
	}

	// 环节2：编码归属。每个编码必须能反查回自己的流程，供提示词库分组展示。
	for _, flow := range flowList {
		for _, command := range flow.CommandList {
			flowCode, flowName := ai.BuiltinDomainOf(command.PromptCode)
			if flowCode != flow.FlowCode || flowName != flow.FlowName {
				t.Fatalf("编码 %s 归属错误：%s/%s", command.PromptCode, flowCode, flowName)
			}
			if ai.DomainNameOf(flow.FlowCode) != flow.FlowName {
				t.Fatalf("流程名反查失败：%s", flow.FlowCode)
			}
		}
	}

	// 环节3：写作类提示词不得要求先输出过程文字，其输出会被原样写进章节。
	for _, command := range commandList {
		if !draftOutputPromptCodeSet[command.PromptCode] {
			continue
		}
		for _, forbidden := range []string{"先列", "先说明改动", "再输出改写", "1. 先"} {
			if strings.Contains(command.PromptText, forbidden) {
				t.Fatalf("提示词 %s 要求输出过程文字（%q）", command.PromptCode, forbidden)
			}
		}
	}

	// 环节4：复审 JSON 契约。工作流按这几个字段解析结论，缺一不可。
	reviewText := ""
	for _, command := range commandList {
		if command.PromptCode == "review" {
			reviewText = command.PromptText
		}
	}
	for _, required := range []string{"verdict_code", "score_value", "issue_list", "level_code", "quote_text", "fix_text"} {
		if !strings.Contains(reviewText, required) {
			t.Fatalf("复审提示词缺少 JSON 字段 %s", required)
		}
	}
	if !strings.Contains(reviewText, "block") {
		t.Fatal("复审提示词缺少判定规则")
	}
}

// TestPromptRetiredChain 覆盖下线清理与身份兜底。
func TestPromptRetiredChain(t *testing.T) {
	codeSet := ai.BuiltinCommandCodeSet()

	// 环节1：现行 8 条任务提示词一律保留。
	for _, command := range ai.BuiltinCommandPrompts() {
		live := &domain.PromptDO{PromptCode: command.PromptCode, PromptKindCode: domain.PromptKindCommand}
		if isRetiredBuiltin(live, codeSet) {
			t.Fatalf("现行任务提示词被误判为下线：%s", command.PromptCode)
		}
	}

	// 环节2：旧身份提示词必须清理，哪怕编码恰好与任务编码相同（种类不同也算下线）。
	for _, retiredCode := range []string{"novelist", "reviser", "planner"} {
		retired := &domain.PromptDO{PromptCode: retiredCode, PromptKindCode: domain.PromptKindSystem}
		if !isRetiredBuiltin(retired, codeSet) {
			t.Fatalf("已下线的内置身份提示词未被清理：%s", retiredCode)
		}
	}
	sameCodeAsCommand := &domain.PromptDO{PromptCode: "polish", PromptKindCode: domain.PromptKindSystem}
	if !isRetiredBuiltin(sameCodeAsCommand, codeSet) {
		t.Fatal("内置身份提示词即使编码与任务同名也必须清理")
	}

	// 环节3：清单外的旧任务编码同样清理（例如历史版本里已删除的任务）。
	staleCommand := &domain.PromptDO{PromptCode: "outline_v0", PromptKindCode: domain.PromptKindCommand}
	if !isRetiredBuiltin(staleCommand, codeSet) {
		t.Fatal("清单外的旧内置任务提示词未被清理")
	}

	// 环节4：身份提示词兜底永远非空。库内没有、查询失败、正文为空，三种情况都回退内置人格。
	if got := pickSystemPromptText(nil, nil); got != ai.BaseSystemPrompt {
		t.Fatalf("无库内身份提示词应回退 BaseSystemPrompt，实际 %q", got)
	}
	if got := pickSystemPromptText(nil, errTestQuery); got != ai.BaseSystemPrompt {
		t.Fatalf("查询失败应回退 BaseSystemPrompt，实际 %q", got)
	}
	empty := &domain.PromptDO{PromptText: "   "}
	if got := pickSystemPromptText(empty, nil); got != ai.BaseSystemPrompt {
		t.Fatalf("空正文应回退 BaseSystemPrompt，实际 %q", got)
	}
	custom := &domain.PromptDO{PromptText: "你是一位冷峻风格的作者。"}
	if got := pickSystemPromptText(custom, nil); got != custom.PromptText {
		t.Fatalf("应使用库内身份提示词，实际 %q", got)
	}

	// 环节5：组合后的系统消息必须同时包含兜底人格与项目文风约束。
	composed := ai.ComposeSystemPrompt("", "冷峻、短句")
	if !strings.Contains(composed, ai.BaseSystemPrompt) || !strings.Contains(composed, "冷峻、短句") {
		t.Fatalf("系统消息组合不完整：%s", composed)
	}

	// 环节6：走真实服务入口验证兜底。库里没有任何身份提示词时 SystemPromptOf 必须返回内置人格。
	capture := &sqlCapture{}
	db, err := gorm.Open(postgres.New(postgres.Config{DSN: "host=127.0.0.1 port=1 user=x dbname=x"}), &gorm.Config{
		DryRun:               true,
		DisableAutomaticPing: true,
		Logger:               logger.New(capture, logger.Config{LogLevel: logger.Info}),
	})
	if err != nil {
		t.Fatalf("打开 DryRun 数据库失败: %v", err)
	}
	promptService := NewPromptService(repo.NewPromptRepo(db), nil)
	if got := promptService.SystemPromptOf(context.Background(), "", ""); got != ai.BaseSystemPrompt {
		t.Fatalf("库内无身份提示词时 SystemPromptOf 应返回 BaseSystemPrompt，实际 %q", got)
	}
	if got := promptService.SystemPromptOf(context.Background(), "", "not_exists"); got != ai.BaseSystemPrompt {
		t.Fatalf("显式编码解析失败时应回退 BaseSystemPrompt，实际 %q", got)
	}
}
