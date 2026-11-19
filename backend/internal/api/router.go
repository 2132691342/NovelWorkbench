// 本文件负责 HTTP 路由装配：中间件链、路由分组注册与健康探针。

package api

import (
	"net/http"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
)

// RouterDeps 路由注册的依赖聚合（由 main.go 装配注入）。
type RouterDeps struct {
	AuthMW        gin.HandlerFunc   // 鉴权中间件（pkg.AuthMiddleware 产物）
	AuditMW       gin.HandlerFunc   // 审计中间件（pkg.AuditMiddleware 产物）
	PermChecker   pkg.PermChecker   // 权限判定（admin 路由收口）
	ReadyCheck    func() error      // 就绪探针（DB Ping）
	AuthAPI       *AuthAPI          // 认证模块处理器
	ProjectAPI    *ProjectAPI       // 项目模块处理器
	VolumeAPI     *VolumeAPI        // 卷模块处理器
	ChapterAPI    *ChapterAPI       // 章节模块处理器
	CharacterAPI  *CharacterAPI     // 角色模块处理器
	LoreAPI       *LoreAPI          // 设定模块处理器
	KnowledgeAPI  *KnowledgeAPI     // 知识库模块处理器
	OutlineAPI    *OutlineAPI       // 创作规划模块处理器
	ContinuityAPI *ContinuityAPI    // 连续性看板模块处理器
	SkillAPI      *SkillAPI         // 技能模块处理器
	MenuAPI       *MenuAPI          // 菜单与权限目录模块处理器
	SettingAPI    *SettingAPI       // 应用配置模块处理器
	StatsAPI      *StatsAPI         // 数据总览模块处理器
	ChatAPI       *ChatAPI          // AI 对话模块处理器
	UserAdminAPI  *UserAdminAPI     // 用户管理模块处理器
	RoleAPI       *RoleAPI          // 角色管理模块处理器
	AuditAPI      *AuditAPI         // 审计日志模块处理器
	ProviderAPI   *ModelProviderAPI // 模型供应商模块处理器
	UsageAPI      *ModelUsageAPI    // 模型用量模块处理器
	StatusAPI     *SystemStatusAPI  // 系统运行状态模块处理器
}

// NewRouter 装配 HTTP 路由；中间件顺序：Recovery → 请求ID → 日志 → CORS（先处理预检）→ 方法白名单 → 鉴权 → 审计。
func NewRouter(mode string, deps RouterDeps) *gin.Engine {
	gin.SetMode(mode)
	router := gin.New()
	router.Use(pkg.RecoveryMiddleware())
	router.Use(pkg.RequestIDMiddleware())
	router.Use(pkg.LoggerMiddleware())
	router.Use(cors.New(cors.Config{
		AllowAllOrigins: true,
		AllowMethods:    []string{http.MethodGet, http.MethodPost, http.MethodOptions},
		AllowHeaders:    []string{"Origin", "Content-Type", "Authorization", "X-Request-ID"},
		ExposeHeaders:   []string{"X-Request-ID"},
		MaxAge:          12 * time.Hour,
	}))
	router.Use(pkg.MethodWhitelistMiddleware())
	router.GET("/health", healthCheck)
	router.GET("/ready", readyCheck(deps.ReadyCheck))

	v1 := router.Group("/api/v1")
	v1.Use(deps.AuthMW)
	v1.Use(deps.AuditMW)
	registerAuthRoutes(v1, deps.AuthAPI)
	registerProjectRoutes(v1, deps.ProjectAPI)
	registerVolumeRoutes(v1, deps.VolumeAPI)
	registerChapterRoutes(v1, deps.ChapterAPI)
	registerCharacterRoutes(v1, deps.CharacterAPI)
	registerLoreRoutes(v1, deps.LoreAPI)
	registerKnowledgeRoutes(v1, deps.KnowledgeAPI)
	registerOutlineRoutes(v1, deps.OutlineAPI)
	registerContinuityRoutes(v1, deps.ContinuityAPI)
	registerSkillRoutes(v1, deps)
	registerSettingRoutes(v1, deps)
	registerStatsRoutes(v1, deps.StatsAPI)
	registerChatRoutes(v1, deps.ChatAPI)
	registerModelRoutes(v1, deps)
	registerMenuRoutes(v1, deps)
	registerAdminRoutes(v1, deps)
	return router
}

// registerModelRoutes 注册模型生态路由（供应商管理与能力探测对所有登录用户开放，写操作收口到管理权限）。
func registerModelRoutes(rg *gin.RouterGroup, deps RouterDeps) {
	modelGroup := rg.Group("/model")
	modelGroup.GET("/capability", deps.ProviderAPI.Capability)
	modelGroup.GET("/chat_options", deps.ProviderAPI.ChatOptions)
	modelGroup.POST("/provider/list", deps.ProviderAPI.List)
	providerWrite := modelGroup.Group("/provider")
	providerWrite.Use(pkg.RequirePermMiddleware(deps.PermChecker, domain.PermModelManage))
	{
		providerWrite.POST("/save", deps.ProviderAPI.Save)
		providerWrite.POST("/delete", deps.ProviderAPI.Delete)
		providerWrite.POST("/test", deps.ProviderAPI.Test)
	}
	usageGroup := rg.Group("/model/usage")
	usageGroup.Use(pkg.RequirePermMiddleware(deps.PermChecker, domain.PermUsageRead))
	{
		usageGroup.GET("/overview", deps.UsageAPI.Overview)
	}
	systemGroup := rg.Group("/system")
	systemGroup.Use(pkg.RequirePermMiddleware(deps.PermChecker, domain.PermSystemRead))
	{
		systemGroup.GET("/status", deps.StatusAPI.Status)
	}
}

// healthCheck 容器健康探针（只回存活，不做依赖检查）。
func healthCheck(c *gin.Context) {
	pkg.Ok(c, gin.H{"status": "up"})
}

// readyCheck 就绪探针（Ping 关键依赖，失败回 503 语义的业务码）。
func readyCheck(check func() error) gin.HandlerFunc {
	return func(c *gin.Context) {
		if check != nil {
			if err := check(); err != nil {
				pkg.Fail(c, domain.RespCodeStorageError, "依赖未就绪")
				return
			}
		}
		pkg.Ok(c, gin.H{"status": "ready"})
	}
}

// registerAdminRoutes 注册系统管理路由（按权限码收口：用户/角色/审计）。
func registerAdminRoutes(rg *gin.RouterGroup, deps RouterDeps) {
	admin := rg.Group("/admin")
	admin.Use(pkg.RequirePermMiddleware(deps.PermChecker, domain.PermUserManage))
	{
		admin.POST("/user/list", deps.UserAdminAPI.List)
		admin.POST("/user/create", deps.UserAdminAPI.Create)
		admin.POST("/user/update", deps.UserAdminAPI.Update)
		admin.POST("/user/delete", deps.UserAdminAPI.Delete)
		admin.POST("/user/reset_password", deps.UserAdminAPI.ResetPassword)
	}
	roleGroup := rg.Group("/admin/role")
	roleGroup.Use(pkg.RequirePermMiddleware(deps.PermChecker, domain.PermRoleManage))
	{
		roleGroup.POST("/list", deps.RoleAPI.List)
		roleGroup.POST("/save", deps.RoleAPI.Save)
		roleGroup.POST("/delete", deps.RoleAPI.Delete)
	}
	auditGroup := rg.Group("/admin/audit")
	auditGroup.Use(pkg.RequirePermMiddleware(deps.PermChecker, domain.PermAuditRead))
	{
		auditGroup.POST("/list", deps.AuditAPI.List)
	}
}

// registerAuthRoutes 注册认证路由（login 免鉴权，由中间件白名单放行；系统不开放注册）。
func registerAuthRoutes(rg *gin.RouterGroup, handler *AuthAPI) {
	rg.POST("/auth/login", handler.Login)
	rg.GET("/auth/profile", handler.Profile)
	rg.POST("/auth/logout", handler.Logout)
}

// registerProjectRoutes 注册项目路由。
func registerProjectRoutes(rg *gin.RouterGroup, handler *ProjectAPI) {
	rg.POST("/project/create", handler.Create)
	rg.POST("/project/update", handler.Update)
	rg.POST("/project/delete", handler.Delete)
	rg.POST("/project/list", handler.List)
	rg.GET("/project/detail/:ProjectULID", handler.Detail)
	rg.GET("/project/tree/:ProjectULID", handler.Tree)
}

// registerVolumeRoutes 注册卷路由。
func registerVolumeRoutes(rg *gin.RouterGroup, handler *VolumeAPI) {
	rg.POST("/volume/create", handler.Create)
	rg.POST("/volume/update", handler.Update)
	rg.POST("/volume/delete", handler.Delete)
}

// registerChapterRoutes 注册章节路由。
func registerChapterRoutes(rg *gin.RouterGroup, handler *ChapterAPI) {
	rg.POST("/chapter/create", handler.Create)
	rg.POST("/chapter/update", handler.Update)
	rg.POST("/chapter/delete", handler.Delete)
	rg.GET("/chapter/detail/:ChapterULID", handler.Detail)
	rg.GET("/chapter/snapshot/list/:ChapterULID", handler.SnapshotList)
	rg.POST("/chapter/snapshot/create", handler.SnapshotCreate)
	rg.POST("/chapter/snapshot/restore", handler.SnapshotRestore)
}

// registerCharacterRoutes 注册角色路由。
func registerCharacterRoutes(rg *gin.RouterGroup, handler *CharacterAPI) {
	rg.POST("/character/save", handler.Save)
	rg.POST("/character/delete", handler.Delete)
	rg.POST("/character/list", handler.List)
	rg.GET("/character/graph/:ProjectULID", handler.Graph)
	rg.POST("/character/relation/save", handler.RelationSave)
	rg.POST("/character/relation/delete", handler.RelationDelete)
}

// registerLoreRoutes 注册设定路由。
func registerLoreRoutes(rg *gin.RouterGroup, handler *LoreAPI) {
	rg.POST("/lore/save", handler.Save)
	rg.POST("/lore/delete", handler.Delete)
	rg.POST("/lore/list", handler.List)
}

// registerKnowledgeRoutes 注册知识库路由。
func registerKnowledgeRoutes(rg *gin.RouterGroup, handler *KnowledgeAPI) {
	rg.POST("/knowledge/upload", handler.Upload)
	rg.POST("/knowledge/list", handler.List)
	rg.POST("/knowledge/delete", handler.Delete)
	rg.POST("/knowledge/search", handler.Search)
	rg.POST("/knowledge/reindex", handler.Reindex)
	rg.GET("/knowledge/chunk/list/:DocULID", handler.ChunkList)
	rg.GET("/knowledge/content/:DocULID", handler.Content)
}

// registerSkillRoutes 注册技能路由（读对所有登录用户开放，写收口到 skill:manage）。
func registerSkillRoutes(rg *gin.RouterGroup, deps RouterDeps) {
	skillGroup := rg.Group("/skill")
	skillGroup.GET("/list", deps.SkillAPI.List)
	skillGroup.GET("/detail/:SkillULID", deps.SkillAPI.Detail)
	skillGroup.POST("/resolve", deps.SkillAPI.Resolve)
	skillGroup.GET("/reference/list", deps.SkillAPI.ReferenceList)
	skillWrite := skillGroup.Group("")
	skillWrite.Use(pkg.RequirePermMiddleware(deps.PermChecker, domain.PermSkillManage))
	{
		skillWrite.POST("/save", deps.SkillAPI.Save)
		skillWrite.POST("/delete", deps.SkillAPI.Delete)
		skillWrite.POST("/reference/save", deps.SkillAPI.ReferenceSave)
		skillWrite.POST("/reference/delete", deps.SkillAPI.ReferenceDelete)
	}
}

// registerMenuRoutes 注册菜单与权限目录路由（导航对登录用户开放，管理收口到权限码）。
func registerMenuRoutes(rg *gin.RouterGroup, deps RouterDeps) {
	rg.GET("/menu/nav", deps.MenuAPI.Nav)
	menuGroup := rg.Group("/menu")
	menuGroup.Use(pkg.RequirePermMiddleware(deps.PermChecker, domain.PermMenuManage))
	{
		menuGroup.GET("/list", deps.MenuAPI.List)
		menuGroup.GET("/detail/:MenuULID", deps.MenuAPI.Detail)
		menuGroup.POST("/save", deps.MenuAPI.Save)
		menuGroup.POST("/delete", deps.MenuAPI.Delete)
	}
	permGroup := rg.Group("/menu/permission")
	permGroup.Use(pkg.RequirePermMiddleware(deps.PermChecker, domain.PermPermissionRead))
	{
		permGroup.GET("/catalog", deps.MenuAPI.PermCatalog)
	}
}

// registerSettingRoutes 注册应用配置路由（读取开放，写入收口到 system:setting:manage）。
func registerSettingRoutes(rg *gin.RouterGroup, deps RouterDeps) {
	rg.GET("/setting/list", deps.SettingAPI.List)
	settingWrite := rg.Group("/setting")
	settingWrite.Use(pkg.RequirePermMiddleware(deps.PermChecker, domain.PermSettingManage))
	{
		settingWrite.POST("/update", deps.SettingAPI.Update)
	}
}

// registerStatsRoutes 注册数据总览路由。
func registerStatsRoutes(rg *gin.RouterGroup, handler *StatsAPI) {
	rg.GET("/stats/overview", handler.Overview)
}

// registerChatRoutes 注册 AI 对话路由（/chat/send 为 SSE 流式响应）。
func registerChatRoutes(rg *gin.RouterGroup, handler *ChatAPI) {
	rg.POST("/chat/send", handler.Send)
	rg.POST("/chat/history", handler.History)
	rg.POST("/chat/session/create", handler.SessionCreate)
	rg.POST("/chat/session/list", handler.SessionList)
	rg.POST("/chat/session/delete", handler.SessionDelete)
}

// registerOutlineRoutes 注册创作规划路由（大纲树与场景卡）。
func registerOutlineRoutes(rg *gin.RouterGroup, handler *OutlineAPI) {
	rg.POST("/outline/node/save", handler.SaveNode)
	rg.POST("/outline/node/delete", handler.DeleteNode)
	rg.POST("/outline/list", handler.ListTree)
	rg.POST("/outline/scene/save", handler.SaveScene)
	rg.POST("/outline/scene/delete", handler.DeleteScene)
	rg.POST("/outline/scene/list", handler.ListScenes)
}

// registerContinuityRoutes 注册连续性看板路由（事实/伏笔/摘要）。
func registerContinuityRoutes(rg *gin.RouterGroup, handler *ContinuityAPI) {
	rg.POST("/continuity/fact/save", handler.SaveFact)
	rg.POST("/continuity/fact/review", handler.ReviewFact)
	rg.POST("/continuity/fact/list", handler.ListFacts)
	rg.POST("/continuity/hook/save", handler.SaveHook)
	rg.POST("/continuity/hook/list", handler.ListHooks)
	rg.POST("/continuity/summary/save", handler.SaveSummary)
	rg.POST("/continuity/summary/list", handler.ListSummaries)
}
