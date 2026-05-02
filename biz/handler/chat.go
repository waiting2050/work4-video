package handler

import (
	"context"
	"encoding/json"
	"log"
	"strings"
	"sync"
	"video/biz/cache"
	"video/biz/model"
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

// WSManager WebSocket连接管理器，用于管理所有在线用户的连接
type WSManager struct {
	clients map[string]*websocket.Conn // 用户ID -> WebSocket连接
	mu      sync.RWMutex               // 读写锁，保证并发安全
}

// wsManager 全局WebSocket连接管理器实例
var wsManager = WSManager{
	clients: make(map[string]*websocket.Conn),
}

// ChatHandler 聊天处理器，处理WebSocket聊天逻辑
type ChatHandler struct {
	chatService *service.ChatService // 聊天服务
	aiService   *service.AIService   // AI聊天服务
}

// NewChatHandler 创建聊天处理器实例
// chatService: 聊天服务
// aiService: AI聊天服务
func NewChatHandler(chatService *service.ChatService, aiService *service.AIService) *ChatHandler {
	return &ChatHandler{
		chatService: chatService,
		aiService:   aiService,
	}
}

// WSMessage WebSocket消息格式（前端发送）
type WSMessage struct {
	Type    int             `json:"type"`    // 消息类型
	Payload json.RawMessage `json:"payload"` // 具体数据
}

// 消息类型常量
const (
	MsgTypePrivateSend   = 1 // 发送私聊
	MsgTypePrivateHistory = 2 // 获取私聊历史
	MsgTypePrivateUnread  = 3 // 获取未读消息
	MsgTypeGroupSend     = 4 // 发送群聊
	MsgTypeGroupHistory  = 5 // 获取群聊历史
	MsgTypeUnreadCount   = 6 // 获取未读计数
	MsgTypeMarkRead      = 7 // 标记消息已读
	MsgTypeMarkRoomRead  = 8 // 标记房间已读
	MsgTypeOfflinePush   = 9 // 离线消息推送
	MsgTypeAIMessage     = 10 // AI消息
)

// PrivateMessagePayload 私聊消息请求数据
type PrivateMessagePayload struct {
	ReceiverID string `json:"receiver_id"` // 接收者用户ID
	Content    string `json:"content"`    // 消息内容
}

// PrivateHistoryPayload 获取私聊历史请求数据
type PrivateHistoryPayload struct {
	TargetID  string `json:"target_id"` // 对方用户ID
	Page      int    `json:"page"`      // 页码（从1开始）
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
	Page      int    `json:"page"`    // 页码（从1开始）
	PageSize int    `json:"page_size"` // 每页数量
}

// UnreadCountPayload 获取未读计数请求
type UnreadCountPayload struct {
	TargetID string `json:"target_id"`
}

// MarkReadPayload 标记已读请求
type MarkReadPayload struct {
	MessageID string `json:"message_id"`
}

// MarkRoomReadPayload 标记房间已读请求
type MarkRoomReadPayload struct {
	TargetID string `json:"target_id"` // 私聊对方ID，或者群聊RoomID
	RoomType string `json:"room_type"` // private 或 group
}

// WSResponse WebSocket响应格式（返回给前端）
type WSResponse struct {
	Type    int         `json:"type"`    // 消息类型，与请求type一致
	Success bool        `json:"success"` // 是否成功
	Data    interface{} `json:"data,omitempty"` // 成功时返回的数据
	Error   string      `json:"error,omitempty"` // 失败时返回的错误信息
}

// WebSocket WebSocket连接处理入口
// ctx: 上下文
// c: Hertz请求上下文
func (h *ChatHandler) WebSocket(ctx context.Context, c *app.RequestContext) {
	// 从认证中间件获取当前用户ID
	userID, ok := utils.GetUserID(c)
	if !ok {
		return
	}

	// 将HTTP连接升级为WebSocket连接
	err := upgrader.Upgrade(c, func(conn *websocket.Conn) {
		// 连接建立成功，设置用户在线状态
		_ = cache.SetUserOnline(userID)

		// 保存到管理器
		wsManager.mu.Lock()
		wsManager.clients[userID] = conn
		wsManager.mu.Unlock()

		// 用户上线，推送离线消息
		if offlineMsgs, err := h.chatService.GetOfflineMessages(userID); err == nil && len(offlineMsgs) > 0 {
			// 批量推送离线消息
			for _, msg := range offlineMsgs {
				sendSuccess(conn, MsgTypeOfflinePush, map[string]interface{}{
					"message": msg,
				})
			}
		}

		// 连接关闭时清理
		defer func() {
			wsManager.mu.Lock()
			delete(wsManager.clients, userID)
			wsManager.mu.Unlock()
			_ = cache.SetUserOffline(userID)
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
// userID: 当前用户ID
// msg: 原始消息字节
// conn: WebSocket连接
func (h *ChatHandler) handleMessage(userID string, msg []byte, conn *websocket.Conn) {
	var wsMsg WSMessage
	if err := json.Unmarshal(msg, &wsMsg); err != nil {
		sendError(conn, 0, "Invalid message format")
		return
	}

	// 根据type分发到对应的处理函数
	switch wsMsg.Type {
	case MsgTypePrivateSend: // 发送私聊消息
		h.handlePrivateMessage(userID, wsMsg.Payload, conn)
	case MsgTypePrivateHistory: // 获取私聊历史
		h.handlePrivateHistory(userID, wsMsg.Payload, conn)
	case MsgTypePrivateUnread: // 获取未读消息
		h.handlePrivateUnread(userID, wsMsg.Payload, conn)
	case MsgTypeGroupSend: // 发送群聊消息
		h.handleGroupMessage(userID, wsMsg.Payload, conn)
	case MsgTypeGroupHistory: // 获取群聊历史
		h.handleGroupHistory(userID, wsMsg.Payload, conn)
	case MsgTypeUnreadCount: // 获取未读计数
		h.handleUnreadCount(userID, wsMsg.Payload, conn)
	case MsgTypeMarkRead: // 标记消息已读
		h.handleMarkRead(userID, wsMsg.Payload, conn)
	case MsgTypeMarkRoomRead: // 标记房间已读
		h.handleMarkRoomRead(userID, wsMsg.Payload, conn)
	default:
		sendError(conn, wsMsg.Type, "Unknown message type")
	}
}

// handlePrivateMessage 处理发送私聊消息
// userID: 当前用户ID
// payload: 消息数据
// conn: WebSocket连接
func (h *ChatHandler) handlePrivateMessage(userID string, payload json.RawMessage, conn *websocket.Conn) {
	var p PrivateMessagePayload
	if err := json.Unmarshal(payload, &p); err != nil {
		sendError(conn, MsgTypePrivateSend, "Invalid payload")
		return
	}

	// 调用服务层发送消息
	message, err := h.chatService.SendPrivateMessage(userID, p.ReceiverID, p.Content)
	if err != nil {
		sendError(conn, MsgTypePrivateSend, "Failed to send message")
		return
	}

	// 返回成功响应给发送者
	sendSuccess(conn, MsgTypePrivateSend, message)

	// 如果接收者在线，实时推送给接收者
	wsManager.mu.RLock()
	if receiverConn, ok := wsManager.clients[p.ReceiverID]; ok {
		sendSuccess(receiverConn, MsgTypePrivateSend, message)
	}
	wsManager.mu.RUnlock()

	// AI判断是否加入对话（异步执行，不阻塞主线程）
	go h.triggerAIForPrivateChat(userID, p.ReceiverID, p.Content)
}

// triggerAIForPrivateChat AI异步处理私聊消息
// senderID: 发送者ID
// receiverID: 接收者ID
// content: 消息内容
func (h *ChatHandler) triggerAIForPrivateChat(senderID, receiverID, content string) {
	if h.aiService == nil || !h.aiService.IsEnabled() {
		return
	}

	ctx := context.Background()

	// 检查是否直接@AI
	mentioned := isAIMentioned(content)

	// 获取聊天上下文
	roomID := h.getPrivateRoomID(senderID, receiverID)
	chatMessages, err := h.getChatContextMessages(roomID, 10)
	if err != nil {
		log.Printf("[ChatHandler] Failed to get chat context for AI: %v", err)
		return
	}

	var decision *service.AIDecision
	if mentioned {
		decision = &service.AIDecision{ShouldRespond: true, Reason: "user mentioned AI"}
	} else {
		decision, err = h.aiService.ShouldRespond(ctx, chatMessages)
		if err != nil {
			log.Printf("[ChatHandler] AI ShouldRespond error: %v", err)
			return
		}
	}

	if !decision.ShouldRespond {
		return
	}

	log.Printf("[ChatHandler] AI decided to respond: %s", decision.Reason)

	// AI生成回复
	aiResponse, err := h.aiService.GenerateResponse(ctx, chatMessages, content)
	if err != nil {
		log.Printf("[ChatHandler] AI GenerateResponse error: %v", err)
		return
	}

	// AI发送消息给两个用户
	aiMessage, err := h.chatService.SendPrivateMessage(service.AIBotUserID, receiverID, aiResponse)
	if err != nil {
		log.Printf("[ChatHandler] AI SendPrivateMessage error: %v", err)
		return
	}

	// 推送AI消息给接收者
	wsManager.mu.RLock()
	if receiverConn, ok := wsManager.clients[receiverID]; ok {
		sendSuccess(receiverConn, MsgTypeAIMessage, aiMessage)
	}
	// 推送AI消息给发送者
	if senderConn, ok := wsManager.clients[senderID]; ok {
		sendSuccess(senderConn, MsgTypeAIMessage, aiMessage)
	}
	wsManager.mu.RUnlock()
}

// handlePrivateHistory 处理获取私聊历史
// userID: 当前用户ID
// payload: 消息数据
// conn: WebSocket连接
func (h *ChatHandler) handlePrivateHistory(userID string, payload json.RawMessage, conn *websocket.Conn) {
	var p PrivateHistoryPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		sendError(conn, MsgTypePrivateHistory, "Invalid payload")
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
		sendError(conn, MsgTypePrivateHistory, "Failed to get history")
		return
	}

	// 返回数据
	sendSuccess(conn, MsgTypePrivateHistory, map[string]interface{}{
		"items": messages,
		"total": total,
	})
}

// handlePrivateUnread 处理获取未读消息
// userID: 当前用户ID
// payload: 消息数据
// conn: WebSocket连接
func (h *ChatHandler) handlePrivateUnread(userID string, payload json.RawMessage, conn *websocket.Conn) {
	var p PrivateUnreadPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		sendError(conn, MsgTypePrivateUnread, "Invalid payload")
		return
	}

	// 调用服务层获取未读消息
	messages, err := h.chatService.GetPrivateUnread(userID, p.TargetID)
	if err != nil {
		sendError(conn, MsgTypePrivateUnread, "Failed to get unread")
		return
	}

	sendSuccess(conn, MsgTypePrivateUnread, map[string]interface{}{
		"items": messages,
	})
}

// handleGroupMessage 处理发送群聊消息
// userID: 当前用户ID
// payload: 消息数据
// conn: WebSocket连接
func (h *ChatHandler) handleGroupMessage(userID string, payload json.RawMessage, conn *websocket.Conn) {
	var p GroupMessagePayload
	if err := json.Unmarshal(payload, &p); err != nil {
		sendError(conn, MsgTypeGroupSend, "Invalid payload")
		return
	}

	// 调用服务层发送群聊消息
	message, err := h.chatService.SendGroupMessage(userID, p.RoomID, p.Content)
	if err != nil {
		sendError(conn, MsgTypeGroupSend, "Failed to send group message")
		return
	}

	// 返回成功响应
	sendSuccess(conn, MsgTypeGroupSend, message)

	// AI异步处理群聊消息
	go h.triggerAIForGroupChat(userID, p.RoomID, p.Content)
}

// triggerAIForGroupChat AI异步处理群聊消息
// senderID: 发送者ID
// roomID: 群聊房间ID
// content: 消息内容
func (h *ChatHandler) triggerAIForGroupChat(senderID, roomID, content string) {
	if h.aiService == nil || !h.aiService.IsEnabled() {
		return
	}

	ctx := context.Background()

	mentioned := isAIMentioned(content)

	chatMessages, err := h.getChatContextMessages(roomID, 10)
	if err != nil {
		log.Printf("[ChatHandler] Failed to get group chat context for AI: %v", err)
		return
	}

	var decision *service.AIDecision
	if mentioned {
		decision = &service.AIDecision{ShouldRespond: true, Reason: "user mentioned AI in group"}
	} else {
		decision, err = h.aiService.ShouldRespond(ctx, chatMessages)
		if err != nil {
			log.Printf("[ChatHandler] AI ShouldRespond error (group): %v", err)
			return
		}
	}

	if !decision.ShouldRespond {
		return
	}

	log.Printf("[ChatHandler] AI decided to respond in group: %s", decision.Reason)

	aiResponse, err := h.aiService.GenerateResponse(ctx, chatMessages, content)
	if err != nil {
		log.Printf("[ChatHandler] AI GenerateResponse error (group): %v", err)
		return
	}

	aiMessage, err := h.chatService.SendGroupMessage(service.AIBotUserID, roomID, aiResponse)
	if err != nil {
		log.Printf("[ChatHandler] AI SendGroupMessage error: %v", err)
		return
	}

	// 推送AI消息给群聊所有在线成员
	wsManager.mu.RLock()
	// 查询群聊成员
	var room model.ChatRoom
	if err := model.DB.Where("id = ?", roomID).First(&room).Error; err == nil {
		for _, memberID := range room.Members {
			if memberConn, ok := wsManager.clients[memberID]; ok {
				sendSuccess(memberConn, MsgTypeAIMessage, aiMessage)
			}
		}
	}
	wsManager.mu.RUnlock()
}

// handleGroupHistory 处理获取群聊历史
// userID: 当前用户ID
// payload: 消息数据
// conn: WebSocket连接
func (h *ChatHandler) handleGroupHistory(userID string, payload json.RawMessage, conn *websocket.Conn) {
	var p GroupHistoryPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		sendError(conn, MsgTypeGroupHistory, "Invalid payload")
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
		sendError(conn, MsgTypeGroupHistory, "Failed to get group history")
		return
	}

	sendSuccess(conn, MsgTypeGroupHistory, map[string]interface{}{
		"items": messages,
		"total": total,
	})
}

// sendSuccess 发送成功响应
// conn: WebSocket连接
// msgType: 消息类型
// data: 响应数据
func sendSuccess(conn *websocket.Conn, msgType int, data interface{}) {
	resp := WSResponse{
		Type:    msgType,
		Success: true,
		Data:    data,
	}
	sendJSON(conn, resp)
}

// sendError 发送错误响应
// conn: WebSocket连接
// msgType: 消息类型
// errMsg: 错误信息
func sendError(conn *websocket.Conn, msgType int, errMsg string) {
	resp := WSResponse{
		Type:    msgType,
		Success: false,
		Error:   errMsg,
	}
	sendJSON(conn, resp)
}

// handleUnreadCount 处理获取未读计数
// userID: 当前用户ID
// payload: 消息数据
// conn: WebSocket连接
func (h *ChatHandler) handleUnreadCount(userID string, payload json.RawMessage, conn *websocket.Conn) {
	var p UnreadCountPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		sendError(conn, MsgTypeUnreadCount, "Invalid payload")
		return
	}

	count, err := h.chatService.GetUnreadCount(userID, p.TargetID)
	if err != nil {
		sendError(conn, MsgTypeUnreadCount, "Failed to get unread count")
		return
	}

	sendSuccess(conn, MsgTypeUnreadCount, map[string]interface{}{
		"target_id": p.TargetID,
		"count":     count,
	})
}

// handleMarkRead 处理标记消息已读
// userID: 当前用户ID
// payload: 消息数据
// conn: WebSocket连接
func (h *ChatHandler) handleMarkRead(userID string, payload json.RawMessage, conn *websocket.Conn) {
	var p MarkReadPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		sendError(conn, MsgTypeMarkRead, "Invalid payload")
		return
	}

	if err := h.chatService.MarkAsRead(userID, p.MessageID); err != nil {
		sendError(conn, MsgTypeMarkRead, "Failed to mark as read")
		return
	}

	sendSuccess(conn, MsgTypeMarkRead, map[string]interface{}{
		"message_id": p.MessageID,
		"success":    true,
	})
}

// handleMarkRoomRead 处理标记房间已读
// userID: 当前用户ID
// payload: 消息数据
// conn: WebSocket连接
func (h *ChatHandler) handleMarkRoomRead(userID string, payload json.RawMessage, conn *websocket.Conn) {
	var p MarkRoomReadPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		sendError(conn, MsgTypeMarkRoomRead, "Invalid payload")
		return
	}

	var roomID string
	if p.RoomType == "private" {
		roomID = h.getPrivateRoomID(userID, p.TargetID)
	} else {
		roomID = p.TargetID
	}

	if err := h.chatService.MarkRoomAsRead(userID, roomID); err != nil {
		sendError(conn, MsgTypeMarkRoomRead, "Failed to mark room as read")
		return
	}

	sendSuccess(conn, MsgTypeMarkRoomRead, map[string]interface{}{
		"room_id": roomID,
		"success": true,
	})
}

// getPrivateRoomID 生成私聊房间ID（和service层保持一致）
// userID1: 用户ID1
// userID2: 用户ID2
func (h *ChatHandler) getPrivateRoomID(userID1, userID2 string) string {
	ids := []string{userID1, userID2}
	if ids[0] > ids[1] {
		ids[0], ids[1] = ids[1], ids[0]
	}
	return ids[0] + "_" + ids[1]
}

// getChatContextMessages 获取聊天上下文消息（用于AI判断）
// roomID: 房间ID
// limit: 限制条数
func (h *ChatHandler) getChatContextMessages(roomID string, limit int) ([]service.ChatContextMessage, error) {
	var messages []model.ChatMessage
	if err := model.DB.Where("room_id = ?", roomID).
		Order("created_at DESC").
		Limit(limit).
		Find(&messages).Error; err != nil {
		return nil, err
	}

	// 收集所有senderID，批量查询用户名
	senderIDs := make(map[string]bool)
	for _, msg := range messages {
		senderIDs[msg.SenderID] = true
	}

	userNames := make(map[string]string)
	if len(senderIDs) > 0 {
		var users []model.User
		ids := make([]string, 0, len(senderIDs))
		for id := range senderIDs {
			ids = append(ids, id)
		}
		model.DB.Where("id IN ?", ids).Find(&users)
		for _, u := range users {
			userNames[u.ID] = u.Username
		}
	}

	// AI助手名称
	userNames[service.AIBotUserID] = "FanAI"

	var result []service.ChatContextMessage
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		name := userNames[msg.SenderID]
		if name == "" {
			name = msg.SenderID
		}
		result = append(result, service.ChatContextMessage{
			SenderID:   msg.SenderID,
			SenderName: name,
			Content:    msg.Content,
		})
	}
	return result, nil
}

// isAIMentioned 检查消息中是否@AI
// content: 消息内容
func isAIMentioned(content string) bool {
	lower := strings.ToLower(content)
	return strings.Contains(lower, "@fanai") ||
		strings.Contains(lower, "@ai") ||
		strings.Contains(lower, "fanai")
}

// sendJSON 将对象序列化为JSON并通过WebSocket发送
// conn: WebSocket连接
// v: 要发送的对象
func sendJSON(conn *websocket.Conn, v interface{}) {
	if data, err := json.Marshal(v); err == nil {
		conn.WriteMessage(websocket.TextMessage, data)
	}
}
