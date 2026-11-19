// 本文件实现角色业务：角色卡 CRUD、关系图谱与关系边维护。

package service

import (
	"context"

	"gorm.io/gorm"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
	"github.com/2132691342/NovelWorkbench/backend/internal/repo"
)

// CharacterService 角色业务：角色卡 CRUD、关系图谱与关系边维护。
type CharacterService struct {
	characterRepo *repo.CharacterRepo // 角色仓储
	projectRepo   *repo.ProjectRepo   // 项目仓储（归属校验）
	db            *gorm.DB            // 事务入口
}

// NewCharacterService 构造角色服务。
func NewCharacterService(characterRepo *repo.CharacterRepo, projectRepo *repo.ProjectRepo, db *gorm.DB) *CharacterService {
	return &CharacterService{characterRepo: characterRepo, projectRepo: projectRepo, db: db}
}

// Save 新建/更新角色（有 character_ulid 为更新）。
func (s *CharacterService) Save(ctx context.Context, userULID string, req *domain.CharacterREQ) (domain.CharacterVO, error) {
	if _, err := mustOwnProject(ctx, s.projectRepo, userULID, req.ProjectULID); err != nil {
		return domain.CharacterVO{}, err
	}
	if req.CharacterULID == "" {
		do := &domain.CharacterDO{
			CharacterULID:   pkg.NewULID(domain.IDPrefixCharacter),
			ProjectULID:     req.ProjectULID,
			CharacterName:   req.CharacterName,
			AliasText:       req.AliasText,
			RoleTypeCode:    req.RoleTypeCode,
			PersonalityText: req.PersonalityText,
			BackgroundText:  req.BackgroundText,
			AppearanceText:  req.AppearanceText,
			GoalText:        req.GoalText,
		}
		if err := s.characterRepo.Create(ctx, do); err != nil {
			return domain.CharacterVO{}, err
		}
		return toCharacterVO(do), nil
	}
	existing, err := s.characterRepo.GetByULID(ctx, req.CharacterULID)
	if err != nil {
		return domain.CharacterVO{}, err
	}
	if existing == nil {
		return domain.CharacterVO{}, pkg.NewBizError(domain.RespCodeNotFound, "角色不存在")
	}
	if existing.ProjectULID != req.ProjectULID {
		return domain.CharacterVO{}, pkg.NewBizError(domain.RespCodeInvalidParam, "角色不属于该项目")
	}
	fields := map[string]any{
		"character_name":   req.CharacterName,
		"alias_text":       req.AliasText,
		"role_type_code":   req.RoleTypeCode,
		"personality_text": req.PersonalityText,
		"background_text":  req.BackgroundText,
		"appearance_text":  req.AppearanceText,
		"goal_text":        req.GoalText,
	}
	if err := s.characterRepo.Update(ctx, req.CharacterULID, fields); err != nil {
		return domain.CharacterVO{}, err
	}
	updated, err := s.characterRepo.GetByULID(ctx, req.CharacterULID)
	if err != nil {
		return domain.CharacterVO{}, err
	}
	return toCharacterVO(updated), nil
}

// Delete 删除角色（连带删其关系边，同事务）。
func (s *CharacterService) Delete(ctx context.Context, userULID string, req *domain.CharacterDeleteREQ) error {
	return repo.WithTx(s.db, func(tx *gorm.DB) error {
		txCharacter := repo.NewCharacterRepo(tx)
		txProject := repo.NewProjectRepo(tx)
		character, err := txCharacter.GetByULID(ctx, req.CharacterULID)
		if err != nil {
			return err
		}
		if character == nil {
			return pkg.NewBizError(domain.RespCodeNotFound, "角色不存在")
		}
		if _, err := mustOwnProject(ctx, txProject, userULID, character.ProjectULID); err != nil {
			return err
		}
		if err := txCharacter.DeleteRelationsByCharacter(ctx, req.CharacterULID); err != nil {
			return err
		}
		return txCharacter.Delete(ctx, req.CharacterULID)
	})
}

// List 查询项目角色列表（类型/关键词过滤）。
func (s *CharacterService) List(ctx context.Context, userULID string, req *domain.CharacterListREQ) ([]domain.CharacterVO, error) {
	if _, err := mustOwnProject(ctx, s.projectRepo, userULID, req.ProjectULID); err != nil {
		return nil, err
	}
	dos, err := s.characterRepo.ListByProject(ctx, req.ProjectULID, req.RoleTypeCode, req.KeywordText)
	if err != nil {
		return nil, err
	}
	vos := make([]domain.CharacterVO, 0, len(dos))
	for i := range dos {
		vos = append(vos, toCharacterVO(&dos[i]))
	}
	return vos, nil
}

// Graph 查询项目角色关系图（节点=角色，边=关系）。
func (s *CharacterService) Graph(ctx context.Context, userULID string, req *domain.ProjectDetailREQ) (domain.CharacterGraphVO, error) {
	if _, err := mustOwnProject(ctx, s.projectRepo, userULID, req.ProjectULID); err != nil {
		return domain.CharacterGraphVO{}, err
	}
	characters, err := s.characterRepo.ListByProject(ctx, req.ProjectULID, "", "")
	if err != nil {
		return domain.CharacterGraphVO{}, err
	}
	relations, err := s.characterRepo.ListRelationsByProject(ctx, req.ProjectULID)
	if err != nil {
		return domain.CharacterGraphVO{}, err
	}
	nodeList := make([]domain.CharacterNodeVO, 0, len(characters))
	for i := range characters {
		nodeList = append(nodeList, domain.CharacterNodeVO{
			CharacterULID: characters[i].CharacterULID,
			CharacterName: characters[i].CharacterName,
			RoleTypeCode:  characters[i].RoleTypeCode,
		})
	}
	edgeList := make([]domain.CharacterEdgeVO, 0, len(relations))
	for i := range relations {
		edgeList = append(edgeList, toCharacterEdgeVO(&relations[i]))
	}
	return domain.CharacterGraphVO{NodeList: nodeList, EdgeList: edgeList}, nil
}

// RelationSave 新建角色关系边（两端须同项目，不自环）。
func (s *CharacterService) RelationSave(ctx context.Context, userULID string, req *domain.CharacterRelationREQ) (domain.CharacterEdgeVO, error) {
	if _, err := mustOwnProject(ctx, s.projectRepo, userULID, req.ProjectULID); err != nil {
		return domain.CharacterEdgeVO{}, err
	}
	if req.FromCharacterULID == req.ToCharacterULID {
		return domain.CharacterEdgeVO{}, pkg.NewBizError(domain.RespCodeInvalidParam, "关系两端不能是同一角色")
	}
	from, err := s.characterRepo.GetByULID(ctx, req.FromCharacterULID)
	if err != nil {
		return domain.CharacterEdgeVO{}, err
	}
	if from == nil {
		return domain.CharacterEdgeVO{}, pkg.NewBizError(domain.RespCodeNotFound, "起点角色不存在")
	}
	to, err := s.characterRepo.GetByULID(ctx, req.ToCharacterULID)
	if err != nil {
		return domain.CharacterEdgeVO{}, err
	}
	if to == nil {
		return domain.CharacterEdgeVO{}, pkg.NewBizError(domain.RespCodeNotFound, "终点角色不存在")
	}
	if from.ProjectULID != req.ProjectULID || to.ProjectULID != req.ProjectULID {
		return domain.CharacterEdgeVO{}, pkg.NewBizError(domain.RespCodeInvalidParam, "关系两端角色须属于同一项目")
	}
	do := &domain.CharacterRelationDO{
		RelationULID:      pkg.NewULID(domain.IDPrefixRelation),
		ProjectULID:       req.ProjectULID,
		FromCharacterULID: req.FromCharacterULID,
		ToCharacterULID:   req.ToCharacterULID,
		RelationTypeCode:  req.RelationTypeCode,
		RelationNote:      req.RelationNote,
	}
	if err := s.characterRepo.CreateRelation(ctx, do); err != nil {
		return domain.CharacterEdgeVO{}, err
	}
	return toCharacterEdgeVO(do), nil
}

// RelationDelete 删除角色关系边。
func (s *CharacterService) RelationDelete(ctx context.Context, userULID string, req *domain.CharacterRelationDeleteREQ) error {
	relation, err := s.characterRepo.GetRelationByULID(ctx, req.RelationULID)
	if err != nil {
		return err
	}
	if relation == nil {
		return pkg.NewBizError(domain.RespCodeNotFound, "关系不存在")
	}
	if _, err := mustOwnProject(ctx, s.projectRepo, userULID, relation.ProjectULID); err != nil {
		return err
	}
	return s.characterRepo.DeleteRelation(ctx, req.RelationULID)
}

// toCharacterVO 组装角色视图对象。
func toCharacterVO(do *domain.CharacterDO) domain.CharacterVO {
	return domain.CharacterVO{
		CharacterULID:   do.CharacterULID,
		ProjectULID:     do.ProjectULID,
		CharacterName:   do.CharacterName,
		AliasText:       do.AliasText,
		RoleTypeCode:    do.RoleTypeCode,
		PersonalityText: do.PersonalityText,
		BackgroundText:  do.BackgroundText,
		AppearanceText:  do.AppearanceText,
		GoalText:        do.GoalText,
	}
}

// toCharacterEdgeVO 组装关系边视图对象。
func toCharacterEdgeVO(do *domain.CharacterRelationDO) domain.CharacterEdgeVO {
	return domain.CharacterEdgeVO{
		RelationULID:      do.RelationULID,
		FromCharacterULID: do.FromCharacterULID,
		ToCharacterULID:   do.ToCharacterULID,
		RelationTypeCode:  do.RelationTypeCode,
		RelationNote:      do.RelationNote,
	}
}
