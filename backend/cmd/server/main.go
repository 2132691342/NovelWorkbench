// Package main 装配并启动 NovelWorkbench HTTP 服务：加载配置、连接基础设施、构建仓储与服务、注册路由、监听端口并优雅关闭。

package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

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
	pkg.SetLogger(zap.Must(zap.NewProduction())) // 结构化 JSON 日志，全局唯一出口
	cfg, err := infra.LoadConfig("")
	if err != nil {
		pkg.SLogger().Errorw("加载配置失败", "error", err)
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := infra.NewPostgres(&cfg.Postgres)
	if err != nil {
		pkg.SLogger().Errorw("Postgres 连接失败", "error", err)
		os.Exit(1)
	}
	if err := pkg.RegisterActorCallbacks(db); err != nil {
		pkg.SLogger().Errorw("审计字段回调注册失败", "error", err)
		os.Exit(1)
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
		&domain.OutlineNodeDO{},
		&domain.DetailedSceneDO{},
		&domain.TemporalFactDO{},
		&domain.ForeshadowHookDO{},
		&domain.ChapterSummaryDO{},
		&domain.KnowledgeChunkDO{},

		&domain.MenuDO{},
		&domain.JobDO{},
		&domain.AppSettingDO{},
		&domain.ChatSessionDO{},
		&domain.ChatMessageDO{},
		&domain.RoleDO{},
		&domain.UserRoleDO{},
		&domain.AuditLogDO{},
		&domain.ModelProviderDO{},
		&domain.ModelCatalogDO{},
		&domain.ModelRoleGrantDO{},
		&domain.ModelUserGrantDO{},
		&domain.ModelUsageStatDO{},
		&domain.NotificationDO{},
		&domain.LoginLogDO{},
		&domain.PromptDO{},
		&domain.ToolDO{},
	); err != nil {
		pkg.SLogger().Errorw("数据库迁移失败", "error", err)
		os.Exit(1)
	}

	// 其余基础设施 Ping 失败仅降级告警，不阻断启动（PG 失败已在上面退出）。
	rdb, err := infra.NewRedis(&cfg.Redis)
	if err != nil && rdb == nil {
		pkg.SLogger().Errorw("Redis 配置无效", "error", err)
		os.Exit(1)
	}
	if err != nil {
		pkg.SLogger().Warnw("Redis 连接失败，令牌吊销降级", "error", err)
	}
	mqConn, err := infra.NewRabbitMQ(&cfg.RabbitMQ)
	if err != nil {
		pkg.SLogger().Warnw("RabbitMQ 连接失败，异步任务降级", "error", err)
	}
	minioClient, minioErr := infra.NewMinIO(&cfg.MinIO)
	if minioErr != nil {
		pkg.SLogger().Warnw("MinIO 初始化失败，对象存储降级", "error", minioErr)
	} else if ok, err := minioClient.BucketExists(ctx, cfg.MinIO.Bucket); err != nil {
		pkg.SLogger().Warnw("MinIO 桶健康检查失败，对象存储降级", "bucket", cfg.MinIO.Bucket, "error", err)
	} else if !ok {
		pkg.SLogger().Warnw("MinIO 桶不存在，对象存储降级", "bucket", cfg.MinIO.Bucket)
	}
	manticoreClient := infra.NewManticore(&cfg.Manticore)
	if err := manticoreClient.Ping(ctx); err != nil {
		pkg.SLogger().Warnw("Manticore 连接失败，全文检索降级", "error", err)
	}

	signer, err := pkg.NewJWTSigner(cfg.JWT.Secret, cfg.JWTExpireTTL())
	if err != nil {
		pkg.SLogger().Errorw("构造 JWT 签名器失败", "error", err)
		os.Exit(1)
	}
	tokenRepo := repo.NewTokenRepo(rdb)
	userRepo := repo.NewUserRepo(db)
	roleRepo := repo.NewRoleRepo(db)
	auditRepo := repo.NewAuditRepo(db)
	projectRepo := repo.NewProjectRepo(db)
	volumeRepo := repo.NewVolumeRepo(db)
	chapterRepo := repo.NewChapterRepo(db)
	characterRepo := repo.NewCharacterRepo(db)
	loreRepo := repo.NewLoreRepo(db)
	knowledgeRepo := repo.NewKnowledgeRepo(db)
	outlineRepo := repo.NewOutlineRepo(db)
	continuityRepo := repo.NewContinuityRepo(db)
	promptRepo := repo.NewPromptRepo(db)
	menuRepo := repo.NewMenuRepo(db)
	settingRepo := repo.NewSettingRepo(db)
	statsRepo := repo.NewStatsRepo(db)
	jobRepo := repo.NewJobRepo(db)
	chatRepo := repo.NewChatRepo(db)
	objectRepo := repo.NewObjectRepo(minioClient, cfg.MinIO.Bucket)
	mqRepo := repo.NewMQRepo(mqConn, cfg.RabbitMQ.QueueDocIndex)
	notificationRepo := repo.NewNotificationRepo(db)
	loginLogRepo := repo.NewLoginLogRepo(db)
	providerRepo := repo.NewModelProviderRepo(db)

	notificationService := service.NewNotificationService(notificationRepo)
	loginLogService := service.NewLoginLogService(loginLogRepo)
	jobService := service.NewJobService(jobRepo, knowledgeRepo, mqRepo)
	authService := service.NewAuthService(userRepo, roleRepo, signer, tokenRepo, loginLogService, notificationService)
	roleService := service.NewRoleService(roleRepo, userRepo, providerRepo)
	menuService := service.NewMenuService(menuRepo, roleService)
	userAdminService := service.NewUserAdminService(userRepo, roleRepo, providerRepo, notificationService)
	auditService := service.NewAuditService(auditRepo, userRepo)
	outlineService := service.NewOutlineService(outlineRepo, projectRepo)
	continuityService := service.NewContinuityService(continuityRepo, projectRepo)
	projectService := service.NewProjectService(projectRepo, volumeRepo, chapterRepo, db, service.ProjectCascadeDeps{
		CharacterRepo:  characterRepo,
		LoreRepo:       loreRepo,
		OutlineRepo:    outlineRepo,
		ContinuityRepo: continuityRepo,
		ChatRepo:       chatRepo,
		KnowledgeRepo:  knowledgeRepo,
		ObjectRepo:     objectRepo,
		Manticore:      manticoreClient,
		ManticoreTable: cfg.Manticore.TableChunks,
	})
	volumeService := service.NewVolumeService(volumeRepo, chapterRepo, projectRepo, db)
	chapterService := service.NewChapterService(chapterRepo, volumeRepo, projectRepo, db)
	characterService := service.NewCharacterService(characterRepo, projectRepo, db)
	loreService := service.NewLoreService(loreRepo, projectRepo)
	promptService := service.NewPromptService(promptRepo, projectRepo)
	toolService := service.NewToolService(repo.NewToolRepo(db), projectRepo, chapterRepo, characterRepo, loreRepo, continuityRepo)
	settingService := service.NewSettingService(settingRepo)
	statsService := service.NewStatsService(statsRepo, projectRepo)

	aiTimeoutSec := cfg.AI.TimeoutSec
	if aiTimeoutSec <= 0 {
		aiTimeoutSec = 60 // 默认模型调用超时（秒）
	}
	aiClient := ai.NewClient()
	usageRepo := repo.NewModelUsageRepo(db)
	usageService := service.NewModelUsageService(usageRepo, ctx)
	aiClient.SetUsageRecorder(usageService)
	providerService := service.NewModelProviderService(providerRepo, roleRepo, userRepo, aiClient,
		cfg.JWT.Secret, time.Duration(aiTimeoutSec)*time.Second)
	// 供应商表为空且配置了网关时播种默认供应商（配置文件即"出厂默认"，之后在页面上管理）。
	if cfg.AI.BaseURL != "" && cfg.AI.ChatModel != "" {
		if err := providerService.SeedDefault(ctx, service.ProviderSeed{
			ProviderName:  "默认模型网关",
			ProtocolCode:  cfg.AI.ProtocolCode,
			BaseURL:       cfg.AI.BaseURL,
			APIKey:        cfg.AI.APIKey,
			ChatModelName: cfg.AI.ChatModel,
			Temperature:   cfg.AI.Temperature,
			MaxTokens:     cfg.AI.MaxTokens,
			TimeoutSec:    aiTimeoutSec,
		}); err != nil {
			pkg.SLogger().Warnw("默认模型供应商播种失败，可在页面手工添加", "error", err)
		}
	}
	// 旧版「供应商自带一个模型」数据升级为模型目录（幂等），失败仅告警不阻断启动。
	if err := providerService.MigrateLegacyModels(ctx); err != nil {
		pkg.SLogger().Warnw("旧版模型数据迁移失败，可在模型供应商页手工补齐", "error", err)
	}
	knowledgeConfig := service.KnowledgeIndexConfig{
		UploadMaxMB:    cfg.App.UploadMaxMB,
		ChunkSize:      cfg.App.ChunkSize,
		ChunkOverlap:   cfg.App.ChunkOverlap,
		RAGTopK:        cfg.App.RAGTopK,
		ManticoreTable: cfg.Manticore.TableChunks,
	}
	knowledgeService := service.NewKnowledgeService(knowledgeRepo, objectRepo, mqRepo, jobRepo, projectRepo,
		settingRepo, aiClient, providerService, manticoreClient, db, knowledgeConfig)
	chatService := service.NewChatService(chatRepo, projectRepo, chapterRepo, characterRepo, knowledgeRepo,
		continuityRepo, settingRepo, promptService, knowledgeService, toolService, providerService, aiClient, db,
		service.ChatConfig{HistoryCount: cfg.AI.HistoryCount})

	// 内置资产与默认配置播种（幂等），失败视为启动期数据库异常，直接退出。
	if err := toolService.SeedDefaults(ctx); err != nil {
		pkg.SLogger().Errorw("内置工具播种失败", "error", err)
		os.Exit(1)
	}
	if err := promptService.SeedDefaults(ctx); err != nil {
		pkg.SLogger().Errorw("内置提示词播种失败", "error", err)
		os.Exit(1)
	}
	if err := settingService.SeedDefaults(ctx); err != nil {
		pkg.SLogger().Errorw("默认配置播种失败", "error", err)
		os.Exit(1)
	}
	if err := roleService.SeedDefaults(ctx); err != nil {
		pkg.SLogger().Errorw("内置角色播种失败", "error", err)
		os.Exit(1)
	}
	if err := menuService.SeedDefaults(ctx); err != nil {
		pkg.SLogger().Errorw("内置菜单播种失败", "error", err)
		os.Exit(1)
	}
	// 用户表为空时引导创建管理员账号，密码取自配置（生产必改）。
	if err := userAdminService.BootstrapAdmin(ctx, "admin", "admin@novelworkbench.local", cfg.App.AdminPassword); err != nil {
		pkg.SLogger().Errorw("管理员引导失败", "error", err)
		os.Exit(1)
	}

	// 知识库索引 worker：消费 RabbitMQ 完成文档索引，随 ctx 取消优雅退出。
	knowledgeWorker := service.NewKnowledgeWorker(knowledgeRepo, objectRepo, mqRepo, jobRepo, projectRepo,
		manticoreClient, db, knowledgeConfig, notificationService)
	workerDone := make(chan struct{})
	go func() {
		defer close(workerDone)
		if err := knowledgeWorker.Run(ctx); err != nil {
			pkg.SLogger().Warnw("知识库索引 worker 异常退出", "error", err)
		}
	}()

	authMW := pkg.AuthMiddleware(signer, tokenRepo, func(checkCtx context.Context, userULID string) (bool, error) {
		user, err := userRepo.GetByULID(checkCtx, userULID)
		if err != nil {
			return false, err
		}
		return user != nil && user.UserStatusCode != domain.UserStatusDisabled, nil
	}, []string{"/api/v1/auth/login"})
	auditMW := pkg.AuditMiddleware(auditService, []string{
		"/api/v1/auth/login", "/api/v1/auth/logout", "/api/v1/chat/send",
	})
	statusService := service.NewSystemStatusService(service.SystemProbeDeps{
		PostgresProbe: func(probeCtx context.Context) error {
			sqlDB, err := db.DB()
			if err != nil {
				return err
			}
			return sqlDB.PingContext(probeCtx)
		},
		RedisProbe: func(probeCtx context.Context) error {
			if rdb == nil {
				return errors.New("redis 未配置")
			}
			return rdb.Ping(probeCtx).Err()
		},
		RabbitMQProbe: func(probeCtx context.Context) error {
			if mqConn == nil || mqConn.IsClosed() {
				return errors.New("rabbitmq 连接不可用")
			}
			return mqRepo.DeclareQueue(probeCtx)
		},
		MinIOProbe: func(probeCtx context.Context) error {
			if minioClient == nil {
				return errors.New("minio 客户端未初始化")
			}
			exists, err := minioClient.BucketExists(probeCtx, cfg.MinIO.Bucket)
			if err != nil {
				return err
			}
			if !exists {
				return errors.New("对象存储桶不存在")
			}
			return nil
		},
		ManticoreProbe: func(probeCtx context.Context) error {
			return manticoreClient.Ping(probeCtx)
		},
		BuildVersion: cfg.App.BuildVersion,
	}, providerService)
	bootstrapService := service.NewBootstrapService(providerService, aiClient, projectRepo, volumeRepo,
		chapterRepo, characterRepo, loreRepo, outlineRepo, db)
	router := api.NewRouter(cfg.Server.Mode, api.RouterDeps{
		AuthMW:      authMW,
		AuditMW:     auditMW,
		PermChecker: roleService,
		ReadyCheck: func() error {
			sqlDB, err := db.DB()
			if err != nil {
				return err
			}
			pingCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			return sqlDB.PingContext(pingCtx)
		},
		AuthAPI:       api.NewAuthAPI(authService),
		ProjectAPI:    api.NewProjectAPI(projectService),
		BootstrapAPI:  api.NewBootstrapAPI(bootstrapService),
		VolumeAPI:     api.NewVolumeAPI(volumeService),
		ChapterAPI:    api.NewChapterAPI(chapterService),
		CharacterAPI:  api.NewCharacterAPI(characterService),
		LoreAPI:       api.NewLoreAPI(loreService),
		KnowledgeAPI:  api.NewKnowledgeAPI(knowledgeService),
		OutlineAPI:    api.NewOutlineAPI(outlineService),
		ContinuityAPI: api.NewContinuityAPI(continuityService),
		PromptAPI:     api.NewPromptAPI(promptService),
		MenuAPI:       api.NewMenuAPI(menuService),
		SettingAPI:    api.NewSettingAPI(settingService),
		StatsAPI:      api.NewStatsAPI(statsService),
		ChatAPI:       api.NewChatAPI(chatService),
		ToolAPI:       api.NewToolAPI(toolService),
		UserAdminAPI:  api.NewUserAdminAPI(userAdminService),
		RoleAPI:       api.NewRoleAPI(roleService),
		AuditAPI:      api.NewAuditAPI(auditService),
		ProviderAPI:   api.NewModelProviderAPI(providerService),
		UsageAPI:      api.NewModelUsageAPI(usageService),
		StatusAPI:     api.NewSystemStatusAPI(statusService),
		NotifyAPI:     api.NewNotificationAPI(notificationService),
		LoginLogAPI:   api.NewLoginLogAPI(loginLogService),
		JobAPI:        api.NewJobAPI(jobService),
	})

	server := &http.Server{
		Addr:         cfg.ServerAddr(),
		Handler:      router,
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeoutSec) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeoutSec) * time.Second, // 0 表示不限（SSE 长连接）
	}
	serveErr := make(chan error, 1)
	go func() {
		pkg.SLogger().Infow("服务已启动", "addr", cfg.ServerAddr())
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
		}
	}()

	select {
	case <-ctx.Done():
		pkg.SLogger().Infow("收到退出信号，开始优雅关闭")
	case err := <-serveErr:
		pkg.SLogger().Errorw("HTTP 服务异常退出", "error", err)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		pkg.SLogger().Warnw("HTTP 服务关闭超时", "error", err)
	}
	select {
	case <-workerDone:
	case <-time.After(5 * time.Second):
		pkg.SLogger().Warnw("知识库索引 worker 关闭超时")
	}
	if sqlDB, err := db.DB(); err == nil {
		if err := sqlDB.Close(); err != nil {
			pkg.SLogger().Warnw("关闭 Postgres 连接失败", "error", err)
		}
	}
	if err := rdb.Close(); err != nil {
		pkg.SLogger().Warnw("关闭 Redis 连接失败", "error", err)
	}
	if mqConn != nil && !mqConn.IsClosed() {
		if err := mqConn.Close(); err != nil {
			pkg.SLogger().Warnw("关闭 RabbitMQ 连接失败", "error", err)
		}
	}
	pkg.SLogger().Infow("服务已关闭")
}
