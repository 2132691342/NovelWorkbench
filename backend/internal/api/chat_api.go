// 本文件实现 AI 对话模块的 HTTP 处理器：SSE 流式发送、会话管理与历史查询。

package api

import (
	"encoding/json"
	"fmt"

	"github.com/gin-gonic/gin"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
	"github.com/2132691342/NovelWorkbench/backend/internal/service"
)

// ChatAPI AI 对话模块 HTTP 处理器（/chat/send 为 SSE 流式响应）。
type ChatAPI struct {
	svc *service.ChatService
}

// NewChatAPI 构造对话处理器。
func NewChatAPI(s *service.ChatService) *ChatAPI {
	return &ChatAPI{svc: s}
}

// Send 发送消息并流式返回生成结果（POST /chat/send，text/event-stream）。
// 流开始前失败输出 JSON 包络；流开始后失败经 SSE error 事件下发。
func (a *ChatAPI) Send(c *gin.Context) {
	var req domain.ChatSendREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "chat.send", bindErr(err))
		return
	}
	writer := &sseChatWriter{ctx: c}
	err := a.svc.Send(c.Request.Context(), c.GetString(pkg.ContextKeyUserULID), &req, writer)
	if err != nil && !writer.started {
		respondErr(c, "chat.send", err)
		return
	}
	if err != nil {
		pkg.SLogger().Errorw("SSE 流已开始，错误仅能记日志", "op", "chat.send", "request_id", c.GetString(pkg.ContextKeyRequestID), "error", err)
	}
}

// History 查询会话历史（POST /chat/history，created_at 升序全量）。
func (a *ChatAPI) History(c *gin.Context) {
	var req domain.ChatHistoryREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "chat.history", bindErr(err))
		return
	}
	page, err := a.svc.History(c.Request.Context(), c.GetString(pkg.ContextKeyUserULID), &req)
	if err != nil {
		respondErr(c, "chat.history", err)
		return
	}
	pkg.Ok(c, page)
}

// SessionCreate 创建会话（POST /chat/session/create）。
func (a *ChatAPI) SessionCreate(c *gin.Context) {
	var req domain.ChatSessionCreateREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "chat.session.create", bindErr(err))
		return
	}
	vo, err := a.svc.SessionCreate(c.Request.Context(), c.GetString(pkg.ContextKeyUserULID), &req)
	if err != nil {
		respondErr(c, "chat.session.create", err)
		return
	}
	pkg.Ok(c, vo)
}

// SessionList 查询项目会话列表（POST /chat/session/list）。
func (a *ChatAPI) SessionList(c *gin.Context) {
	var req domain.ChatSessionListREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "chat.session.list", bindErr(err))
		return
	}
	vos, err := a.svc.SessionList(c.Request.Context(), c.GetString(pkg.ContextKeyUserULID), &req)
	if err != nil {
		respondErr(c, "chat.session.list", err)
		return
	}
	pkg.Ok(c, vos)
}

// SessionDelete 删除会话并级联删消息（POST /chat/session/delete）。
func (a *ChatAPI) SessionDelete(c *gin.Context) {
	var req domain.ChatSessionDeleteREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "chat.session.delete", bindErr(err))
		return
	}
	if err := a.svc.SessionDelete(c.Request.Context(), c.GetString(pkg.ContextKeyUserULID), &req); err != nil {
		respondErr(c, "chat.session.delete", err)
		return
	}
	pkg.Ok(c, nil)
}

// sseChatWriter 将流式事件写为 text/event-stream：事件行 `data: {json}`，心跳为 `: ping` 注释行。
type sseChatWriter struct {
	ctx     *gin.Context // Gin 上下文
	started bool         // SSE 响应头是否已提交（提交后失败只能随流下发）
}

// Event 输出一条流式事件并刷新缓冲。
func (w *sseChatWriter) Event(dto domain.ChatStreamEventDTO) error {
	payload, err := json.Marshal(dto)
	if err != nil {
		return err
	}
	w.begin()
	if _, err := fmt.Fprintf(w.ctx.Writer, "data: %s\n\n", payload); err != nil {
		return err
	}
	w.ctx.Writer.Flush()
	return nil
}

// Ping 输出心跳注释行并刷新缓冲。
func (w *sseChatWriter) Ping() error {
	w.begin()
	if _, err := fmt.Fprint(w.ctx.Writer, ": ping\n\n"); err != nil {
		return err
	}
	w.ctx.Writer.Flush()
	return nil
}

// begin 首次输出前提交 SSE 响应头（禁缓存与反向代理缓冲）。
func (w *sseChatWriter) begin() {
	if w.started {
		return
	}
	w.ctx.Header("Content-Type", "text/event-stream; charset=utf-8")
	w.ctx.Header("Cache-Control", "no-cache")
	w.ctx.Header("X-Accel-Buffering", "no")
	w.started = true
}
