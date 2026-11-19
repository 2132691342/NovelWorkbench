// 本文件实现小说项目业务：项目 CRUD、列表、目录树与级联删除。

package service

import (
	"context"

	"gorm.io/gorm"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/infra"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
	"github.com/2132691342/NovelWorkbench/backend/internal/repo"
)

// ProjectCascadeDeps 项目级联删除依赖（DB 子表仓储 + 外部存储清理客户端）。
type ProjectCascadeDeps struct {
	CharacterRepo  *repo.CharacterRepo    // 角色与关系边
	LoreRepo       *repo.LoreRepo         // 世界观设定
	ChatRepo       *repo.ChatRepo         // AI 会话与消息
	KnowledgeRepo  *repo.KnowledgeRepo    // 知识库文档与切片
	ObjectRepo     *repo.ObjectRepo       // MinIO 原文对象
	Manticore      *infra.ManticoreClient // Manticore 全文索引
	ManticoreTable string                 // 全文索引表名
}

// ProjectService 小说项目业务：项目 CRUD、列表、目录树与级联删除。
type ProjectService struct {
	projectRepo *repo.ProjectRepo  // 项目仓储（归属校验）
	volumeRepo  *repo.VolumeRepo   // 卷仓储
	chapterRepo *repo.ChapterRepo  // 章节仓储
	cascade     ProjectCascadeDeps // 级联删除依赖
	db          *gorm.DB           // 事务入口
}

// NewProjectService 构造项目服务。
func NewProjectService(projectRepo *repo.ProjectRepo, volumeRepo *repo.VolumeRepo, chapterRepo *repo.ChapterRepo, db *gorm.DB, cascade ProjectCascadeDeps) *ProjectService {
	return &ProjectService{projectRepo: projectRepo, volumeRepo: volumeRepo, chapterRepo: chapterRepo, cascade: cascade, db: db}
}

// Create 创建项目（状态默认 draft）。
func (s *ProjectService) Create(ctx context.Context, userULID string, req *domain.ProjectCreateREQ) (domain.ProjectVO, error) {
	do := &domain.ProjectDO{
		ProjectULID:   pkg.NewULID(domain.IDPrefixProject),
		UserULID:      userULID,
		ProjectName:   req.ProjectName,
		GenreCode:     req.GenreCode,
		StyleText:     req.StyleText,
		SynopsisText:  req.SynopsisText,
		ProjectStatus: "draft",
	}
	if err := s.projectRepo.Create(ctx, do); err != nil {
		return domain.ProjectVO{}, err
	}
	return toProjectVO(do, 0, 0), nil
}

// Update 更新项目可编辑字段。
func (s *ProjectService) Update(ctx context.Context, userULID string, req *domain.ProjectUpdateREQ) (domain.ProjectVO, error) {
	if _, err := mustOwnProject(ctx, s.projectRepo, userULID, req.ProjectULID); err != nil {
		return domain.ProjectVO{}, err
	}
	fields := map[string]any{
		"project_name":  req.ProjectName,
		"genre_code":    req.GenreCode,
		"style_text":    req.StyleText,
		"synopsis_text": req.SynopsisText,
	}
	if req.ProjectStatus != "" {
		fields["project_status"] = req.ProjectStatus
	}
	if err := s.projectRepo.Update(ctx, req.ProjectULID, fields); err != nil {
		return domain.ProjectVO{}, err
	}
	return s.projectVOOf(ctx, userULID, req.ProjectULID)
}

// Delete 级联删除项目全部从属数据（DB 单事务），事务提交后清理 MinIO/Manticore。
func (s *ProjectService) Delete(ctx context.Context, userULID string, req *domain.ProjectDeleteREQ) error {
	if _, err := mustOwnProject(ctx, s.projectRepo, userULID, req.ProjectULID); err != nil {
		return err
	}
	docs, err := s.cascade.KnowledgeRepo.ListDocumentsByProject(ctx, req.ProjectULID)
	if err != nil {
		return err
	}
	err = repo.WithTx(s.db, func(tx *gorm.DB) error {
		txProject := repo.NewProjectRepo(tx)
		txVolume := repo.NewVolumeRepo(tx)
		txChapter := repo.NewChapterRepo(tx)
		txCharacter := repo.NewCharacterRepo(tx)
		txLore := repo.NewLoreRepo(tx)
		txChat := repo.NewChatRepo(tx)
		txKnowledge := repo.NewKnowledgeRepo(tx)
		if err := txChapter.DeleteSnapshotsByProject(ctx, req.ProjectULID); err != nil {
			return err
		}
		if err := txChapter.DeleteByProject(ctx, req.ProjectULID); err != nil {
			return err
		}
		if err := txVolume.DeleteByProject(ctx, req.ProjectULID); err != nil {
			return err
		}
		if err := txCharacter.DeleteByProject(ctx, req.ProjectULID); err != nil {
			return err
		}
		if err := txLore.DeleteByProject(ctx, req.ProjectULID); err != nil {
			return err
		}
		if err := txChat.DeleteByProject(ctx, req.ProjectULID); err != nil {
			return err
		}
		if err := txKnowledge.DeleteChunksByProject(ctx, req.ProjectULID); err != nil {
			return err
		}
		if err := txKnowledge.DeleteDocumentsByProject(ctx, req.ProjectULID); err != nil {
			return err
		}
		if err := txProject.DeleteWordStatsByProject(ctx, req.ProjectULID); err != nil {
			return err
		}
		return txProject.Delete(ctx, req.ProjectULID)
	})
	if err != nil {
		return err
	}
	s.cleanupKnowledgeExternal(ctx, docs)
	return nil
}

// cleanupKnowledgeExternal 清理项目文档的外部副本；失败仅告警，数据库一致性已由事务保证。
func (s *ProjectService) cleanupKnowledgeExternal(ctx context.Context, docs []domain.KnowledgeDocumentDO) {
	for i := range docs {
		if err := s.cascade.ObjectRepo.RemoveObject(ctx, docs[i].ObjectKey); err != nil {
			pkg.SLogger().Warnw("项目删除：MinIO 原文清理失败", "doc_ulid", docs[i].DocULID, "error", err)
		}
		if s.cascade.Manticore == nil {
			continue
		}
		if err := deleteManticoreChunks(ctx, s.cascade.Manticore, s.cascade.ManticoreTable, docs[i].DocULID); err != nil {
			pkg.SLogger().Warnw("项目删除：Manticore 切片清理失败", "doc_ulid", docs[i].DocULID, "error", err)
		}
	}
}

// List 分页查询当前用户的项目（名称模糊 + 状态过滤）。
func (s *ProjectService) List(ctx context.Context, userULID string, req *domain.ProjectListREQ) (domain.PageDTO[domain.ProjectVO], error) {
	req.Normalize()
	dos, total, err := s.projectRepo.List(ctx, userULID, req.KeywordText, req.ProjectStatus, req.PageNum, req.PageSize)
	if err != nil {
		return domain.PageDTO[domain.ProjectVO]{}, err
	}
	projectULIDs := make([]string, 0, len(dos))
	for i := range dos {
		projectULIDs = append(projectULIDs, dos[i].ProjectULID)
	}
	volumeCounts, err := s.projectRepo.CountVolumesByProject(ctx, projectULIDs)
	if err != nil {
		return domain.PageDTO[domain.ProjectVO]{}, err
	}
	chapterCounts, err := s.projectRepo.CountChaptersByProject(ctx, projectULIDs)
	if err != nil {
		return domain.PageDTO[domain.ProjectVO]{}, err
	}
	items := make([]domain.ProjectVO, 0, len(dos))
	for i := range dos {
		items = append(items, toProjectVO(&dos[i], int(volumeCounts[dos[i].ProjectULID]), int(chapterCounts[dos[i].ProjectULID])))
	}
	return domain.NewPageDTO(total, req.PageREQ, items), nil
}

// Detail 查询项目详情（含卷数/章节数）。
func (s *ProjectService) Detail(ctx context.Context, userULID string, req *domain.ProjectDetailREQ) (domain.ProjectVO, error) {
	return s.projectVOOf(ctx, userULID, req.ProjectULID)
}

// Tree 查询项目目录树（卷列表 + 每卷章节升序）。
func (s *ProjectService) Tree(ctx context.Context, userULID string, req *domain.ProjectTreeREQ) (domain.ProjectTreeVO, error) {
	project, err := mustOwnProject(ctx, s.projectRepo, userULID, req.ProjectULID)
	if err != nil {
		return domain.ProjectTreeVO{}, err
	}
	volumes, err := s.volumeRepo.ListByProject(ctx, req.ProjectULID)
	if err != nil {
		return domain.ProjectTreeVO{}, err
	}
	chapters, err := s.chapterRepo.ListByProject(ctx, req.ProjectULID)
	if err != nil {
		return domain.ProjectTreeVO{}, err
	}
	chaptersByVolume := make(map[string][]domain.ChapterBriefVO, len(volumes))
	for i := range chapters {
		chapterBrief := toChapterBriefVO(&chapters[i])
		chaptersByVolume[chapters[i].VolumeULID] = append(chaptersByVolume[chapters[i].VolumeULID], chapterBrief)
	}
	volumeList := make([]domain.VolumeTreeNodeVO, 0, len(volumes))
	for i := range volumes {
		chapterList := chaptersByVolume[volumes[i].VolumeULID]
		if chapterList == nil {
			chapterList = []domain.ChapterBriefVO{}
		}
		volumeList = append(volumeList, domain.VolumeTreeNodeVO{
			VolumeVO:    toVolumeVO(&volumes[i]),
			ChapterList: chapterList,
		})
	}
	return domain.ProjectTreeVO{
		ProjectVO:  toProjectVO(project, len(volumes), len(chapters)),
		VolumeList: volumeList,
	}, nil
}

// projectVOOf 加载项目并组装视图（含卷数/章节数），归属校验在内。
func (s *ProjectService) projectVOOf(ctx context.Context, userULID, projectULID string) (domain.ProjectVO, error) {
	project, err := mustOwnProject(ctx, s.projectRepo, userULID, projectULID)
	if err != nil {
		return domain.ProjectVO{}, err
	}
	volumeCounts, err := s.projectRepo.CountVolumesByProject(ctx, []string{projectULID})
	if err != nil {
		return domain.ProjectVO{}, err
	}
	chapterCounts, err := s.projectRepo.CountChaptersByProject(ctx, []string{projectULID})
	if err != nil {
		return domain.ProjectVO{}, err
	}
	return toProjectVO(project, int(volumeCounts[projectULID]), int(chapterCounts[projectULID])), nil
}

// mustOwnProject 校验项目存在且归属当前用户，返回项目。
func mustOwnProject(ctx context.Context, projectRepo *repo.ProjectRepo, userULID, projectULID string) (*domain.ProjectDO, error) {
	project, err := projectRepo.GetByULID(ctx, projectULID)
	if err != nil {
		return nil, err
	}
	if project == nil {
		return nil, pkg.NewBizError(domain.RespCodeNotFound, "项目不存在")
	}
	if project.UserULID != userULID {
		return nil, pkg.NewBizError(domain.RespCodeForbidden, "无权操作该项目")
	}
	return project, nil
}

// toProjectVO 组装项目视图对象。
func toProjectVO(do *domain.ProjectDO, volumeCount, chapterCount int) domain.ProjectVO {
	return domain.ProjectVO{
		ProjectULID:    do.ProjectULID,
		ProjectName:    do.ProjectName,
		GenreCode:      do.GenreCode,
		StyleText:      do.StyleText,
		SynopsisText:   do.SynopsisText,
		ProjectStatus:  do.ProjectStatus,
		TotalWordCount: do.TotalWordCount,
		VolumeCount:    volumeCount,
		ChapterCount:   chapterCount,
		CreatedAtMs:    do.CreatedAt.UnixMilli(),
		UpdatedAtMs:    do.UpdatedAt.UnixMilli(),
	}
}
