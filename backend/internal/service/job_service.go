// 本文件实现异步任务中心业务：任务分页查询与失败任务重试。

package service

import (
	"context"
	"encoding/json"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
	"github.com/2132691342/NovelWorkbench/backend/internal/repo"
)

// JobService 异步任务中心业务。
type JobService struct {
	jobRepo       *repo.JobRepo       // 任务仓储
	knowledgeRepo *repo.KnowledgeRepo // 知识库仓储（文档名与重试状态）
	mqRepo        *repo.MQRepo        // 队列仓储（重投索引任务）
}

// NewJobService 构造任务中心服务。
func NewJobService(jobRepo *repo.JobRepo, knowledgeRepo *repo.KnowledgeRepo, mqRepo *repo.MQRepo) *JobService {
	return &JobService{jobRepo: jobRepo, knowledgeRepo: knowledgeRepo, mqRepo: mqRepo}
}

// ListPage 分页查询任务并回填文档名（批量查询避免 N+1）。
func (s *JobService) ListPage(ctx context.Context, req *domain.JobListREQ) (domain.PageDTO[domain.JobVO], error) {
	req.Normalize()
	dos, total, err := s.jobRepo.ListPage(ctx, req)
	if err != nil {
		return domain.PageDTO[domain.JobVO]{}, err
	}
	docULIDList := make([]string, 0, len(dos))
	payloadByJob := make(map[string]domain.KnowledgeIndexJobDTO, len(dos))
	for i := range dos {
		var payload domain.KnowledgeIndexJobDTO
		if err := json.Unmarshal([]byte(dos[i].PayloadJSON), &payload); err == nil && payload.DocULID != "" {
			payloadByJob[dos[i].JobULID] = payload
			docULIDList = append(docULIDList, payload.DocULID)
		}
	}
	docNameByULID := make(map[string]string, len(docULIDList))
	documents, err := s.knowledgeRepo.GetDocumentsByULIDs(ctx, docULIDList)
	if err != nil {
		return domain.PageDTO[domain.JobVO]{}, err
	}
	for i := range documents {
		docNameByULID[documents[i].DocULID] = documents[i].DocName
	}
	vos := make([]domain.JobVO, 0, len(dos))
	for i := range dos {
		payload := payloadByJob[dos[i].JobULID]
		vos = append(vos, domain.JobVO{
			JobULID:       dos[i].JobULID,
			JobTypeCode:   dos[i].JobTypeCode,
			JobStatusCode: dos[i].JobStatusCode,
			DocULID:       payload.DocULID,
			DocName:       docNameByULID[payload.DocULID],
			RetryCount:    dos[i].RetryCount,
			ErrorMessage:  dos[i].ErrorMessage,
			CreatedAt:     pkg.FormatTime(dos[i].CreatedAt),
			UpdatedAt:     pkg.FormatTime(dos[i].UpdatedAt),
		})
	}
	return domain.NewPageDTO(total, req.PageREQ, vos), nil
}

// Stats 统计各状态任务数（任务中心顶部卡片）。
func (s *JobService) Stats(ctx context.Context) (domain.JobStatsVO, error) {
	countByStatus, err := s.jobRepo.CountByStatus(ctx)
	if err != nil {
		return domain.JobStatsVO{}, err
	}
	return domain.JobStatsVO{
		QueuedCount:  countByStatus[domain.JobStatusQueued],
		RunningCount: countByStatus[domain.JobStatusRunning],
		SuccessCount: countByStatus[domain.JobStatusSuccess],
		FailedCount:  countByStatus[domain.JobStatusFailed],
	}, nil
}

// Retry 重试失败任务：重置任务与文档状态后重新投递索引消息。
func (s *JobService) Retry(ctx context.Context, req *domain.JobRetryREQ) (domain.JobVO, error) {
	do, err := s.jobRepo.GetByULID(ctx, req.JobULID)
	if err != nil {
		return domain.JobVO{}, err
	}
	if do == nil {
		return domain.JobVO{}, pkg.NewBizError(domain.RespCodeNotFound, "任务不存在")
	}
	if do.JobStatusCode != domain.JobStatusFailed {
		return domain.JobVO{}, pkg.NewBizError(domain.RespCodeBizFailed, "仅失败任务可以重试")
	}
	var payload domain.KnowledgeIndexJobDTO
	if err := json.Unmarshal([]byte(do.PayloadJSON), &payload); err != nil || payload.DocULID == "" {
		return domain.JobVO{}, pkg.NewBizError(domain.RespCodeInvalidParam, "任务载荷缺少文档信息")
	}
	doc, err := s.knowledgeRepo.GetDocument(ctx, payload.DocULID)
	if err != nil {
		return domain.JobVO{}, err
	}
	if doc == nil {
		return domain.JobVO{}, pkg.NewBizError(domain.RespCodeNotFound, "关联文档已不存在")
	}
	fields := map[string]any{
		"job_status_code": domain.JobStatusQueued,
		"retry_count":     0,
		"error_message":   "",
	}
	if err := s.jobRepo.Update(ctx, do.JobULID, fields); err != nil {
		return domain.JobVO{}, err
	}
	if err := s.knowledgeRepo.UpdateDocument(ctx, doc.DocULID, map[string]any{
		"doc_status_code": "pending",
		"error_message":   "",
	}); err != nil {
		return domain.JobVO{}, err
	}
	if err := s.mqRepo.PublishIndexJob(ctx, payload); err != nil {
		_ = s.jobRepo.Update(ctx, do.JobULID, map[string]any{
			"job_status_code": domain.JobStatusFailed,
			"error_message":   pkg.TruncateText("重试投递失败: "+err.Error(), 512),
		})
		return domain.JobVO{}, pkg.WrapBizError(domain.RespCodeStorageError, "任务重新入队失败", err)
	}
	return domain.JobVO{
		JobULID:       do.JobULID,
		JobTypeCode:   do.JobTypeCode,
		JobStatusCode: domain.JobStatusQueued,
		DocULID:       doc.DocULID,
		DocName:       doc.DocName,
		CreatedAt:     pkg.FormatTime(do.CreatedAt),
		UpdatedAt:     pkg.FormatTime(do.UpdatedAt),
	}, nil
}
