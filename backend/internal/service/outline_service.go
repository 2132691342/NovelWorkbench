package service

import (
	"context"
	"encoding/json"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
	"github.com/2132691342/NovelWorkbench/backend/internal/repo"
)

// OutlineService 创作规划业务：大纲节点树与详细场景卡片管理。
type OutlineService struct {
	outlineRepo *repo.OutlineRepo
	projectRepo *repo.ProjectRepo
}

// NewOutlineService 构造创作规划服务。
func NewOutlineService(outlineRepo *repo.OutlineRepo, projectRepo *repo.ProjectRepo) *OutlineService {
	return &OutlineService{outlineRepo: outlineRepo, projectRepo: projectRepo}
}

// SaveNode 新建/更新大纲节点（有 outline_ulid 为更新），校验项目归属与父节点归属。
func (s *OutlineService) SaveNode(ctx context.Context, userULID string, req *domain.OutlineNodeREQ) (domain.OutlineNodeVO, error) {
	if _, err := mustOwnProject(ctx, s.projectRepo, userULID, req.ProjectULID); err != nil {
		return domain.OutlineNodeVO{}, err
	}
	statusCode := req.NodeStatusCode
	if statusCode == "" {
		statusCode = "draft"
	}
	if req.OutlineULID == "" {
		sortOrder := req.SortOrder
		if sortOrder == 0 {
			maxOrder, err := s.outlineRepo.MaxNodeOrder(ctx, req.ProjectULID, req.ParentOutlineULID)
			if err != nil {
				return domain.OutlineNodeVO{}, err
			}
			sortOrder = maxOrder + 1
		}
		do := &domain.OutlineNodeDO{
			OutlineULID:       pkg.NewULID(domain.IDPrefixOutline),
			ProjectULID:       req.ProjectULID,
			ParentOutlineULID: req.ParentOutlineULID,
			NodeTypeCode:      req.NodeTypeCode,
			TitleText:         req.TitleText,
			SummaryText:       req.SummaryText,
			NodeStatusCode:    statusCode,
			SortOrder:         sortOrder,
		}
		if err := s.validateParent(ctx, req.ProjectULID, req.ParentOutlineULID, ""); err != nil {
			return domain.OutlineNodeVO{}, err
		}
		if err := s.outlineRepo.CreateNode(ctx, do); err != nil {
			return domain.OutlineNodeVO{}, err
		}
		return toOutlineNodeVO(do), nil
	}
	existing, err := s.loadOwnedNode(ctx, req.ProjectULID, req.OutlineULID)
	if err != nil {
		return domain.OutlineNodeVO{}, err
	}
	if err := s.validateParent(ctx, req.ProjectULID, req.ParentOutlineULID, existing.OutlineULID); err != nil {
		return domain.OutlineNodeVO{}, err
	}
	fields := map[string]any{
		"parent_outline_ulid": req.ParentOutlineULID,
		"node_type_code":      req.NodeTypeCode,
		"title_text":          req.TitleText,
		"summary_text":        req.SummaryText,
		"node_status_code":    statusCode,
		"sort_order":          req.SortOrder,
	}
	if err := s.outlineRepo.UpdateNode(ctx, req.ProjectULID, req.OutlineULID, fields); err != nil {
		return domain.OutlineNodeVO{}, err
	}
	updated, err := s.outlineRepo.GetNodeByULID(ctx, req.ProjectULID, req.OutlineULID)
	if err != nil {
		return domain.OutlineNodeVO{}, err
	}
	return toOutlineNodeVO(updated), nil
}

// DeleteNode 删除大纲节点；存在子节点时拒绝删除（避免形成孤儿树）。
func (s *OutlineService) DeleteNode(ctx context.Context, userULID string, req *domain.OutlineNodeDeleteREQ) error {
	if _, err := mustOwnProject(ctx, s.projectRepo, userULID, req.ProjectULID); err != nil {
		return err
	}
	if _, err := s.loadOwnedNode(ctx, req.ProjectULID, req.OutlineULID); err != nil {
		return err
	}
	childCount, err := s.outlineRepo.CountNodesByParent(ctx, req.ProjectULID, req.OutlineULID)
	if err != nil {
		return err
	}
	if childCount > 0 {
		return pkg.NewBizError(domain.RespCodeConflict, "该节点下还有子节点，请先删除子节点")
	}
	return s.outlineRepo.DeleteNode(ctx, req.ProjectULID, req.OutlineULID)
}

// ListTree 查询项目大纲树（节点按排序号升序，递归组装父子关系）。
func (s *OutlineService) ListTree(ctx context.Context, userULID string, req *domain.OutlineTreeREQ) ([]domain.OutlineNodeVO, error) {
	if _, err := mustOwnProject(ctx, s.projectRepo, userULID, req.ProjectULID); err != nil {
		return nil, err
	}
	dos, err := s.outlineRepo.ListNodesByProject(ctx, req.ProjectULID)
	if err != nil {
		return nil, err
	}
	childrenMap := make(map[string][]domain.OutlineNodeVO, len(dos))
	roots := make([]domain.OutlineNodeVO, 0)
	for index := range dos {
		vo := toOutlineNodeVO(&dos[index])
		if vo.ParentOutlineULID == "" {
			roots = append(roots, vo)
			continue
		}
		childrenMap[vo.ParentOutlineULID] = append(childrenMap[vo.ParentOutlineULID], vo)
	}
	return attachOutlineChildren(roots, childrenMap), nil
}

// SaveScene 新建/更新详细场景卡片，校验项目与所属节点归属。
func (s *OutlineService) SaveScene(ctx context.Context, userULID string, req *domain.DetailedSceneREQ) (domain.DetailedSceneVO, error) {
	if _, err := mustOwnProject(ctx, s.projectRepo, userULID, req.ProjectULID); err != nil {
		return domain.DetailedSceneVO{}, err
	}
	if _, err := s.loadOwnedNode(ctx, req.ProjectULID, req.OutlineULID); err != nil {
		return domain.DetailedSceneVO{}, err
	}
	characterJSON, err := encodeCharacterULIDs(req.CharacterULIDList)
	if err != nil {
		return domain.DetailedSceneVO{}, err
	}
	if req.SceneULID == "" {
		sortOrder := req.SortOrder
		if sortOrder == 0 {
			maxOrder, err := s.outlineRepo.MaxSceneOrder(ctx, req.ProjectULID, req.OutlineULID)
			if err != nil {
				return domain.DetailedSceneVO{}, err
			}
			sortOrder = maxOrder + 1
		}
		do := &domain.DetailedSceneDO{
			SceneULID:         pkg.NewULID(domain.IDPrefixScene),
			ProjectULID:       req.ProjectULID,
			OutlineULID:       req.OutlineULID,
			CharacterULIDList: characterJSON,
			LocationText:      req.LocationText,
			ConflictText:      req.ConflictText,
			PaceCode:          req.PaceCode,
			EstimatedWords:    req.EstimatedWords,
			EntryStateText:    req.EntryStateText,
			GoalText:          req.GoalText,
			KeyChoiceText:     req.KeyChoiceText,
			OutcomeText:       req.OutcomeText,
			ExitStateText:     req.ExitStateText,
			SortOrder:         sortOrder,
		}
		if err := s.outlineRepo.CreateScene(ctx, do); err != nil {
			return domain.DetailedSceneVO{}, err
		}
		return toDetailedSceneVO(do)
	}
	if _, err := s.loadOwnedScene(ctx, req.ProjectULID, req.SceneULID); err != nil {
		return domain.DetailedSceneVO{}, err
	}
	fields := map[string]any{
		"outline_ulid":        req.OutlineULID,
		"character_ulid_list": characterJSON,
		"location_text":       req.LocationText,
		"conflict_text":       req.ConflictText,
		"pace_code":           req.PaceCode,
		"estimated_words":     req.EstimatedWords,
		"entry_state_text":    req.EntryStateText,
		"goal_text":           req.GoalText,
		"key_choice_text":     req.KeyChoiceText,
		"outcome_text":        req.OutcomeText,
		"exit_state_text":     req.ExitStateText,
		"sort_order":          req.SortOrder,
	}
	if err := s.outlineRepo.UpdateScene(ctx, req.ProjectULID, req.SceneULID, fields); err != nil {
		return domain.DetailedSceneVO{}, err
	}
	updated, err := s.outlineRepo.GetSceneByULID(ctx, req.ProjectULID, req.SceneULID)
	if err != nil {
		return domain.DetailedSceneVO{}, err
	}
	return toDetailedSceneVO(updated)
}

// DeleteScene 删除详细场景卡片。
func (s *OutlineService) DeleteScene(ctx context.Context, userULID string, req *domain.DetailedSceneDeleteREQ) error {
	if _, err := mustOwnProject(ctx, s.projectRepo, userULID, req.ProjectULID); err != nil {
		return err
	}
	if _, err := s.loadOwnedScene(ctx, req.ProjectULID, req.SceneULID); err != nil {
		return err
	}
	return s.outlineRepo.DeleteScene(ctx, req.ProjectULID, req.SceneULID)
}

// ListScenes 查询场景列表；可按节点过滤（outline_ulid 为空则查项目全部场景）。
func (s *OutlineService) ListScenes(ctx context.Context, userULID string, req *domain.DetailedSceneListREQ) ([]domain.DetailedSceneVO, error) {
	if _, err := mustOwnProject(ctx, s.projectRepo, userULID, req.ProjectULID); err != nil {
		return nil, err
	}
	if req.OutlineULID != "" {
		if _, err := s.loadOwnedNode(ctx, req.ProjectULID, req.OutlineULID); err != nil {
			return nil, err
		}
	}
	dos, err := s.outlineRepo.ListScenes(ctx, req.ProjectULID, req.OutlineULID)
	if err != nil {
		return nil, err
	}
	vos := make([]domain.DetailedSceneVO, 0, len(dos))
	for index := range dos {
		vo, err := toDetailedSceneVO(&dos[index])
		if err != nil {
			return nil, err
		}
		vos = append(vos, vo)
	}
	return vos, nil
}

// loadOwnedNode 按项目 ULID 加载大纲节点，不存在或越权时返回业务错误。
func (s *OutlineService) loadOwnedNode(ctx context.Context, projectULID, outlineULID string) (*domain.OutlineNodeDO, error) {
	node, err := s.outlineRepo.GetNodeByULID(ctx, projectULID, outlineULID)
	if err != nil {
		return nil, err
	}
	if node == nil {
		return nil, pkg.NewBizError(domain.RespCodeNotFound, "大纲节点不存在")
	}
	if node.ProjectULID != projectULID {
		return nil, pkg.NewBizError(domain.RespCodeForbidden, "无权操作该大纲节点")
	}
	return node, nil
}

// loadOwnedScene 按项目 ULID 加载场景卡片，不存在或越权时返回业务错误。
func (s *OutlineService) loadOwnedScene(ctx context.Context, projectULID, sceneULID string) (*domain.DetailedSceneDO, error) {
	scene, err := s.outlineRepo.GetSceneByULID(ctx, projectULID, sceneULID)
	if err != nil {
		return nil, err
	}
	if scene == nil {
		return nil, pkg.NewBizError(domain.RespCodeNotFound, "场景卡片不存在")
	}
	if scene.ProjectULID != projectULID {
		return nil, pkg.NewBizError(domain.RespCodeForbidden, "无权操作该场景卡片")
	}
	return scene, nil
}

// validateParent 校验父节点归属并阻止把节点挂到自身或后代上。
func (s *OutlineService) validateParent(ctx context.Context, projectULID, parentOutlineULID, selfOutlineULID string) error {
	if parentOutlineULID == "" {
		return nil
	}
	if parentOutlineULID == selfOutlineULID {
		return pkg.NewBizError(domain.RespCodeInvalidParam, "父节点不能是自身")
	}
	if _, err := s.loadOwnedNode(ctx, projectULID, parentOutlineULID); err != nil {
		return err
	}
	// 向上回溯父链，发现当前节点即说明会构成环。
	current := parentOutlineULID
	for current != "" {
		node, err := s.loadOwnedNode(ctx, projectULID, current)
		if err != nil {
			return err
		}
		if selfOutlineULID != "" && node.ParentOutlineULID == selfOutlineULID {
			return pkg.NewBizError(domain.RespCodeInvalidParam, "父节点不能是自身的子节点")
		}
		current = node.ParentOutlineULID
	}
	return nil
}

// encodeCharacterULIDs 将角色 ULID 数组编码为 JSON 字符串（空数组存 []）。
func encodeCharacterULIDs(characterULIDs []string) (string, error) {
	if len(characterULIDs) == 0 {
		return "[]", nil
	}
	encoded, err := json.Marshal(characterULIDs)
	if err != nil {
		return "", pkg.WrapBizError(domain.RespCodeInvalidParam, "角色 ULID 列表编码失败", err)
	}
	return string(encoded), nil
}

// attachOutlineChildren 递归挂载子节点并按排序号返回子树。
func attachOutlineChildren(parents []domain.OutlineNodeVO, childrenMap map[string][]domain.OutlineNodeVO) []domain.OutlineNodeVO {
	result := make([]domain.OutlineNodeVO, 0, len(parents))
	for _, parent := range parents {
		parent.Children = attachOutlineChildren(childrenMap[parent.OutlineULID], childrenMap)
		result = append(result, parent)
	}
	return result
}

// toOutlineNodeVO 将大纲节点持久化对象转换为视图对象。
func toOutlineNodeVO(do *domain.OutlineNodeDO) domain.OutlineNodeVO {
	return domain.OutlineNodeVO{
		OutlineULID:       do.OutlineULID,
		ProjectULID:       do.ProjectULID,
		ParentOutlineULID: do.ParentOutlineULID,
		NodeTypeCode:      do.NodeTypeCode,
		TitleText:         do.TitleText,
		SummaryText:       do.SummaryText,
		NodeStatusCode:    do.NodeStatusCode,
		SortOrder:         do.SortOrder,
		UpdatedAtMs:       do.UpdatedAt.UnixMilli(),
	}
}

// toDetailedSceneVO 将场景持久化对象转换为视图对象（解析角色 ULID JSON 数组）。
func toDetailedSceneVO(do *domain.DetailedSceneDO) (domain.DetailedSceneVO, error) {
	var characterULIDs []string
	if do.CharacterULIDList != "" {
		if err := json.Unmarshal([]byte(do.CharacterULIDList), &characterULIDs); err != nil {
			return domain.DetailedSceneVO{}, pkg.WrapBizError(domain.RespCodeInternalError, "角色 ULID 列表解析失败", err)
		}
	}
	if characterULIDs == nil {
		characterULIDs = []string{}
	}
	return domain.DetailedSceneVO{
		SceneULID:         do.SceneULID,
		ProjectULID:       do.ProjectULID,
		OutlineULID:       do.OutlineULID,
		CharacterULIDList: characterULIDs,
		LocationText:      do.LocationText,
		ConflictText:      do.ConflictText,
		PaceCode:          do.PaceCode,
		EstimatedWords:    do.EstimatedWords,
		EntryStateText:    do.EntryStateText,
		GoalText:          do.GoalText,
		KeyChoiceText:     do.KeyChoiceText,
		OutcomeText:       do.OutcomeText,
		ExitStateText:     do.ExitStateText,
		SortOrder:         do.SortOrder,
		UpdatedAtMs:       do.UpdatedAt.UnixMilli(),
	}, nil
}
