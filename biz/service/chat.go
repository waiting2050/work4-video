package service

import (
	"log"
	"sort"
	"strings"
	"time"

	"video/biz/cache"
	"video/biz/model"
	"video/biz/utils"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ChatService 聊天服务
type ChatService struct {
	db *gorm.DB
}

// NewChatService 创建聊天服务实例
func NewChatService(db *gorm.DB) *ChatService {
	return &ChatService{db: db}
}

// SendPrivateMessage 发送私聊消息（Redis + MySQL）
func (s *ChatService) SendPrivateMessage(senderID, receiverID, content string) (*model.ChatMessage, error) {
	roomID := s.getPrivateRoomID(senderID, receiverID)
	message := &model.ChatMessage{
		ID:        uuid.New().String(),
		SenderID:  senderID,
		RoomType:  "private",
		RoomID:    roomID,
		Content:   content,
		ReadBy:    []string{senderID}, // 发送者已读
		CreatedAt: time.Now(),
	}

	// 1. 持久化到MySQL
	if err := s.db.Create(message).Error; err != nil {
		log.Printf("[ChatService] Failed to create message: %v", err)
		return nil, utils.Wrap(err, utils.CodeInternalError)
	}

	// 2. 检查接收者是否在线，不在线则存入离线消息
	isOnline := cache.CheckUserOnline(receiverID)
	if !isOnline {
		_ = cache.AddOfflineMessage(receiverID, message)
	}

	// 3. 增加未读计数
	_ = cache.IncrUnreadCount(roomID, receiverID)

	// 4. 缓存最新消息列表
	_ = cache.AddRecentMessage(roomID, message)

	return message, nil
}

// GetOfflineMessages 获取并清除离线消息
func (s *ChatService) GetOfflineMessages(userID string) ([]model.ChatMessage, error) {
	messages, err := cache.GetOfflineMessages(userID)
	if err != nil {
		log.Printf("[ChatService] Failed to get offline messages: %v", err)
		return nil, utils.Wrap(err, utils.CodeInternalError)
	}
	return messages, nil
}

// SendGroupMessage 发送群聊消息
func (s *ChatService) SendGroupMessage(senderID, roomID, content string) (*model.ChatMessage, error) {
	var room model.ChatRoom
	if err := s.db.Where("id = ? AND type = ?", roomID, "group").First(&room).Error; err != nil {
		return nil, utils.New(utils.CodeInternalError)
	}
	message := &model.ChatMessage{
		ID:        uuid.New().String(),
		SenderID:  senderID,
		RoomType:  "group",
		RoomID:    roomID,
		Content:   content,
		ReadBy:    []string{senderID}, // 发送者已读
		CreatedAt: time.Now(),
	}

	// 1. 持久化到MySQL
	if err := s.db.Create(message).Error; err != nil {
		log.Printf("[ChatService] Failed to create group message: %v", err)
		return nil, utils.Wrap(err, utils.CodeInternalError)
	}

	// 2. 缓存群聊最新消息
	_ = cache.AddRecentMessage(roomID, message)

	return message, nil
}

// GetPrivateHistory 获取私聊历史记录（优先从Redis缓存获取，没有则查MySQL）
func (s *ChatService) GetPrivateHistory(userID, targetID string, page, pageSize int) ([]model.ChatMessage, int64, error) {
	roomID := s.getPrivateRoomID(userID, targetID)
	offset := (page - 1) * pageSize

	// 1. 先尝试从Redis获取最近消息
	if page == 1 && pageSize <= 50 {
		cachedMessages, err := cache.GetRecentMessages(roomID, pageSize)
		if err == nil && len(cachedMessages) > 0 {
			// 获取总数
			var total int64
			_ = s.db.Model(&model.ChatMessage{}).
				Where("room_id = ? AND room_type = ?", roomID, "private").
				Count(&total).Error
			return cachedMessages, total, nil
		}
	}

	// 2. Redis没有或者不是首页，查MySQL
	var messages []model.ChatMessage
	var total int64
	if err := s.db.Model(&model.ChatMessage{}).
		Where("room_id = ? AND room_type = ?", roomID, "private").
		Count(&total).Error; err != nil {
		return nil, 0, utils.Wrap(err, utils.CodeDatabaseError)
	}
	if err := s.db.Where("room_id = ? AND room_type = ?", roomID, "private").
		Order("created_at DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&messages).Error; err != nil {
		return nil, 0, utils.Wrap(err, utils.CodeDatabaseError)
	}
	return messages, total, nil
}

// GetPrivateUnread 获取未读私聊消息（优先Redis）
func (s *ChatService) GetPrivateUnread(userID, targetID string) ([]model.ChatMessage, error) {
	roomID := s.getPrivateRoomID(userID, targetID)

	// 1. 先从Redis获取
	messages, err := cache.GetRecentMessages(roomID, 100)
	if err == nil {
		// 过滤出未读的
		var unread []model.ChatMessage
		for _, msg := range messages {
			if msg.SenderID != userID {
				isRead := false
				for _, reader := range msg.ReadBy {
					if reader == userID {
						isRead = true
						break
					}
				}
				if !isRead {
					unread = append(unread, msg)
				}
			}
		}
		if len(unread) > 0 {
			return unread, nil
		}
	}

	// 2. Redis没有，查MySQL
	var dbMessages []model.ChatMessage
	if err := s.db.Where("room_id = ? AND room_type = ? AND sender_id != ?", roomID, "private", userID).
		Not(map[string]interface{}{"read_by": []string{userID}}). // 排除自己已读的
		Order("created_at ASC").
		Find(&dbMessages).Error; err != nil {
		return nil, utils.Wrap(err, utils.CodeDatabaseError)
	}
	return dbMessages, nil
}

// GetGroupHistory 获取群聊历史记录
func (s *ChatService) GetGroupHistory(roomID string, page, pageSize int) ([]model.ChatMessage, int64, error) {
	offset := (page - 1) * pageSize

	// 1. 优先从Redis获取最近消息
	if page == 1 && pageSize <= 50 {
		cachedMessages, err := cache.GetRecentMessages(roomID, pageSize)
		if err == nil && len(cachedMessages) > 0 {
			var total int64
			_ = s.db.Model(&model.ChatMessage{}).
				Where("room_id = ? AND room_type = ?", roomID, "group").
				Count(&total).Error
			return cachedMessages, total, nil
		}
	}

	// 2. Redis没有，查MySQL
	var messages []model.ChatMessage
	var total int64
	if err := s.db.Model(&model.ChatMessage{}).
		Where("room_id = ? AND room_type = ?", roomID, "group").
		Count(&total).Error; err != nil {
		return nil, 0, utils.Wrap(err, utils.CodeDatabaseError)
	}
	if err := s.db.Where("room_id = ? AND room_type = ?", roomID, "group").
		Order("created_at DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&messages).Error; err != nil {
		return nil, 0, utils.Wrap(err, utils.CodeDatabaseError)
	}
	return messages, total, nil
}

// GetUnreadCount 获取未读消息计数
func (s *ChatService) GetUnreadCount(userID, targetID string) (int64, error) {
	roomID := s.getPrivateRoomID(userID, targetID)
	count, err := cache.GetUnreadCount(roomID, userID)
	if err == nil {
		return count, nil
	}
	// Redis失败，从MySQL计算
	var c int64
	_ = s.db.Model(&model.ChatMessage{}).
		Where("room_id = ? AND room_type = ? AND sender_id != ?", roomID, "private", userID).
		Not(map[string]interface{}{"read_by": []string{userID}}).
		Count(&c)
	return c, nil
}

// MarkAsRead 标记消息已读（MySQL + Redis）
func (s *ChatService) MarkAsRead(userID, messageID string) error {
	var message model.ChatMessage
	if err := s.db.Where("id = ?", messageID).First(&message).Error; err != nil {
		return utils.New(utils.CodeVideoNotFound)
	}

	// 检查是否已读
	for _, id := range message.ReadBy {
		if id == userID {
			return nil // 已经读过了
		}
	}

	// 标记已读
	message.ReadBy = append(message.ReadBy, userID)
	if err := s.db.Save(&message).Error; err != nil {
		return err
	}

	// 减少Redis未读计数
	_ = cache.DecrUnreadCount(message.RoomID, userID)

	return nil
}

// MarkRoomAsRead 标记整个房间已读
func (s *ChatService) MarkRoomAsRead(userID, roomID string) error {
	// MySQL批量标记
	if err := s.db.Model(&model.ChatMessage{}).
		Where("room_id = ? AND sender_id != ?", roomID, userID).
		Update("read_by", gorm.Expr("JSON_ARRAY_APPEND(read_by, '$', ?)", userID)).Error; err != nil {
		return err
	}
	// Redis清零
	_ = cache.ClearUnreadCount(roomID, userID)
	return nil
}

// getPrivateRoomID 生成私聊房间ID
func (s *ChatService) getPrivateRoomID(userID1, userID2 string) string {
	ids := []string{userID1, userID2}
	sort.Strings(ids) // 排序保证顺序一致
	return strings.Join(ids, "_")
}
