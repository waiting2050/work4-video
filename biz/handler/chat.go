package handler

import (
	"context"
	"encoding/json"
	"log"
	"sync"

	"video/biz/service"
	"video/biz/utils"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/hertz-contrib/websocket"
)

// upgrader WebSocket升级器，用于将HTTP连接升级为WebSocket连接
var upgrader = websocket.HertzUpgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(ctx *app.RequestContext) bool {
		return true // 允许所有来源
	},
}

// WSManager WebSocket连接管理器
type WSManager struct {
	clients map[string]*websocket.Conn // 用户ID -> WebSocket连接
	mu      sync.RWMutex                 // 读写锁，保证并发安全
}

// wsManager 全局WebSocket连接管理器实例
var wsManager = WSManager{
	clients: make(map[string]*websocket.Conn),
}

// ChatHandler 聊天处理器
type ChatHandler struct {
	chatService *service.ChatService
}

// NewChatHandler 创建聊天处理器实例
func NewChatHandler(chatService *service.ChatService) *ChatHandler {
	return &ChatHandler{chatService: chatService}
}

// WSMessage WebSocket消息格式（前端发送）
type WSMessage struct {
	Type    int             `json:"type"`    // 消息类型：1-私聊 2-私聊历史 3-未读消息 4-群聊 5-群聊历史
	Payload json.RawMessage `json:"payload"` // 具体数据
}

// PrivateMessagePayload 私聊消息请求数据
type PrivateMessagePayload struct {
	ReceiverID string `json:"receiver_id"` // 接收者用户ID
	Content    string `json:"content"`    // 消息内容
}

// PrivateHistoryPayload 获取私聊历史请求数据
type PrivateHistoryPayload struct {
	TargetID string `json:"target_id"` // 对方用户ID
	Page     int    `json:"page"`     // 页码（从1开始）
	PageSize int    `json:"page_size"` // 每页数量
}

// PrivateUnreadPayload 获取未读消息请求数据
type PrivateUnreadPayload struct {
	TargetID string `json:"target_id"` // 对方用户ID
}

// GroupMessagePayload 群聊消息请求数据
type GroupMessagePayload struct {
	RoomID  string `json:"room_id"` // 群聊房间ID
	Content string `json:"content"` // 消息内容
}

// GroupHistoryPayload 获取群聊历史请求数据
type GroupHistoryPayload struct {
	RoomID   string `json:"room_id"` // 群聊房间ID
	Page     int    `json:"page"`    // 页码（从1开始）
	PageSize int    `json:"page_size"` // 每页数量
}

// WSResponse WebSocket响应格式（返回给前端）
type WSResponse struct {
	Type    int         `json:"type"`    // 消息类型，与请求type一致
	Success bool        `json:"success"` // 是否成功
	Data    interface{} `json:"data,omitempty"` // 成功时返回的数据
	Error   string      `json:"error,omitempty"` // 失败时返回的错误信息
}

// WebSocket WebSocket连接处理入口
func (h *ChatHandler) WebSocket(ctx context.Context, c *app.RequestContext) {
	// 从认证中间件获取当前用户ID
	userID, ok := utils.GetUserID(c)
	if !ok {
		return
	}

	// 将HTTP连接升级为WebSocket连接
	err := upgrader.Upgrade(c, func(conn *websocket.Conn) {
		// 连接建立成功，保存到管理器
		wsManager.mu.Lock()
		wsManager.clients[userID] = conn
		wsManager.mu.Unlock()

		// 连接关闭时清理
		defer func() {
			wsManager.mu.Lock()
			delete(wsManager.clients, userID)
			wsManager.mu.Unlock()
		}()

		// 循环读取消息
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				log.Printf("[ChatHandler] Read error: %v", err)
				break
			}

			// 处理消息
			h.handleMessage(userID, msg, conn)
		}
	})

	if err != nil {
		log.Printf("[ChatHandler] Upgrade error: %v", err)
	}
}

// handleMessage 根据消息类型分发处理
func (h *ChatHandler) handleMessage(userID string, msg []byte, conn *websocket.Conn) {
	var wsMsg WSMessage
	if err := json.Unmarshal(msg, &wsMsg); err != nil {
		sendError(conn, 0, "Invalid message format")
		return
	}

	// 根据type分发到对应的处理函数
	switch wsMsg.Type {
	case 1: // 发送私聊消息
		h.handlePrivateMessage(userID, wsMsg.Payload, conn)
	case 2: // 获取私聊历史
		h.handlePrivateHistory(userID, wsMsg.Payload, conn)
	case 3: // 获取未读消息
		h.handlePrivateUnread(userID, wsMsg.Payload, conn)
	case 4: // 发送群聊消息
		h.handleGroupMessage(userID, wsMsg.Payload, conn)
	case 5: // 获取群聊历史
		h.handleGroupHistory(userID, wsMsg.Payload, conn)
	default:
		sendError(conn, wsMsg.Type, "Unknown message type")
	}
}

// handlePrivateMessage 处理发送私聊消息
func (h *ChatHandler) handlePrivateMessage(userID string, payload json.RawMessage, conn *websocket.Conn) {
	var p PrivateMessagePayload
	if err := json.Unmarshal(payload, &p); err != nil {
		sendError(conn, 1, "Invalid payload")
		return
	}

	// 调用服务层发送消息
	message, err := h.chatService.SendPrivateMessage(userID, p.ReceiverID, p.Content)
	if err != nil {
		sendError(conn, 1, "Failed to send message")
		return
	}

	// 返回成功响应给发送者
	sendSuccess(conn, 1, message)

	// 如果接收者在线，实时推送给接收者
	wsManager.mu.RLock()
	if receiverConn, ok := wsManager.clients[p.ReceiverID]; ok {
		sendSuccess(receiverConn, 1, message)
	}
	wsManager.mu.RUnlock()
}

// handlePrivateHistory 处理获取私聊历史
func (h *ChatHandler) handlePrivateHistory(userID string, payload json.RawMessage, conn *websocket.Conn) {
	var p PrivateHistoryPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		sendError(conn, 2, "Invalid payload")
		return
	}
	// 设置默认值
	if p.Page < 1 {
		p.Page = 1
	}
	if p.PageSize < 1 {
		p.PageSize = 20
	}

	// 调用服务层获取历史记录
	messages, total, err := h.chatService.GetPrivateHistory(userID, p.TargetID, p.Page, p.PageSize)
	if err != nil {
		sendError(conn, 2, "Failed to get history")
		return
	}

	// 返回数据
	sendSuccess(conn, 2, map[string]interface{}{
		"items": messages,
		"total": total,
	})
}

// handlePrivateUnread 处理获取未读消息
func (h *ChatHandler) handlePrivateUnread(userID string, payload json.RawMessage, conn *websocket.Conn) {
	var p PrivateUnreadPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		sendError(conn, 3, "Invalid payload")
		return
	}

	// 调用服务层获取未读消息
	messages, err := h.chatService.GetPrivateUnread(userID, p.TargetID)
	if err != nil {
		sendError(conn, 3, "Failed to get unread")
		return
	}

	sendSuccess(conn, 3, map[string]interface{}{
		"items": messages,
	})
}

// handleGroupMessage 处理发送群聊消息
func (h *ChatHandler) handleGroupMessage(userID string, payload json.RawMessage, conn *websocket.Conn) {
	var p GroupMessagePayload
	if err := json.Unmarshal(payload, &p); err != nil {
		sendError(conn, 4, "Invalid payload")
		return
	}

	// 调用服务层发送群聊消息
	message, err := h.chatService.SendGroupMessage(userID, p.RoomID, p.Content)
	if err != nil {
		sendError(conn, 4, "Failed to send group message")
		return
	}

	// 返回成功响应
	sendSuccess(conn, 4, message)
}

// handleGroupHistory 处理获取群聊历史
func (h *ChatHandler) handleGroupHistory(userID string, payload json.RawMessage, conn *websocket.Conn) {
	var p GroupHistoryPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		sendError(conn, 5, "Invalid payload")
		return
	}
	if p.Page < 1 {
		p.Page = 1
	}
	if p.PageSize < 1 {
		p.PageSize = 20
	}

	// 调用服务层获取历史记录
	messages, total, err := h.chatService.GetGroupHistory(p.RoomID, p.Page, p.PageSize)
	if err != nil {
		sendError(conn, 5, "Failed to get group history")
		return
	}

	sendSuccess(conn, 5, map[string]interface{}{
		"items": messages,
		"total": total,
	})
}

// sendSuccess 发送成功响应
func sendSuccess(conn *websocket.Conn, msgType int, data interface{}) {
	resp := WSResponse{
		Type:    msgType,
		Success: true,
		Data:    data,
	}
	sendJSON(conn, resp)
}

// sendError 发送错误响应
func sendError(conn *websocket.Conn, msgType int, errMsg string) {
	resp := WSResponse{
		Type:    msgType,
		Success: false,
		Error:   errMsg,
	}
	sendJSON(conn, resp)
}

// sendJSON 将对象序列化为JSON并通过WebSocket发送
func sendJSON(conn *websocket.Conn, v interface{}) {
	if data, err := json.Marshal(v); err == nil {
		conn.WriteMessage(websocket.TextMessage, data)
	}
}
