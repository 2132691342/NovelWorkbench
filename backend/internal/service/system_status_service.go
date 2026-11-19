// 本文件实现系统运行状态业务：并行探测各依赖并给出 AI 能力结论。

package service

import (
	"context"
	"runtime"
	"sync"
	"time"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
)

// DependencyProbe 单个依赖的探针（返回 nil 表示可用）。
type DependencyProbe func(ctx context.Context) error

// SystemProbeDeps 运行状态探测依赖（由 main 装配，缺失的探针视为未配置）。
type SystemProbeDeps struct {
	PostgresProbe  DependencyProbe // PostgreSQL Ping
	RedisProbe     DependencyProbe // Redis Ping
	RabbitMQProbe  DependencyProbe // RabbitMQ 连接状态
	MinIOProbe     DependencyProbe // MinIO 桶可用性
	ManticoreProbe DependencyProbe // Manticore 探活
	BuildVersion   string          // 构建版本（编译期注入或默认 dev）
}

// SystemStatusService 系统运行状态业务：并行探测外部依赖并汇总 AI 对话能力。
type SystemStatusService struct {
	deps        SystemProbeDeps       // 依赖探针集合
	providerSvc *ModelProviderService // 供应商服务（模型选择）
	startedAt   time.Time             // 进程启动时刻
}

// NewSystemStatusService 构造运行状态服务。
func NewSystemStatusService(deps SystemProbeDeps, providerSvc *ModelProviderService) *SystemStatusService {
	if deps.BuildVersion == "" {
		deps.BuildVersion = "dev"
	}
	return &SystemStatusService{deps: deps, providerSvc: providerSvc, startedAt: time.Now()}
}

// Status 汇总系统运行状态（依赖探测并发执行，单项失败不影响其余结果）。
func (s *SystemStatusService) Status(ctx context.Context) (domain.SystemStatusVO, error) {
	probeList := []struct {
		code     string
		name     string
		probe    DependencyProbe
		required bool
	}{
		{"postgres", "PostgreSQL（主库）", s.deps.PostgresProbe, true},
		{"redis", "Redis（令牌吊销 / 缓存）", s.deps.RedisProbe, false},
		{"rabbitmq", "RabbitMQ（索引任务队列）", s.deps.RabbitMQProbe, false},
		{"minio", "MinIO（知识库原文对象存储）", s.deps.MinIOProbe, false},
		{"manticore", "Manticore（全文检索）", s.deps.ManticoreProbe, false},
	}
	results := make([]domain.DependencyHealthVO, len(probeList))
	var waitGroup sync.WaitGroup
	for i := range probeList {
		waitGroup.Add(1)
		go func(index int) {
			defer waitGroup.Done()
			results[index] = probeDependency(ctx, probeList[index].code, probeList[index].name,
				probeList[index].probe, probeList[index].required)
		}(i)
	}
	waitGroup.Wait()
	vo := domain.SystemStatusVO{
		ServerStartedAtMs: s.startedAt.UnixMilli(),
		ServerUptimeSec:   int64(time.Since(s.startedAt).Seconds()),
		GoVersionText:     runtime.Version(),
		BuildVersionText:  s.deps.BuildVersion,
		DependencyList:    results,
	}
	capability, err := s.providerSvc.Capability(ctx)
	if err != nil {
		pkg.SLogger().Warnw("AI 能力探测失败", "error", err)
	}
	vo.ChatReady = capability.ChatReady
	vo.HintText = capability.HintText
	modelHealth := domain.DependencyHealthVO{
		DependencyCode: "model",
		DependencyName: "模型供应商（对话）",
		StatusCode:     domain.HealthStatusUp,
		MessageText:    capability.HintText,
	}
	if !capability.ChatReady {
		modelHealth.StatusCode = domain.HealthStatusDown
	}
	vo.DependencyList = append(vo.DependencyList, modelHealth)
	return vo, nil
}

// probeDependency 执行单个依赖探测并归类状态（必需依赖失败记为 down，其余为 degraded）。
func probeDependency(ctx context.Context, code, name string, probe DependencyProbe, required bool) domain.DependencyHealthVO {
	health := domain.DependencyHealthVO{
		DependencyCode: code,
		DependencyName: name,
		StatusCode:     domain.HealthStatusUp,
		MessageText:    "正常",
	}
	if probe == nil {
		health.StatusCode = domain.HealthStatusDegraded
		health.MessageText = "未配置"
		return health
	}
	startedAt := time.Now()
	probeErr := probe(ctx)
	health.LatencyMs = time.Since(startedAt).Milliseconds()
	if probeErr == nil {
		return health
	}
	if required {
		health.StatusCode = domain.HealthStatusDown
	} else {
		health.StatusCode = domain.HealthStatusDegraded
	}
	health.MessageText = truncateRunes(probeErr.Error(), 200)
	return health
}
