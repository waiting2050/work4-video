package service

import (
	"errors"
	"fmt"
	"video/biz/model"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type SocialService struct {
	db *gorm.DB
}

func NewSocialService(db *gorm.DB) *SocialService {
	return &SocialService{db: db}
}

func (s *SocialService) FollowAction(userID, toUserID string, actionType int) error {
	if userID == toUserID {
		return errors.New("cannot follow yourself")
	}

	var targetUser model.User
	if err := s.db.Where("id = ? AND deleted_at IS NULL", toUserID).First(&targetUser).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("target user not found")
		}
		return fmt.Errorf("failed to check target user: %w", err)
	}

	if actionType == 0 {
		var existingFollow model.Follow
		err := s.db.Where("follower_id = ? AND followee_id = ? AND deleted_at IS NULL", userID, toUserID).First(&existingFollow).Error

		if err == nil {
			return errors.New("already following this user")
		}

		follow := model.Follow{
			ID: uuid.New().String(),
			FollowerID: userID,
			FolloweeID: toUserID,
		}

		if err := s.db.Create(&follow).Error; err != nil {
			return fmt.Errorf("failed to create follow: %w", err)
		}

		return nil
	} else if actionType == 1 {
		result := s.db.Where("follower_id = ? AND followee_id = ? AND deleted_at IS NULL", userID, toUserID).Delete(&model.Follow{})
		if result.Error != nil {
			return fmt.Errorf("failed to delete follow: %w", result.Error)
		}

		if result.RowsAffected == 0 {
			return errors.New("follow relationship not found")
		}

		return nil
	}

	return errors.New("invalid action type")
}

func (s *SocialService) GetFollowList(userID string, pageNum, pageSize int) ([]model.User, int64, error) {
	var follows []model.Follow
	var total int64

	offset := (pageNum - 1) * pageSize

	if err := s.db.Model(&model.Follow{}).Where("follower_id = ? AND deleted_at IS NULL", userID).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count follows: %w", err)
	}

	if err := s.db.Where("follower_id = ? AND deleted_at IS NULL", userID).
		Order("created_at DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&follows).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to get follows: %w", err)
	}

	var followeeIDs []string
	for _, follow := range follows {
		followeeIDs = append(followeeIDs, follow.FolloweeID)
	}

	var users []model.User
	if len(followeeIDs) > 0 {
		if err := s.db.Where("id IN ? AND deleted_at IS NULL", followeeIDs).Find(&users).Error; err != nil {
			return nil, 0, fmt.Errorf("failed to get users: %w", err)
		}
	}

	return users, total, nil
}

func (s *SocialService) GetFollowerList(userID string, pageNum, pageSize int) ([]model.User, int64, error) {
	var follows []model.Follow
	var total int64

	offset := (pageNum - 1) * pageSize

	if err := s.db.Model(&model.Follow{}).Where("followee_id = ? AND deleted_at IS NULL", userID).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count followers: %w", err)
	}

	if err := s.db.Where("followee_id = ? AND deleted_at IS NULL", userID).
		Order("created_at DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&follows).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to get followers: %w", err)
	}

	var followerIDs []string
	for _, follow := range follows {
		followerIDs = append(followerIDs, follow.FollowerID)
	}

	var users []model.User
	if len(followerIDs) > 0 {
		if err := s.db.Where("id IN ? AND deleted_at IS NULL", followerIDs).Find(&users).Error; err != nil {
			return nil, 0, fmt.Errorf("failed to get users: %w", err)
		}
	}

	return users, total, nil
}

func (s *SocialService) GetFriendList(userID string, pageNum, pageSize int) ([]model.User, int64, error) {
	var following []model.Follow
	if err := s.db.Where("follower_id = ? AND deleted_at IS NULL", userID).Find(&following).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to get following: %w", err)
	}

	followingMap := make(map[string]bool)
	for _, f := range following {
		followingMap[f.FolloweeID] = true
	}

	var followers []model.Follow
	if err := s.db.Where("followee_id = ? AND deleted_at IS NULL", userID).Find(&followers).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to get followers: %w", err)
	}

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
		end := offset + pageSize
		if end > len(friendIDs) {
			end = len(friendIDs)
		}
		if offset < len(friendIDs) {
			if err := s.db.Where("id IN ? AND deleted_at IS NULL", friendIDs[offset:end]).Find(&users).Error; err != nil {
				return nil, 0, fmt.Errorf("failed to get users: %w", err)
			}
		}
	}

	return users, total, nil
}
