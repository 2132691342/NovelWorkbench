// 本文件实现世界观设定业务：设定条目 CRUD 与列表查询。

package service

import (
	"context"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
	"github.com/2132691342/NovelWorkbench/backend/internal/repo"
)

// LoreService 世界观设定业务：设定条目 CRUD 与检索。
type LoreService struct {
	loreRepo    *repo.LoreRepo    // 设定仓储
	projectRepo *repo.ProjectRepo // 项目仓储（归属校验）
}

// NewLoreService 构造设定服务。
func NewLoreService(loreRepo *repo.LoreRepo, projectRepo *repo.ProjectRepo) *LoreService {
	return &LoreService{loreRepo: loreRepo, projectRepo: projectRepo}
}

// Save 新建/更新设定（有 lore_ulid 为更新）。
func (s *LoreService) Save(ctx context.Context, userULID string, req *domain.LoreREQ) (domain.LoreVO, error) {
	if _, err := mustOwnProject(ctx, s.projectRepo, userULID, req.ProjectULID); err != nil {
		return domain.LoreVO{}, err
	}
	if req.LoreULID == "" {
		contentModeCode := req.ContentModeCode
		if contentModeCode == "" {
			contentModeCode = domain.LoreContentModeRich
		}
		do := &domain.LoreDO{
			LoreULID:        pkg.NewULID(domain.IDPrefixLore),
			ProjectULID:     req.ProjectULID,
			LoreName:        req.LoreName,
			LoreTypeCode:    req.LoreTypeCode,
			ContentText:     req.ContentText,
			ContentModeCode: contentModeCode,
			SortOrder:       req.SortOrder,
		}
		if err := s.loreRepo.Create(ctx, do); err != nil {
			return domain.LoreVO{}, err
		}
		return toLoreVO(do), nil
	}
	existing, err := s.loreRepo.GetByULID(ctx, req.LoreULID)
	if err != nil {
		return domain.LoreVO{}, err
	}
	if existing == nil {
		return domain.LoreVO{}, pkg.NewBizError(domain.RespCodeNotFound, "设定不存在")
	}
	if existing.ProjectULID != req.ProjectULID {
		return domain.LoreVO{}, pkg.NewBizError(domain.RespCodeInvalidParam, "设定不属于该项目")
	}
	fields := map[string]any{
		"lore_name":      req.LoreName,
		"lore_type_code": req.LoreTypeCode,
		"content_text":   req.ContentText,
		"sort_order":     req.SortOrder,
	}
	if req.ContentModeCode != "" {
		fields["content_mode_code"] = req.ContentModeCode
	}
	if err := s.loreRepo.Update(ctx, req.LoreULID, fields); err != nil {
		return domain.LoreVO{}, err
	}
	updated, err := s.loreRepo.GetByULID(ctx, req.LoreULID)
	if err != nil {
		return domain.LoreVO{}, err
	}
	return toLoreVO(updated), nil
}

// Delete 删除设定。
func (s *LoreService) Delete(ctx context.Context, userULID string, req *domain.LoreDeleteREQ) error {
	lore, err := s.loreRepo.GetByULID(ctx, req.LoreULID)
	if err != nil {
		return err
	}
	if lore == nil {
		return pkg.NewBizError(domain.RespCodeNotFound, "设定不存在")
	}
	if _, err := mustOwnProject(ctx, s.projectRepo, userULID, lore.ProjectULID); err != nil {
		return err
	}
	return s.loreRepo.Delete(ctx, req.LoreULID)
}

// List 查询项目设定列表（类型/关键词过滤）。
func (s *LoreService) List(ctx context.Context, userULID string, req *domain.LoreListREQ) ([]domain.LoreVO, error) {
	if _, err := mustOwnProject(ctx, s.projectRepo, userULID, req.ProjectULID); err != nil {
		return nil, err
	}
	dos, err := s.loreRepo.ListByProject(ctx, req.ProjectULID, req.LoreTypeCode, req.KeywordText)
	if err != nil {
		return nil, err
	}
	vos := make([]domain.LoreVO, 0, len(dos))
	for i := range dos {
		vos = append(vos, toLoreVO(&dos[i]))
	}
	return vos, nil
}

// toLoreVO 组装设定视图对象。
func toLoreVO(do *domain.LoreDO) domain.LoreVO {
	return domain.LoreVO{
		LoreULID:     do.LoreULID,
		ProjectULID:  do.ProjectULID,
		LoreName:     do.LoreName,
		LoreTypeCode: do.LoreTypeCode,
		ContentText:     do.ContentText,
		ContentModeCode: do.ContentModeCode,
		SortOrder:       do.SortOrder,
		UpdatedAtMs:  do.UpdatedAt.UnixMilli(),
	}
}
