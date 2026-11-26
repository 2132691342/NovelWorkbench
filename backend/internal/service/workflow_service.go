// 本文件实现工作流业务主体：模板播种、运行创建、查询与人工审核（执行引擎见 workflow_engine.go）。

package service

import (
	"context"
	"encoding/json"

	"github.com/2132691342/NovelWorkbench/backend/internal/ai"
	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
	"github.com/2132691342/NovelWorkbench/backend/internal/repo"
)

// WorkflowService 工作流业务：模板、运行实例、审核门禁与执行调度。
type WorkflowService struct {
	workflowRepo   *repo.WorkflowRepo    // 工作流仓储
	projectRepo    *repo.ProjectRepo     // 项目仓储（归属校验）
	chapterRepo    *repo.ChapterRepo     // 章节仓储（目标章节与定稿入库）
	volumeRepo     *repo.VolumeRepo      // 卷仓储（无章节时新建章挂载）
	continuityRepo *repo.ContinuityRepo  // 章节摘要仓储
	promptSvc      *PromptService        // 提示词解析（按编码取正文）
	providerSvc    *ModelProviderService // 模型选择与授权
	aiClient       *ai.Client            // 模型网关（执行引擎用）
	notifySvc      *NotificationService  // 站内通知（运行结束提醒）
	runner         *workflowRunner       // 执行器（后台 goroutine 调度）
	auditRecorder  pkg.AuditRecorder     // 审计写入（步骤留痕，可为 nil）
}

// NewWorkflowService 构造工作流服务（执行器在同包内装配，避免 service 之间循环依赖）。
func NewWorkflowService(workflowRepo *repo.WorkflowRepo, projectRepo *repo.ProjectRepo,
	chapterRepo *repo.ChapterRepo, volumeRepo *repo.VolumeRepo, continuityRepo *repo.ContinuityRepo,
	promptSvc *PromptService, providerSvc *ModelProviderService, aiClient *ai.Client,
	notifySvc *NotificationService) *WorkflowService {
	svc := &WorkflowService{
		workflowRepo: workflowRepo, projectRepo: projectRepo, chapterRepo: chapterRepo,
		volumeRepo: volumeRepo, continuityRepo: continuityRepo, promptSvc: promptSvc,
		providerSvc: providerSvc, aiClient: aiClient, notifySvc: notifySvc,
	}
	svc.runner = newWorkflowRunner(svc)
	return svc
}

// SetAuditRecorder 注入审计写入器（步骤执行留痕，复用运行链路 ID 串成时间线）。
func (s *WorkflowService) SetAuditRecorder(recorder pkg.AuditRecorder) {
	s.auditRecorder = recorder
}

// SeedDefaults 播种内置模板（幂等：按编码查缺补插，步骤定义随代码升级同步）。
func (s *WorkflowService) SeedDefaults(ctx context.Context) error {
	for _, seed := range workflowTemplateSeedList {
		stepSchemaJSON := marshalWorkflowSteps(seed.StepList)
		existing, err := s.workflowRepo.GetTemplateByCode(ctx, seed.TemplateCode)
		if err != nil {
			return err
		}
		if existing == nil {
			if err := s.workflowRepo.CreateTemplate(ctx, &domain.WorkflowTemplateDO{
				TemplateULID:    pkg.NewULID(domain.IDPrefixWorkflowTemplate),
				TemplateCode:    seed.TemplateCode,
				TemplateName:    seed.TemplateName,
				DescriptionText: seed.DescriptionText,
				StepSchemaJSON:  stepSchemaJSON,
				BuiltinFlag:     true,
				EnabledFlag:     true,
				SortOrder:       seed.SortOrder,
			}); err != nil {
				return err
			}
			continue
		}
		if existing.StepSchemaJSON == stepSchemaJSON && existing.TemplateName == seed.TemplateName &&
			existing.DescriptionText == seed.DescriptionText {
			continue
		}
		if err := s.workflowRepo.UpdateTemplate(ctx, existing.TemplateULID, map[string]any{
			"template_name":    seed.TemplateName,
			"description_text": seed.DescriptionText,
			"step_schema_json": stepSchemaJSON,
			"sort_order":       seed.SortOrder,
		}); err != nil {
			return err
		}
	}
	return nil
}

// ListTemplates 返回可用模板（含步骤定义，供前端预览流程）。
func (s *WorkflowService) ListTemplates(ctx context.Context) ([]domain.WorkflowTemplateVO, error) {
	dos, err := s.workflowRepo.ListTemplates(ctx)
	if err != nil {
		return nil, err
	}
	vos := make([]domain.WorkflowTemplateVO, 0, len(dos))
	for i := range dos {
		vos = append(vos, toWorkflowTemplateVO(&dos[i]))
	}
	return vos, nil
}

// CreateRun 创建工作流运行：解析目标章节 → 固化参数 → 落库 → 按需启动执行。
func (s *WorkflowService) CreateRun(ctx context.Context, userULID string, req *domain.WorkflowRunCreateREQ) (domain.WorkflowRunVO, error) {
	project, err := mustOwnProject(ctx, s.projectRepo, userULID, req.ProjectULID)
	if err != nil {
		return domain.WorkflowRunVO{}, err
	}
	template, err := s.workflowRepo.GetTemplateByCode(ctx, req.TemplateCode)
	if err != nil {
		return domain.WorkflowRunVO{}, err
	}
	if template == nil || !template.EnabledFlag {
		return domain.WorkflowRunVO{}, pkg.NewBizError(domain.RespCodeNotFound, "工作流模板不存在或已停用")
	}
	stepList := unmarshalWorkflowSteps(template.StepSchemaJSON)
	if len(stepList) == 0 {
		return domain.WorkflowRunVO{}, pkg.NewBizError(domain.RespCodeBizFailed, "模板未定义步骤，无法执行")
	}
	params := req.WorkflowRunParamsDTO
	if err := normalizeWorkflowParams(&params); err != nil {
		return domain.WorkflowRunVO{}, err
	}
	chapterULIDList, err := s.resolveChapterList(ctx, project, template.TemplateCode, &params)
	if err != nil {
		return domain.WorkflowRunVO{}, err
	}
	params.ChapterULIDList = chapterULIDList
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return domain.WorkflowRunVO{}, pkg.WrapBizError(domain.RespCodeInternalError, "运行参数序列化失败", err)
	}
	chapterULIDListJSON, err := json.Marshal(chapterULIDList)
	if err != nil {
		return domain.WorkflowRunVO{}, pkg.WrapBizError(domain.RespCodeInternalError, "章节列表序列化失败", err)
	}
	runDO := &domain.WorkflowRunDO{
		RunULID:             pkg.NewULID(domain.IDPrefixWorkflowRun),
		ProjectULID:         req.ProjectULID,
		UserULID:            userULID,
		TemplateCode:        template.TemplateCode,
		TemplateName:        template.TemplateName,
		RunStatusCode:       domain.WorkflowRunStatusQueued,
		ChapterULIDListJSON: string(chapterULIDListJSON),
		ParamsJSON:          string(paramsJSON),
		TotalChapterCount:   len(chapterULIDList),
		TraceID:             firstNonEmpty(pkg.RequestIDFromContext(ctx), pkg.NewULID(domain.IDPrefixTrace)),
	}
	if err := s.workflowRepo.CreateRun(ctx, runDO); err != nil {
		return domain.WorkflowRunVO{}, err
	}
	vo := toWorkflowRunVO(runDO, project.ProjectName, "")
	if req.AutoStartFlag {
		s.runner.start(runDO.RunULID)
	}
	return vo, nil
}

