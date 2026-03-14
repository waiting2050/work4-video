package service

import (
	"errors"
	"fmt"
	"log"

	"video/biz/model"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type SocialService struct {
	db *gorm.DB
}

// NewSocialService 创建社交服务实例
// 参数：
//   - db: 数据库连接
// 返回：
//   - *SocialService: 社交服务实例
func NewSocialService(db *gorm.DB) *SocialService {
	return &SocialService{db: db}
}

// FollowAction 关注/取消关注操作
// 参数：
//   - userID: 当前用户ID
//   - toUserID: 目标用户ID
//   - actionType: 操作类型，0=关注，1=取消关注
// 返回：
//   - error: 错误信息
func (s *SocialService) FollowAction(userID, toUserID string, actionType int) error {
	// 不能关注自己
	if userID == toUserID {
		log.Printf("[SocialService.FollowAction] Cannot follow yourself: %s", userID)
		return errors.New("cannot follow yourself")
	}

	// 检查目标用户是否存在
	var targetUser model.User
	if err := s.db.Where("id = ? AND deleted_at IS NULL", toUserID).First(&targetUser).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Printf("[SocialService.FollowAction] Target user not found: %s", toUserID)
			return errors.New("target user not found")
		}
		log.Printf("[SocialService.FollowAction] Failed to check target user: %v", err)
		return fmt.Errorf("failed to check target user: %w", err)
	}

	if actionType == 0 {
		// 关注操作 - 使用事务避免竞态条件
		tx := s.db.Begin()
		defer func() {
			if r := recover(); r != nil {
				tx.Rollback()
			}
		}()

		var existingFollow model.Follow
		err := tx.Where("follower_id = ? AND followee_id = ? AND deleted_at IS NULL", userID, toUserID).First(&existingFollow).Error

		if err == nil {
			tx.Rollback()
			log.Printf("[SocialService.FollowAction] Already following user: %s -> %s", userID, toUserID)
			return errors.New("already following this user")
		}

		follow := model.Follow{
			ID:         uuid.New().String(),
			FollowerID: userID,
			FolloweeID: toUserID,
		}

		if err := tx.Create(&follow).Error; err != nil {
			tx.Rollback()
			log.Printf("[SocialService.FollowAction] Failed to create follow: %v", err)
			return fmt.Errorf("failed to create follow: %w", err)
		}

		if err := tx.Commit().Error; err != nil {
			log.Printf("[SocialService.FollowAction] Failed to commit transaction: %v", err)
			return fmt.Errorf("failed to commit transaction: %w", err)
		}

		log.Printf("[SocialService.FollowAction] Successfully followed user: %s -> %s", userID, toUserID)
		return nil
	} else if actionType == 1 {
		// 取消关注操作
		result := s.db.Where("follower_id = ? AND followee_id = ? AND deleted_at IS NULL", userID, toUserID).Delete(&model.Follow{})
		if result.Error != nil {
			log.Printf("[SocialService.FollowAction] Failed to delete follow: %v", result.Error)
			return fmt.Errorf("failed to delete follow: %w", result.Error)
		}

		if result.RowsAffected == 0 {
			log.Printf("[SocialService.FollowAction] Follow relationship not found: %s -> %s", userID, toUserID)
			return errors.New("follow relationship not found")
		}

		log.Printf("[SocialService.FollowAction] Successfully unfollowed user: %s -> %s", userID, toUserID)
		return nil
	}

	log.Printf("[SocialService.FollowAction] Invalid action type: %d", actionType)
	return errors.New("invalid action type")
}

// GetFollowList 获取关注列表
// 参数：
//   - userID: 用户ID
//   - pageNum: 页码，从1开始
//   - pageSize: 每页数量
// 返回：
//   - []model.User: 用户列表
//   - int64: 总数
//   - error: 错误信息
func (s *SocialService) GetFollowList(userID string, pageNum, pageSize int) ([]model.User, int64, error) {
	var follows []model.Follow
	var total int64

	offset := (pageNum - 1) * pageSize

	// 查询关注总数
	if err := s.db.Model(&model.Follow{}).Where("follower_id = ? AND deleted_at IS NULL", userID).Count(&total).Error; err != nil {
		log.Printf("[SocialService.GetFollowList] Failed to count follows: %v", err)
		return nil, 0, fmt.Errorf("failed to count follows: %w", err)
	}

	// 查询关注列表
	if err := s.db.Where("follower_id = ? AND deleted_at IS NULL", userID).
		Order("created_at DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&follows).Error; err != nil {
		log.Printf("[SocialService.GetFollowList] Failed to get follows: %v", err)
		return nil, 0, fmt.Errorf("failed to get follows: %w", err)
	}

	// 获取关注用户ID列表
	var followeeIDs []string
	for _, follow := range follows {
		followeeIDs = append(followeeIDs, follow.FolloweeID)
	}

	// 查询用户信息
	var users []model.User
	if len(followeeIDs) > 0 {
		if err := s.db.Where("id IN ? AND deleted_at IS NULL", followeeIDs).Find(&users).Error; err != nil {
			log.Printf("[SocialService.GetFollowList] Failed to get users: %v", err)
			return nil, 0, fmt.Errorf("failed to get users: %w", err)
		}
	}

	log.Printf("[SocialService.GetFollowList] Successfully got %d follows for user %s", len(users), userID)
	return users, total, nil
}

// GetFollowerList 获取粉丝列表
// 参数：
//   - userID: 用户ID
//   - pageNum: 页码，从1开始
//   - pageSize: 每页数量
// 返回：
//   - []model.User: 用户列表
//   - int64: 总数
//   - error: 错误信息
func (s *SocialService) GetFollowerList(userID string, pageNum, pageSize int) ([]model.User, int64, error) {
	var follows []model.Follow
	var total int64

	offset := (pageNum - 1) * pageSize

	// 查询粉丝总数
	if err := s.db.Model(&model.Follow{}).Where("followee_id = ? AND deleted_at IS NULL", userID).Count(&total).Error; err != nil {
		log.Printf("[SocialService.GetFollowerList] Failed to count followers: %v", err)
		return nil, 0, fmt.Errorf("failed to count followers: %w", err)
	}

	// 查询粉丝列表
	if err := s.db.Where("followee_id = ? AND deleted_at IS NULL", userID).
		Order("created_at DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&follows).Error; err != nil {
		log.Printf("[SocialService.GetFollowerList] Failed to get followers: %v", err)
		return nil, 0, fmt.Errorf("failed to get followers: %w", err)
	}

	// 获取粉丝用户ID列表
	var followerIDs []string
	for _, follow := range follows {
		followerIDs = append(followerIDs, follow.FollowerID)
	}

	// 查询用户信息
	var users []model.User
	if len(followerIDs) > 0 {
		if err := s.db.Where("id IN ? AND deleted_at IS NULL", followerIDs).Find(&users).Error; err != nil {
			log.Printf("[SocialService.GetFollowerList] Failed to get users: %v", err)
			return nil, 0, fmt.Errorf("failed to get users: %w", err)
		}
	}

	log.Printf("[SocialService.GetFollowerList] Successfully got %d followers for user %s", len(users), userID)
	return users, total, nil
}

// GetFriendList 获取好友列表（互相关注）
// 参数：
//   - userID: 用户ID
//   - pageNum: 页码，从1开始
//   - pageSize: 每页数量
// 返回：
//   - []model.User: 用户列表
//   - int64: 总数
//   - error: 错误信息
func (s *SocialService) GetFriendList(userID string, pageNum, pageSize int) ([]model.User, int64, error) {
	// 参数验证
	if pageNum < 1 {
		pageNum = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100 // 限制最大页大小
	}

	// 查询用户关注列表
	var following []model.Follow
	if err := s.db.Where("follower_id = ? AND deleted_at IS NULL", userID).Find(&following).Error; err != nil {
		log.Printf("[SocialService.GetFriendList] Failed to get following: %v", err)
		return nil, 0, fmt.Errorf("failed to get following: %w", err)
	}

	// 构建关注用户ID映射
	followingMap := make(map[string]bool)
	for _, f := range following {
		followingMap[f.FolloweeID] = true
	}

	// 查询用户粉丝列表
	var followers []model.Follow
	if err := s.db.Where("followee_id = ? AND deleted_at IS NULL", userID).Find(&followers).Error; err != nil {
		log.Printf("[SocialService.GetFriendList] Failed to get followers: %v", err)
		return nil, 0, fmt.Errorf("failed to get followers: %w", err)
	}

	// 筛选互相关注的用户ID
	var friendIDs []string
	for _, f := range followers {
		if followingMap[f.FollowerID] {
			friendIDs = append(friendIDs, f.FollowerID)
		}
	}

	total := int64(len(friendIDs))

	// 别的函数先查数据库，利用数据库分页，但是数据库不存在直接的好友记录，所以
	// 没法用同样的方法，选择在切片里实现分页
	var users []model.User
	if len(friendIDs) > 0 {
		offset := (pageNum - 1) * pageSize
		if offset < 0 {
			offset = 0
		}
		if offset < len(friendIDs) {
			end := offset + pageSize
			if end > len(friendIDs) {
				end = len(friendIDs)
			}
			if err := s.db.Where("id IN ? AND deleted_at IS NULL", friendIDs[offset:end]).Find(&users).Error; err != nil {
				log.Printf("[SocialService.GetFriendList] Failed to get users: %v", err)
				return nil, 0, fmt.Errorf("failed to get users: %w", err)
			}
		}
	}

	log.Printf("[SocialService.GetFriendList] Successfully got %d friends for user %s", len(users), userID)
	return users, total, nil
}
