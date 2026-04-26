package service

import (
	"log"
	"sort"
	"strings"

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

// SendPrivateMessage 发送私聊消息
// senderID: 发送者ID, receiverID: 接收者ID, content: 消息内容
func (s *ChatService) SendPrivateMessage(senderID, receiverID, content string) (*model.ChatMessage, error) {
	roomID := s.getPrivateRoomID(senderID, receiverID)
	message := &model.ChatMessage{
		ID:        uuid.New().String(),
		SenderID:  senderID,
		RoomType:  "private",
		RoomID:    roomID,
		Content:   content,
		ReadBy:    []string{senderID}, // 发送者已读
	}
	if err := s.db.Create(message).Error; err != nil {
		log.Printf("[ChatService] Failed to create message: %v", err)
		return nil, utils.Wrap(err, utils.CodeInternalError)
	}
	return message, nil
}

// SendGroupMessage 发送群聊消息
// senderID: 发送者ID, roomID: 群聊房间ID, content: 消息内容
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
	}
	if err := s.db.Create(message).Error; err != nil {
		log.Printf("[ChatService] Failed to create group message: %v", err)
		return nil, utils.Wrap(err, utils.CodeInternalError)
	}
	return message, nil
}

// GetPrivateHistory 获取私聊历史记录（分页）
// userID: 当前用户ID, targetID: 对方用户ID, page: 页码, pageSize: 每页数量
func (s *ChatService) GetPrivateHistory(userID, targetID string, page, pageSize int) ([]model.ChatMessage, int64, error) {
	roomID := s.getPrivateRoomID(userID, targetID)
	var messages []model.ChatMessage
	var total int64
	offset := (page - 1) * pageSize
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

// GetPrivateUnread 获取未读私聊消息
// userID: 当前用户ID, targetID: 对方用户ID
func (s *ChatService) GetPrivateUnread(userID, targetID string) ([]model.ChatMessage, error) {
	roomID := s.getPrivateRoomID(userID, targetID)
	var messages []model.ChatMessage
	if err := s.db.Where("room_id = ? AND room_type = ? AND sender_id != ?", roomID, "private", userID).
		Not(map[string]interface{}{"read_by": []string{userID}}). // 排除自己已读的
		Order("created_at ASC").
		Find(&messages).Error; err != nil {
		return nil, utils.Wrap(err, utils.CodeDatabaseError)
	}
	return messages, nil
}

// GetGroupHistory 获取群聊历史记录（分页）
// roomID: 群聊房间ID, page: 页码, pageSize: 每页数量
func (s *ChatService) GetGroupHistory(roomID string, page, pageSize int) ([]model.ChatMessage, int64, error) {
	var messages []model.ChatMessage
	var total int64
	offset := (page - 1) * pageSize
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

// MarkAsRead 标记消息已读
// userID: 当前用户ID, messageID: 消息ID
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
	// 添加到已读列表
	message.ReadBy = append(message.ReadBy, userID)
	return s.db.Save(&message).Error
}

// getPrivateRoomID 生成私聊房间ID
// 将两个用户ID排序后拼接，保证A-B和B-A生成相同的房间ID
func (s *ChatService) getPrivateRoomID(userID1, userID2 string) string {
	ids := []string{userID1, userID2}
	sort.Strings(ids) // 排序保证顺序一致
	return strings.Join(ids, "_")
}
