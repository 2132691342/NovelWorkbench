package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/2132691342/NovelWorkbench/backend/internal/ai"
	"github.com/2132691342/NovelWorkbench/backend/internal/api"
	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/infra"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
	"github.com/2132691342/NovelWorkbench/backend/internal/repo"
	"github.com/2132691342/NovelWorkbench/backend/internal/service"
)

// main 装配并启动 NovelWorkbench HTTP 服务（配置 → 基础设施 → 仓储/服务/路由 → 优雅关闭）。
func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	cfg, err := infra.LoadConfig("")
	if err != nil {
		slog.Error("加载配置失败", "error", err)
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := infra.NewPostgres(&cfg.Postgres)
	if err != nil {
		slog.Error("Postgres 连接失败", "error", err)
		os.Exit(1)
	}
	if err := db.Exec("CREATE EXTENSION IF NOT EXISTS vector").Error; err != nil {
		slog.Warn("创建 vector 扩展失败，知识库向量功能将不可用", "error", err)
	}
	if err := db.AutoMigrate(
		&domain.UserDO{},
		&domain.ProjectDO{},
		&domain.VolumeDO{},
		&domain.ChapterDO{},
		&domain.ChapterSnapshotDO{},
		&domain.WordCountStatDO{},
		&domain.CharacterDO{},
		&domain.CharacterRelationDO{},
		&domain.LoreDO{},
		&domain.KnowledgeDocumentDO{},
		&domain.KnowledgeChunkDO{},
		&domain.SlashCommandDO{},
		&domain.JobDO{},
		&domain.AppSettingDO{},
		&domain.ChatSessionDO{},
		&domain.ChatMessageDO{},
	); err != nil {
		slog.Error("数据库迁移失败", "error", err)
		os.Exit(1)
	}

	// 其余基础设施 Ping 失败仅降级告警，不阻断启动（PG 失败已在上面退出）。
	rdb, err := infra.NewRedis(&cfg.Redis)
	if err != nil && rdb == nil {
		slog.Error("Redis 配置无效", "error", err)
		os.Exit(1)
	}
	if err != nil {
		slog.Warn("Redis 连接失败，令牌吊销降级", "error", err)
	}
	mqConn, err := infra.NewRabbitMQ(&cfg.RabbitMQ)
	if err != nil {
		slog.Warn("RabbitMQ 连接失败，异步任务降级", "error", err)
	}
	minioClient, minioErr := infra.NewMinIO(&cfg.MinIO)
	if minioErr != nil {
		slog.Warn("MinIO 初始化失败，对象存储降级", "error", minioErr)
	} else if ok, err := minioClient.BucketExists(ctx, cfg.MinIO.Bucket); err != nil {
		slog.Warn("MinIO 桶健康检查失败，对象存储降级", "bucket", cfg.MinIO.Bucket, "error", err)
	} else if !ok {
		slog.Warn("MinIO 桶不存在，对象存储降级", "bucket", cfg.MinIO.Bucket)
	}
	manticoreClient := infra.NewManticore(&cfg.Manticore)
	if err := manticoreClient.Ping(ctx); err != nil {
		slog.Warn("Manticore 连接失败，全文检索降级", "error", err)
	}

	signer, err := pkg.NewJWTSigner(cfg.JWT.Secret, cfg.JWTExpireTTL())
	if err != nil {
		slog.Error("构造 JWT 签名器失败", "error", err)
		os.Exit(1)
	}
	tokenRepo := repo.NewTokenRepo(rdb)
	userRepo := repo.NewUserRepo(db)
	projectRepo := repo.NewProjectRepo(db)
	volumeRepo := repo.NewVolumeRepo(db)
	chapterRepo := repo.NewChapterRepo(db)
	characterRepo := repo.NewCharacterRepo(db)
	loreRepo := repo.NewLoreRepo(db)
	knowledgeRepo := repo.NewKnowledgeRepo(db)
	slashRepo := repo.NewSlashRepo(db)
	settingRepo := repo.NewSettingRepo(db)
	statsRepo := repo.NewStatsRepo(db)
	jobRepo := repo.NewJobRepo(db)
	chatRepo := repo.NewChatRepo(db)
	objectRepo := repo.NewObjectRepo(minioClient, cfg.MinIO.Bucket)
	mqRepo := repo.NewMQRepo(mqConn, cfg.RabbitMQ.QueueDocIndex)

	authService := service.NewAuthService(userRepo, signer, tokenRepo)
	projectService := service.NewProjectService(projectRepo, volumeRepo, chapterRepo, db)
	volumeService := service.NewVolumeService(volumeRepo, projectRepo)
	chapterService := service.NewChapterService(chapterRepo, volumeRepo, projectRepo, db)
	characterService := service.NewCharacterService(characterRepo, projectRepo, db)
	loreService := service.NewLoreService(loreRepo, projectRepo)
	slashService := service.NewSlashService(slashRepo)
	settingService := service.NewSettingService(settingRepo)
	statsService := service.NewStatsService(statsRepo, projectRepo)

	aiTimeoutSec := cfg.AI.TimeoutSec
	if aiTimeoutSec <= 0 {
		aiTimeoutSec = 60 // 默认模型调用超时（秒）
	}
	aiClient, err := ai.NewClient(cfg.AI.BaseURL, cfg.AI.APIKey, cfg.AI.ChatModel, cfg.AI.EmbeddingModel, time.Duration(aiTimeoutSec)*time.Second)
	if err != nil {
		slog.Error("构造 AI 客户端失败", "error", err)
		os.Exit(1)
	}
	knowledgeConfig := service.KnowledgeIndexConfig{
		UploadMaxMB:    cfg.App.UploadMaxMB,
		ChunkSize:      cfg.App.ChunkSize,
		ChunkOverlap:   cfg.App.ChunkOverlap,
		RAGTopK:        cfg.App.RAGTopK,
		ManticoreTable: cfg.Manticore.TableChunks,
	}
	knowledgeService := service.NewKnowledgeService(knowledgeRepo, objectRepo, mqRepo, jobRepo, projectRepo, aiClient, manticoreClient, db, knowledgeConfig)
	chatService := service.NewChatService(chatRepo, projectRepo, chapterRepo, characterRepo, knowledgeRepo,
		settingRepo, slashService, knowledgeService, aiClient, db,
		service.ChatConfig{HistoryCount: cfg.AI.HistoryCount, RAGTopK: cfg.App.RAGTopK})

	// 内置技能与默认配置播种（幂等），失败视为启动期数据库异常，直接退出。
	if err := slashService.SeedBuiltin(ctx); err != nil {
		slog.Error("内置斜杠命令播种失败", "error", err)
		os.Exit(1)
	}
	if err := settingService.SeedDefaults(ctx); err != nil {
		slog.Error("默认配置播种失败", "error", err)
		os.Exit(1)
	}

	// 知识库索引 worker：消费 RabbitMQ 完成文档索引，随 ctx 取消优雅退出。
	knowledgeWorker := service.NewKnowledgeWorker(knowledgeRepo, objectRepo, mqRepo, jobRepo, aiClient, manticoreClient, db, knowledgeConfig)
	workerDone := make(chan struct{})
	go func() {
		defer close(workerDone)
		if err := knowledgeWorker.Run(ctx); err != nil {
			slog.Warn("知识库索引 worker 异常退出", "error", err)
		}
	}()

	authMW := pkg.AuthMiddleware(signer, tokenRepo, []string{"/api/v1/auth/register", "/api/v1/auth/login"})
	router := api.NewRouter(cfg.Server.Mode, api.RouterDeps{
		AuthMW:       authMW,
		AuthAPI:      api.NewAuthAPI(authService),
		ProjectAPI:   api.NewProjectAPI(projectService),
		VolumeAPI:    api.NewVolumeAPI(volumeService),
		ChapterAPI:   api.NewChapterAPI(chapterService),
		CharacterAPI: api.NewCharacterAPI(characterService),
		LoreAPI:      api.NewLoreAPI(loreService),
		KnowledgeAPI: api.NewKnowledgeAPI(knowledgeService),
		SlashAPI:     api.NewSlashAPI(slashService),
		SettingAPI:   api.NewSettingAPI(settingService),
		StatsAPI:     api.NewStatsAPI(statsService),
		ChatAPI:      api.NewChatAPI(chatService),
	})

	server := &http.Server{
		Addr:         cfg.ServerAddr(),
		Handler:      router,
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeoutSec) * time.Second,
		WriteTimeout: 0, // SSE 长连接不受写超时限制
	}
	serveErr := make(chan error, 1)
	go func() {
		slog.Info("服务已启动", "addr", cfg.ServerAddr())
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
		}
	}()

	select {
	case <-ctx.Done():
		slog.Info("收到退出信号，开始优雅关闭")
	case err := <-serveErr:
		slog.Error("HTTP 服务异常退出", "error", err)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Warn("HTTP 服务关闭超时", "error", err)
	}
	select {
	case <-workerDone:
	case <-time.After(5 * time.Second):
		slog.Warn("知识库索引 worker 关闭超时")
	}
	if sqlDB, err := db.DB(); err == nil {
		if err := sqlDB.Close(); err != nil {
			slog.Warn("关闭 Postgres 连接失败", "error", err)
		}
	}
	if err := rdb.Close(); err != nil {
		slog.Warn("关闭 Redis 连接失败", "error", err)
	}
	if mqConn != nil && !mqConn.IsClosed() {
		if err := mqConn.Close(); err != nil {
			slog.Warn("关闭 RabbitMQ 连接失败", "error", err)
		}
	}
	slog.Info("服务已关闭")
}
