// 本文件实现「开跑就绪度」：把「素材齐不齐」变成一张可勾选的清单，缺什么一眼看到、一键补齐。
// 就绪度只做检查与指路，不代替作者补素材；阻塞项未就绪时前端给出警告但仍允许继续。

package service

import (
	"context"
	"strings"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
)

// 就绪度检查项编码（前端按编码映射图标与分组，值不可随意改）。
const (
	readinessItemProjectSynopsis   = "project_synopsis"   // 项目简介
	readinessItemProjectStyle      = "project_style"      // 文风设定
	readinessItemCharacter         = "character"          // 角色卡
	readinessItemCharacterRelation = "character_relation" // 人物关系
	readinessItemLore              = "lore"               // 世界观设定
	readinessItemOutline           = "outline"            // 大纲节点
	readinessItemChapterOutline    = "chapter_outline"    // 章节要求（大纲章节点/场景卡）
	readinessItemPrompt            = "prompt"             // 提示词
	readinessItemModel             = "model"              // 可用模型
)

// 就绪度阈值（数字与提示文案一一对应，改这里就要同步改 hint）。
const (
	readinessCharacterMin = 3 // 角色卡下限
	readinessRelationMin  = 2 // 人物关系下限
	readinessLoreMin      = 3 // 世界观设定下限
	readinessOutlineMin   = 1 // 大纲节点下限
	readinessChapterMin   = 1 // 章节要求下限
	readinessPromptMin    = 8 // 启用中的提示词下限（覆盖三条流程各自引用的编码）
)

// readinessPromptCodeList 工作流三条模板的核心提示词编码（三条齐全即可开跑）。
var readinessPromptCodeList = []string{"chapter", "polish", "deai"}

// readinessBuilder 就绪度装配中间态：逐项累积，最后统一算分。
type readinessBuilder struct {
	itemList []domain.WorkflowReadinessItemVO // 已装配的检查项
	readyNum int                              // 就绪项数
	blockNum int                              // 未就绪的阻塞项数
}

// add 追加一个检查项并按就绪情况累计计数。
func (b *readinessBuilder) add(item domain.WorkflowReadinessItemVO) {
	if item.ReadyFlag {
		b.readyNum++
	} else if item.BlockingFlag {
		b.blockNum++
	}
	b.itemList = append(b.itemList, item)
}

// Readiness 计算项目开跑就绪度：素材、大纲、提示词、模型四类配置是否够跑一次任务。
func (s *WorkflowService) Readiness(ctx context.Context, userULID, projectULID string) (domain.WorkflowReadinessVO, error) {
	project, err := mustOwnProject(ctx, s.projectRepo, userULID, projectULID)
	if err != nil {
		return domain.WorkflowReadinessVO{}, err
	}
	builder := &readinessBuilder{itemList: make([]domain.WorkflowReadinessItemVO, 0, 9)}
	builder.add(readinessTextItem(readinessItemProjectSynopsis, "项目简介", project.SynopsisText,
		"到工作台在项目卡上点「编辑」，补一句简介（AI 靠它把握全书基调）", "/dashboard"))
	builder.add(readinessTextItem(readinessItemProjectStyle, "文风设定", project.StyleText,
		"在项目编辑里补一句文风要求（如「冷峻短句、少形容词」），产出会更稳", "/dashboard"))

	characterCount, relationCount, err := s.readinessCharacterCounts(ctx, projectULID)
	if err != nil {
		return domain.WorkflowReadinessVO{}, err
	}
	builder.add(readinessCountItem(readinessItemCharacter, "角色卡", characterCount, readinessCharacterMin, true,
		"先到素材库补 "+intToText(int64(readinessCharacterMin))+" 张角色卡（主角、对手、关键配角）",
		readinessRouteOf(projectULID, "character")))
	builder.add(readinessCountItem(readinessItemCharacterRelation, "人物关系", relationCount, readinessRelationMin, false,
		"在角色页补几条人物关系，AI 才不会把两个人的立场写反",
		readinessRouteOf(projectULID, "character")))

	loreCount, err := s.readinessLoreCount(ctx, projectULID)
	if err != nil {
		return domain.WorkflowReadinessVO{}, err
	}
	builder.add(readinessCountItem(readinessItemLore, "世界观设定", loreCount, readinessLoreMin, false,
		"补 "+intToText(int64(readinessLoreMin))+" 条世界观设定（世界规则、地点、关键物品）",
		readinessRouteOf(projectULID, "lore")))

	outlineCount, chapterOutlineCount, err := s.readinessOutlineCounts(ctx, projectULID)
	if err != nil {
		return domain.WorkflowReadinessVO{}, err
	}
	builder.add(readinessCountItem(readinessItemOutline, "大纲节点", outlineCount, readinessOutlineMin, true,
		"先到创作规划搭一个大纲（至少一卷一章），AI 才有落笔位置",
		"/outline/"+projectULID))
	builder.add(readinessCountItem(readinessItemChapterOutline, "章节要求", chapterOutlineCount, readinessChapterMin, false,
		"给要写的章节补一条要求或一张场景卡，产出会贴合你的设想",
		"/outline/"+projectULID))

	promptCount, promptReadyFlag, err := s.readinessPromptCount(ctx, projectULID)
	if err != nil {
		return domain.WorkflowReadinessVO{}, err
	}
	builder.add(domain.WorkflowReadinessItemVO{
		ItemCode:     readinessItemPrompt,
		ItemName:     "提示词",
		ReadyFlag:    promptReadyFlag,
		BlockingFlag: true,
		CurrentValue: intToText(int64(promptCount)) + " 条",
		TargetValue:  "≥" + intToText(int64(readinessPromptMin)) + " 条",
		HintText:     readinessPromptHint(promptReadyFlag),
		FixRoute:     "/prompts",
	})

	modelReadyFlag, modelHintText := s.readinessModelReady(ctx, userULID)
	builder.add(domain.WorkflowReadinessItemVO{
		ItemCode:     readinessItemModel,
		ItemName:     "可用模型",
		ReadyFlag:    modelReadyFlag,
		BlockingFlag: true,
		CurrentValue: readinessBoolText(modelReadyFlag),
		TargetValue:  "至少 1 个",
		HintText:     modelHintText,
		FixRoute:     "/models",
	})

	return domain.WorkflowReadinessVO{
		ProjectULID:   projectULID,
		ScoreValue:    readinessScoreOf(builder.readyNum, len(builder.itemList)),
		ReadyFlag:     builder.blockNum == 0,
		BlockingCount: builder.blockNum,
		ItemList:      builder.itemList,
	}, nil
}

// readinessTextItem 装配「文本非空」类检查项。
func readinessTextItem(itemCode, itemName, textValue, hintText, fixRoute string) domain.WorkflowReadinessItemVO {
	readyFlag := strings.TrimSpace(textValue) != ""
	return domain.WorkflowReadinessItemVO{
		ItemCode:     itemCode,
		ItemName:     itemName,
		ReadyFlag:    readyFlag,
		BlockingFlag: false,
		CurrentValue: readinessBoolText(readyFlag),
		TargetValue:  "已填写",
		HintText:     readinessHintOf(readyFlag, hintText),
		FixRoute:     fixRoute,
	}
}

// readinessCountItem 装配「数量达标」类检查项。
func readinessCountItem(itemCode, itemName string, currentCount, targetCount int, blockingFlag bool,
	hintText, fixRoute string) domain.WorkflowReadinessItemVO {
	readyFlag := currentCount >= targetCount
	return domain.WorkflowReadinessItemVO{
		ItemCode:     itemCode,
		ItemName:     itemName,
		ReadyFlag:    readyFlag,
		BlockingFlag: blockingFlag,
		CurrentValue: intToText(int64(currentCount)) + " 条",
		TargetValue:  "≥" + intToText(int64(targetCount)) + " 条",
		HintText:     readinessHintOf(readyFlag, hintText),
		FixRoute:     fixRoute,
	}
}

// readinessHintOf 就绪时给确认文案，未就绪时给补齐建议（前端只在未就绪项显示按钮）。
func readinessHintOf(readyFlag bool, hintText string) string {
	if readyFlag {
		return "已就绪"
	}
	return hintText
}

// readinessBoolText 布尔值转「已填写 / 未填写」。
func readinessBoolText(flag bool) string {
	if flag {
		return "已就绪"
	}
	return "未就绪"
}

// readinessPromptHint 提示词缺失时指出缺哪个编码（编码比数量更能指导补齐）。
func readinessPromptHint(readyFlag bool) string {
	if readyFlag {
		return "已就绪"
	}
	return "到提示词库补齐 chapter / polish / deai 三个编码，并确认已启用"
}

// readinessCharacterCounts 统计角色卡与人物关系数量。
func (s *WorkflowService) readinessCharacterCounts(ctx context.Context, projectULID string) (int, int, error) {
	characters, err := s.characterRepo.ListByProject(ctx, projectULID, "", "")
	if err != nil {
		return 0, 0, err
	}
	relations, err := s.characterRepo.ListRelationsByProject(ctx, projectULID)
	if err != nil {
		return 0, 0, err
	}
	return len(characters), len(relations), nil
}

// readinessLoreCount 统计世界观设定条数（创作笔记 note 不计入）。
func (s *WorkflowService) readinessLoreCount(ctx context.Context, projectULID string) (int, error) {
	loreList, err := s.loreRepo.ListByProject(ctx, projectULID, "", "")
	if err != nil {
		return 0, err
	}
	count := 0
	for i := range loreList {
		if loreList[i].LoreTypeCode == "note" {
			continue
		}
		count++
	}
	return count, nil
}

// readinessOutlineCounts 统计大纲节点数与「章节要求」数（章节节点与场景卡合起来算）。
func (s *WorkflowService) readinessOutlineCounts(ctx context.Context, projectULID string) (int, int, error) {
	nodeList, err := s.outlineRepo.ListNodesByProject(ctx, projectULID)
	if err != nil {
		return 0, 0, err
	}
	sceneList, err := s.outlineRepo.ListScenes(ctx, projectULID, "")
	if err != nil {
		return 0, 0, err
	}
	chapterCount := len(sceneList)
	for i := range nodeList {
		if nodeList[i].NodeTypeCode == "chapter" {
			chapterCount++
		}
	}
	return len(nodeList), chapterCount, nil
}

// readinessPromptCount 统计启用中的提示词：数量达标或关键编码齐备即视为就绪。
func (s *WorkflowService) readinessPromptCount(ctx context.Context, projectULID string) (int, bool, error) {
	promptList, err := s.promptRepo.List(ctx, &domain.PromptListREQ{ProjectULID: projectULID, EnabledOnly: true})
	if err != nil {
		return 0, false, err
	}
	codeSet := make(map[string]bool, len(promptList))
	for i := range promptList {
		codeSet[promptList[i].PromptCode] = true
	}
	// 关键编码是硬约束：缺一个编码，对应模板的某一步就取不到正文。
	allCodeReady := true
	for _, promptCode := range readinessPromptCodeList {
		if !codeSet[promptCode] {
			allCodeReady = false
			break
		}
	}
	return len(promptList), allCodeReady || len(promptList) >= readinessPromptMin, nil
}

// readinessModelReady 判断当前用户是否有可用模型（与执行引擎同一套授权判定）。
func (s *WorkflowService) readinessModelReady(ctx context.Context, userULID string) (bool, string) {
	capability, err := s.providerSvc.Capability(ctx, userULID)
	if err != nil {
		return false, "模型目录读取失败，请稍后重试或联系管理员"
	}
	if capability.ChatReady {
		return true, "已就绪"
	}
	if strings.TrimSpace(capability.HintText) != "" {
		return false, capability.HintText
	}
	return false, "当前账号还没有可用模型，请联系管理员分配"
}

// readinessRouteOf 拼接项目作用域的前端路由（素材类页面统一走 /material/{项目}?tab=xxx）。
func readinessRouteOf(projectULID, tabCode string) string {
	return "/material/" + projectULID + "?tab=" + tabCode
}

// readinessScoreOf 计算就绪项占比（0-100，四舍五入到整数）。
func readinessScoreOf(readyNum, totalCount int) int {
	if totalCount <= 0 {
		return 0
	}
	return readyNum * 100 / totalCount
}

// ListPendingReview 待审核收件箱：当前用户停在审核门禁的运行，最多 10 条。
func (s *WorkflowService) ListPendingReview(ctx context.Context, userULID string) ([]domain.WorkflowRunVO, error) {
	dos, err := s.workflowRepo.ListPendingReviewRuns(ctx, userULID, workflowPendingReviewLimit)
	if err != nil {
		return nil, err
	}
	projectNameMap, err := s.projectNameMap(ctx, dos)
	if err != nil {
		return nil, err
	}
	chapterNameMap, err := s.chapterNameMap(ctx, dos)
	if err != nil {
		return nil, err
	}
	vos := make([]domain.WorkflowRunVO, 0, len(dos))
	for i := range dos {
		currentChapterULID := currentChapterULIDOfRun(&dos[i])
		vos = append(vos, toWorkflowRunVO(&dos[i], projectNameMap[dos[i].ProjectULID], chapterNameMap[currentChapterULID]))
	}
	return vos, nil
}

// 收件箱条数上限（Dashboard 只展示「现在最该处理的那几件」）。
const workflowPendingReviewLimit = 10
