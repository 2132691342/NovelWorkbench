package api

import (
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
)

// RouterDeps 路由注册的依赖聚合（由 main.go 装配注入）。
type RouterDeps struct {
	AuthMW       gin.HandlerFunc // 鉴权中间件（pkg.AuthMiddleware 产物）
	AuthAPI      *AuthAPI        // 认证模块处理器
	ProjectAPI   *ProjectAPI     // 项目模块处理器
	VolumeAPI    *VolumeAPI      // 卷模块处理器
	ChapterAPI   *ChapterAPI     // 章节模块处理器
	CharacterAPI *CharacterAPI   // 角色模块处理器
	LoreAPI      *LoreAPI        // 设定模块处理器
	KnowledgeAPI *KnowledgeAPI   // 知识库模块处理器
	SlashAPI     *SlashAPI       // 斜杠命令模块处理器
	SettingAPI   *SettingAPI     // 应用配置模块处理器
	StatsAPI     *StatsAPI       // 数据总览模块处理器
	ChatAPI      *ChatAPI        // AI 对话模块处理器
}

// NewRouter 装配 HTTP 路由；中间件顺序：Recovery → 请求ID → 日志 → 方法白名单 → CORS → 鉴权。
func NewRouter(mode string, deps RouterDeps) *gin.Engine {
	gin.SetMode(mode)
	router := gin.New()
	router.Use(pkg.RecoveryMiddleware())
	router.Use(pkg.RequestIDMiddleware())
	router.Use(pkg.LoggerMiddleware())
	router.Use(pkg.MethodWhitelistMiddleware())
	router.Use(cors.Default())
	router.GET("/health", healthCheck)

	v1 := router.Group("/api/v1")
	v1.Use(deps.AuthMW)
	registerAuthRoutes(v1, deps.AuthAPI)
	registerProjectRoutes(v1, deps.ProjectAPI)
	registerVolumeRoutes(v1, deps.VolumeAPI)
	registerChapterRoutes(v1, deps.ChapterAPI)
	registerCharacterRoutes(v1, deps.CharacterAPI)
	registerLoreRoutes(v1, deps.LoreAPI)
	registerKnowledgeRoutes(v1, deps.KnowledgeAPI)
	registerSlashRoutes(v1, deps.SlashAPI)
	registerSettingRoutes(v1, deps.SettingAPI)
	registerStatsRoutes(v1, deps.StatsAPI)
	registerChatRoutes(v1, deps.ChatAPI)
	return router
}

// healthCheck 容器健康探针。
func healthCheck(c *gin.Context) {
	pkg.Ok(c, gin.H{"status": "up"})
}

// registerAuthRoutes 注册认证路由（register/login 免鉴权，由中间件白名单放行）。
func registerAuthRoutes(rg *gin.RouterGroup, handler *AuthAPI) {
	rg.POST("/auth/register", handler.Register)
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
	rg.GET("/project/detail", handler.Detail)
	rg.GET("/project/tree", handler.Tree)
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
	rg.GET("/chapter/detail", handler.Detail)
	rg.GET("/chapter/snapshot/list", handler.SnapshotList)
	rg.POST("/chapter/snapshot/create", handler.SnapshotCreate)
	rg.POST("/chapter/snapshot/restore", handler.SnapshotRestore)
}

// registerCharacterRoutes 注册角色路由。
func registerCharacterRoutes(rg *gin.RouterGroup, handler *CharacterAPI) {
	rg.POST("/character/save", handler.Save)
	rg.POST("/character/delete", handler.Delete)
	rg.POST("/character/list", handler.List)
	rg.GET("/character/graph", handler.Graph)
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
	rg.GET("/knowledge/chunk/list", handler.ChunkList)
	rg.GET("/knowledge/content", handler.Content)
}

// registerSlashRoutes 注册斜杠命令路由。
func registerSlashRoutes(rg *gin.RouterGroup, handler *SlashAPI) {
	rg.GET("/slash/list", handler.List)
	rg.POST("/slash/save", handler.Save)
	rg.POST("/slash/delete", handler.Delete)
	rg.POST("/slash/parse", handler.Parse)
}

// registerSettingRoutes 注册应用配置路由。
func registerSettingRoutes(rg *gin.RouterGroup, handler *SettingAPI) {
	rg.GET("/setting/list", handler.List)
	rg.POST("/setting/update", handler.Update)
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
