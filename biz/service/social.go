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
	if err := s.db.Where("id = ?", toUserID).First(&targetUser).Error; err != nil {
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
		err := tx.Where("follower_id = ? AND followee_id = ?", userID, toUserID).First(&existingFollow).Error

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
		result := s.db.Where("follower_id = ? AND followee_id = ?", userID, toUserID).Delete(&model.Follow{})
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
	if err := s.db.Model(&model.Follow{}).Where("follower_id = ?", userID).Count(&total).Error; err != nil {
		log.Printf("[SocialService.GetFollowList] Failed to count follows: %v", err)
		return nil, 0, fmt.Errorf("failed to count follows: %w", err)
	}

	// 查询关注列表
	if err := s.db.Where("follower_id = ?", userID).
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
		if err := s.db.Where("id IN ?", followeeIDs).Find(&users).Error; err != nil {
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
	if err := s.db.Model(&model.Follow{}).Where("followee_id = ?", userID).Count(&total).Error; err != nil {
		log.Printf("[SocialService.GetFollowerList] Failed to count followers: %v", err)
		return nil, 0, fmt.Errorf("failed to count followers: %w", err)
	}

	// 查询粉丝列表
	if err := s.db.Where("followee_id = ?", userID).
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
		if err := s.db.Where("id IN ?", followerIDs).Find(&users).Error; err != nil {
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

	offset := (pageNum - 1) * pageSize

	var friendIDs []string
	var total int64

	subQuery := s.db.Model(&model.Follow{}).
		Select("1").
		Where("follower_id = ? AND followee_id = follows.follower_id", userID)

	if err := s.db.Model(&model.Follow{}).
		Select("DISTINCT follower_id"). // 防止软删除记录干扰
		Where("follower_id != ?", userID).
		Where("followee_id = ?", userID).
		Where("EXISTS (?)", subQuery).
		Count(&total).Error; err != nil {
		log.Printf("[SocialService.GetFriendList] Failed to count friends: %v", err)
		return nil, 0, fmt.Errorf("failed to count friends: %w", err)
	}

	if err := s.db.Model(&model.Follow{}).
		Select("DISTINCT follower_id").
		Where("follower_id != ?", userID).
		Where("followee_id = ?", userID).
		Where("EXISTS (?)", subQuery).
		Offset(offset).Limit(pageSize).
		Pluck("follower_id", &friendIDs).Error; err != nil {
		log.Printf("[SocialService.GetFriendList] Failed to get friend IDs: %v", err)
		return nil, 0, fmt.Errorf("failed to get friend IDs: %w", err)
	}

	var users []model.User
	if len(friendIDs) > 0 {
		if err := s.db.Where("id IN ?", friendIDs).Find(&users).Error; err != nil {
			log.Printf("[SocialService.GetFriendList] Failed to get users: %v", err)
			return nil, 0, fmt.Errorf("failed to get users: %w", err)
		}
	}

	log.Printf("[SocialService.GetFriendList] Successfully got %d friends for user %s", len(users), userID)
	return users, total, nil
}
