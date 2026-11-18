package service

import (
	"context"

	"gorm.io/gorm"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
	"github.com/2132691342/NovelWorkbench/backend/internal/repo"
)

// VolumeService 卷业务：卷的增删改（归属校验经所属项目）。
type VolumeService struct {
	volumeRepo  *repo.VolumeRepo
	chapterRepo *repo.ChapterRepo
	projectRepo *repo.ProjectRepo
	db          *gorm.DB
}

// NewVolumeService 构造卷服务。
func NewVolumeService(volumeRepo *repo.VolumeRepo, chapterRepo *repo.ChapterRepo, projectRepo *repo.ProjectRepo, db *gorm.DB) *VolumeService {
	return &VolumeService{volumeRepo: volumeRepo, chapterRepo: chapterRepo, projectRepo: projectRepo, db: db}
}

// Create 创建卷（排序号缺省时取项目内最大值 +1）。
func (s *VolumeService) Create(ctx context.Context, userULID string, req *domain.VolumeCreateREQ) (domain.VolumeVO, error) {
	if _, err := mustOwnProject(ctx, s.projectRepo, userULID, req.ProjectULID); err != nil {
		return domain.VolumeVO{}, err
	}
	volumeOrder := req.VolumeOrder
	if volumeOrder <= 0 {
		maxOrder, err := s.volumeRepo.MaxOrder(ctx, req.ProjectULID)
		if err != nil {
			return domain.VolumeVO{}, err
		}
		volumeOrder = maxOrder + 1
	}
	do := &domain.VolumeDO{
		VolumeULID:  pkg.NewULID(domain.IDPrefixVolume),
		ProjectULID: req.ProjectULID,
		VolumeName:  req.VolumeName,
		VolumeOrder: volumeOrder,
		SummaryText: req.SummaryText,
	}
	if err := s.volumeRepo.Create(ctx, do); err != nil {
		return domain.VolumeVO{}, err
	}
	return toVolumeVO(do), nil
}

// Update 更新卷（仅覆盖非空字段）。
func (s *VolumeService) Update(ctx context.Context, userULID string, req *domain.VolumeUpdateREQ) (domain.VolumeVO, error) {
	volume, err := s.mustOwnVolume(ctx, userULID, req.VolumeULID)
	if err != nil {
		return domain.VolumeVO{}, err
	}
	fields := map[string]any{}
	if req.VolumeName != "" {
		fields["volume_name"] = req.VolumeName
	}
	if req.VolumeOrder > 0 {
		fields["volume_order"] = req.VolumeOrder
	}
	if req.SummaryText != "" {
		fields["summary_text"] = req.SummaryText
	}
	if len(fields) > 0 {
		if err := s.volumeRepo.Update(ctx, req.VolumeULID, fields); err != nil {
			return domain.VolumeVO{}, err
		}
		volume, err = s.volumeRepo.GetByULID(ctx, req.VolumeULID)
		if err != nil {
			return domain.VolumeVO{}, err
		}
		if volume == nil {
			return domain.VolumeVO{}, pkg.NewBizError(domain.RespCodeNotFound, "卷不存在")
		}
	}
	return toVolumeVO(volume), nil
}

// Delete 删除卷及其章节/快照，并回补项目累计字数（单事务）。
func (s *VolumeService) Delete(ctx context.Context, userULID string, req *domain.VolumeDeleteREQ) error {
	return repo.WithTx(s.db, func(tx *gorm.DB) error {
		txVolume := repo.NewVolumeRepo(tx)
		txChapter := repo.NewChapterRepo(tx)
		txProject := repo.NewProjectRepo(tx)
		volume, err := txVolume.GetByULID(ctx, req.VolumeULID)
		if err != nil {
			return err
		}
		if volume == nil {
			return pkg.NewBizError(domain.RespCodeNotFound, "卷不存在")
		}
		if _, err := mustOwnProject(ctx, txProject, userULID, volume.ProjectULID); err != nil {
			return err
		}
		totalWordCount, err := txChapter.SumWordCountByVolume(ctx, req.VolumeULID)
		if err != nil {
			return err
		}
		if err := txChapter.DeleteSnapshotsByVolume(ctx, req.VolumeULID); err != nil {
			return err
		}
		if err := txChapter.DeleteByVolume(ctx, req.VolumeULID); err != nil {
			return err
		}
		if err := txVolume.Delete(ctx, req.VolumeULID); err != nil {
			return err
		}
		if totalWordCount != 0 {
			return applyWordDelta(ctx, txProject, volume.ProjectULID, -totalWordCount)
		}
		return nil
	})
}

// mustOwnVolume 加载卷并校验所属项目归属当前用户。
func (s *VolumeService) mustOwnVolume(ctx context.Context, userULID, volumeULID string) (*domain.VolumeDO, error) {
	volume, err := s.volumeRepo.GetByULID(ctx, volumeULID)
	if err != nil {
		return nil, err
	}
	if volume == nil {
		return nil, pkg.NewBizError(domain.RespCodeNotFound, "卷不存在")
	}
	if _, err := mustOwnProject(ctx, s.projectRepo, userULID, volume.ProjectULID); err != nil {
		return nil, err
	}
	return volume, nil
}

// toVolumeVO 组装卷视图对象。
func toVolumeVO(do *domain.VolumeDO) domain.VolumeVO {
	return domain.VolumeVO{
		VolumeULID:  do.VolumeULID,
		VolumeName:  do.VolumeName,
		VolumeOrder: do.VolumeOrder,
		SummaryText: do.SummaryText,
	}
}
