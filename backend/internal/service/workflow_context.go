// 本文件实现工作流章节上下文装配：把项目素材、人物关系、世界观、大纲与场景卡拼进提示词。
// 每块独立取数、独立限长，取不到就跳过——缺素材不该让整次运行失败。

package service

import (
	"context"
	"strings"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
)

// 上下文各块的限长约束（合计约 3000 字，正文另受 workflowChapterMaxRunes 控制）。
const (
	workflowContextRelationMax    = 20   // 人物关系条数上限
	workflowContextRelationRunes  = 40   // 每条关系描述上限
	workflowContextLoreMax        = 8    // 世界观设定条数上限
	workflowContextLoreRunes      = 120  // 每条设定上限
	workflowContextSceneMax       = 3    // 场景卡条数上限
	workflowContextSceneRunes     = 150  // 每条场景卡上限
	workflowContextChapterRunes   = 600  // 章节要求（大纲节点概要）上限
	workflowContextSummaryRunes   = 300  // 前情摘要每条上限
	workflowContextSummaryMax     = 5    // 前情摘要条数上限
	workflowContextRequirementMax = 1000 // 项目整体要求上限（与参数落库一致）
	workflowContextTotalMaxRunes  = 3000 // 除正文外各块合计上限（超了保留首尾）
)

// chapterContextText 装配章节上下文：项目设定、人物关系、世界观、章节要求、前情摘要与伏笔。
func (s *WorkflowService) chapterContextText(ctx context.Context, chapter *domain.ChapterDO,
	params domain.WorkflowRunParamsDTO) (string, error) {
	project, err := s.projectRepo.GetByULID(ctx, chapter.ProjectULID)
	if err != nil {
		return "", err
	}
	var builder strings.Builder
	if project != nil {
		if project.SynopsisText != "" {
			builder.WriteString("【项目简介】\n" + project.SynopsisText + "\n\n")
		}
		if project.StyleText != "" {
			builder.WriteString("【文风要求】\n" + project.StyleText + "\n\n")
		}
	}
	if requirementText := strings.TrimSpace(params.ExtraRequirementText); requirementText != "" {
		builder.WriteString("【项目整体要求】\n" + truncateRunes(requirementText, workflowContextRequirementMax) + "\n\n")
	}
	builder.WriteString("【本章】第 " + intToText(int64(chapter.ChapterOrder)) + " 章 " + chapter.ChapterName + "\n")
	if chapter.SummaryText != "" {
		builder.WriteString("章节梗概：" + chapter.SummaryText + "\n")
	}
	s.appendRelationBlock(ctx, &builder, chapter.ProjectULID)
	s.appendLoreBlock(ctx, &builder, chapter.ProjectULID)
	s.appendChapterRequirementBlock(ctx, &builder, chapter)
	s.appendSummaryBlock(ctx, &builder, chapter)
	hookText, err := s.pendingHookText(ctx, chapter.ProjectULID)
	if err == nil && hookText != "" {
		builder.WriteString("\n" + hookText)
	}
	return condenseContextText(builder.String()), nil
}

// condenseContextText 收敛除正文外的上下文总长：超出上限时保留首尾、标注省略字数。
// 各块已单独限长，这里只兜总量，避免素材多的项目把提示词顶爆。
func condenseContextText(contextText string) string {
	trimmed := strings.TrimSpace(contextText)
	runes := []rune(trimmed)
	if len(runes) <= workflowContextTotalMaxRunes {
		return trimmed
	}
	headCount := workflowContextTotalMaxRunes * 3 / 5
	tailCount := workflowContextTotalMaxRunes * 2 / 5
	omittedCount := len(runes) - headCount - tailCount
	return string(runes[:headCount]) + "\n\n……（上下文过长，中间省略 " +
		intToText(int64(omittedCount)) + " 字）……\n\n" + string(runes[len(runes)-tailCount:])
}

// appendRelationBlock 追加人物关系摘要（谁—关系—谁），最多 20 条。
func (s *WorkflowService) appendRelationBlock(ctx context.Context, builder *strings.Builder, projectULID string) {
	relationList, err := s.characterRepo.ListRelationsByProject(ctx, projectULID)
	if err != nil || len(relationList) == 0 {
		return
	}
	characterList, err := s.characterRepo.ListByProject(ctx, projectULID, "", "")
	if err != nil {
		return
	}
	nameMap := make(map[string]string, len(characterList))
	for i := range characterList {
		nameMap[characterList[i].CharacterULID] = characterList[i].CharacterName
	}
	var relationBuilder strings.Builder
	count := 0
	for i := range relationList {
		fromName := nameMap[relationList[i].FromCharacterULID]
		toName := nameMap[relationList[i].ToCharacterULID]
		if fromName == "" || toName == "" {
			continue // 关系边指向已删除角色：跳过，不写「某某—未知」
		}
		lineText := fromName + "—" + relationTypeTextOf(relationList[i].RelationTypeCode)
		if noteText := strings.TrimSpace(relationList[i].RelationNote); noteText != "" {
			lineText += "（" + noteText + "）"
		}
		lineText += "—" + toName
		relationBuilder.WriteString("- " + truncateRunes(lineText, workflowContextRelationRunes) + "\n")
		count++
		if count >= workflowContextRelationMax {
			break
		}
	}
	if count > 0 {
		builder.WriteString("\n【人物关系】\n" + relationBuilder.String())
	}
}

// appendLoreBlock 追加世界观设定（按类型分组，最多 8 条，跳过创作笔记）。
func (s *WorkflowService) appendLoreBlock(ctx context.Context, builder *strings.Builder, projectULID string) {
	loreList, err := s.loreRepo.ListByProject(ctx, projectULID, "", "")
	if err != nil || len(loreList) == 0 {
		return
	}
	var loreBuilder strings.Builder
	count := 0
	for i := range loreList {
		if loreList[i].LoreTypeCode == "note" {
			continue // 创作笔记是作者自用，不进提示词
		}
		contentText := stripHTMLTags(loreList[i].ContentText)
		if strings.TrimSpace(contentText) == "" {
			continue
		}
		lineText := "[" + loreTypeTextOf(loreList[i].LoreTypeCode) + "] " + loreList[i].LoreName +
			"：" + truncateRunes(contentText, workflowContextLoreRunes)
		loreBuilder.WriteString("- " + lineText + "\n")
		count++
		if count >= workflowContextLoreMax {
			break
		}
	}
	if count > 0 {
		builder.WriteString("\n【世界观设定】\n" + loreBuilder.String())
	}
}

// appendChapterRequirementBlock 追加章节要求：本章大纲节点（匹配不到取所属卷纲）与场景卡。
func (s *WorkflowService) appendChapterRequirementBlock(ctx context.Context, builder *strings.Builder,
	chapter *domain.ChapterDO) {
	nodeList, err := s.outlineRepo.ListNodesByProject(ctx, chapter.ProjectULID)
	if err != nil {
		return
	}
	chapterNode := matchChapterOutlineNode(nodeList, chapter)
	requirementText := ""
	if chapterNode != nil {
		requirementText = outlineNodeRequirementText(chapterNode)
	}
	if requirementText == "" {
		// 章节点缺失时退到所属卷纲：卷纲至少说明这一卷要干什么，比空着强。
		if volumeNode := findVolumeOutlineNode(nodeList, chapterNode); volumeNode != nil {
			requirementText = "（本章未单独规划，以下是所属卷的卷纲）\n" + outlineNodeRequirementText(volumeNode)
		}
	}
	if requirementText != "" {
		builder.WriteString("\n【章节要求】\n" + truncateRunes(requirementText, workflowContextChapterRunes) + "\n")
	}
	s.appendSceneBlock(ctx, builder, chapter, chapterNode)
}

// appendSceneBlock 追加场景卡摘要（最多 3 条）。
func (s *WorkflowService) appendSceneBlock(ctx context.Context, builder *strings.Builder,
	chapter *domain.ChapterDO, chapterNode *domain.OutlineNodeDO) {
	sceneList, err := s.outlineRepo.ListScenes(ctx, chapter.ProjectULID, "")
	if err != nil || len(sceneList) == 0 {
		return
	}
	var sceneBuilder strings.Builder
	count := 0
	for i := range sceneList {
		// 有章节点时只取挂在该节点下的场景卡；没有章节点时取全项目前几张。
		if chapterNode != nil && sceneList[i].OutlineULID != chapterNode.OutlineULID {
			continue
		}
		lineText := sceneSummaryTextOf(&sceneList[i])
		if lineText == "" {
			continue
		}
		sceneBuilder.WriteString("- " + truncateRunes(lineText, workflowContextSceneRunes) + "\n")
		count++
		if count >= workflowContextSceneMax {
			break
		}
	}
	if count > 0 {
		builder.WriteString("\n【场景卡】\n" + sceneBuilder.String())
	}
}

// appendSummaryBlock 追加前情摘要（其他章节，最多 5 条）。
func (s *WorkflowService) appendSummaryBlock(ctx context.Context, builder *strings.Builder, chapter *domain.ChapterDO) {
	summaries, err := s.continuityRepo.ListSummariesByProject(ctx, chapter.ProjectULID, "", "")
	if err != nil || len(summaries) == 0 {
		return
	}
	var summaryBuilder strings.Builder
	appended := 0
	for i := range summaries {
		if summaries[i].ChapterULID == chapter.ChapterULID {
			continue
		}
		summaryBuilder.WriteString("- " + truncateRunes(summaries[i].SummaryText, workflowContextSummaryRunes) + "\n")
		appended++
		if appended >= workflowContextSummaryMax {
			break
		}
	}
	if appended > 0 {
		builder.WriteString("\n【前情摘要】\n" + summaryBuilder.String())
	}
}

// matchChapterOutlineNode 按章节名匹配大纲章节点（名称一致优先，其次按序号在同级章节点里取）。
func matchChapterOutlineNode(nodeList []domain.OutlineNodeDO, chapter *domain.ChapterDO) *domain.OutlineNodeDO {
	chapterNodeList := make([]*domain.OutlineNodeDO, 0, len(nodeList))
	for i := range nodeList {
		if nodeList[i].NodeTypeCode == "chapter" {
			chapterNodeList = append(chapterNodeList, &nodeList[i])
		}
	}
	if len(chapterNodeList) == 0 {
		return nil
	}
	chapterName := strings.TrimSpace(chapter.ChapterName)
	if chapterName != "" {
		for _, node := range chapterNodeList {
			if strings.TrimSpace(node.TitleText) == chapterName {
				return node
			}
		}
		for _, node := range chapterNodeList {
			if chapterName != "" && (strings.Contains(node.TitleText, chapterName) ||
				strings.Contains(chapterName, node.TitleText)) {
				return node
			}
		}
	}
	// 名称匹配不上时按排序号兜底：章节点通常按目录顺序排列，用章节序号定位。
	if chapter.ChapterOrder > 0 && chapter.ChapterOrder <= len(chapterNodeList) {
		return chapterNodeList[chapter.ChapterOrder-1]
	}
	return nil
}

// findVolumeOutlineNode 取章节点所属卷（章节点缺失时按卷排序号兜底到第一卷）。
func findVolumeOutlineNode(nodeList []domain.OutlineNodeDO, chapterNode *domain.OutlineNodeDO) *domain.OutlineNodeDO {
	var volumeNodeList []*domain.OutlineNodeDO
	for i := range nodeList {
		if nodeList[i].NodeTypeCode == "volume" {
			volumeNodeList = append(volumeNodeList, &nodeList[i])
		}
	}
	if len(volumeNodeList) == 0 {
		return nil
	}
	if chapterNode != nil && chapterNode.ParentOutlineULID != "" {
		for _, node := range volumeNodeList {
			if node.OutlineULID == chapterNode.ParentOutlineULID {
				return node
			}
		}
	}
	return volumeNodeList[0]
}

// outlineNodeRequirementText 拼节点标题与概要（两者都空返回空串，调用方据此跳过该块）。
func outlineNodeRequirementText(node *domain.OutlineNodeDO) string {
	titleText := strings.TrimSpace(node.TitleText)
	summaryText := strings.TrimSpace(node.SummaryText)
	switch {
	case titleText == "" && summaryText == "":
		return ""
	case summaryText == "":
		return titleText
	case titleText == "":
		return summaryText
	default:
		return titleText + "：" + summaryText
	}
}

// sceneSummaryTextOf 拼一张场景卡的可读摘要（地点 / 冲突 / 目标 / 结果）。
func sceneSummaryTextOf(scene *domain.DetailedSceneDO) string {
	partList := make([]string, 0, 4)
	if text := strings.TrimSpace(scene.LocationText); text != "" {
		partList = append(partList, "地点："+text)
	}
	if text := strings.TrimSpace(scene.ConflictText); text != "" {
		partList = append(partList, "冲突："+text)
	}
	if text := strings.TrimSpace(scene.GoalText); text != "" {
		partList = append(partList, "目标："+text)
	}
	if text := strings.TrimSpace(scene.OutcomeText); text != "" {
		partList = append(partList, "结果："+text)
	}
	return strings.Join(partList, "；")
}

// relationTypeTextOf 关系类型编码转中文（未知编码原样返回，不丢信息）。
func relationTypeTextOf(relationTypeCode string) string {
	switch relationTypeCode {
	case "friend":
		return "朋友"
	case "enemy":
		return "敌对"
	case "lover":
		return "恋人"
	case "kin":
		return "亲属"
	case "teacher":
		return "师徒"
	case "colleague":
		return "同僚"
	case "rival":
		return "竞争"
	default:
		if relationTypeCode == "" {
			return "关联"
		}
		return relationTypeCode
	}
}

// loreTypeTextOf 设定类型编码转中文（未知编码原样返回）。
func loreTypeTextOf(loreTypeCode string) string {
	switch loreTypeCode {
	case "world":
		return "世界"
	case "rule":
		return "规则"
	case "item":
		return "物品"
	case "place":
		return "地点"
	case "event":
		return "事件"
	default:
		if loreTypeCode == "" {
			return "设定"
		}
		return loreTypeCode
	}
}
