package service

import (
	"log"

	"video/biz/model"
	"video/biz/utils"

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
		log.Printf("[SocialService.FollowAction] Cannot follow yourself: %s", userID)
		return utils.New(utils.CodeInvalidAction)
	}

	var targetUser model.User
	if err := s.db.Where("id = ?", toUserID).First(&targetUser).Error; err != nil {
		log.Printf("[SocialService.FollowAction] Target user not found: %s", toUserID)
		return utils.New(utils.CodeUserNotFound)
	}

	if actionType == 0 {
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
			return utils.New(utils.CodeAlreadyFollowed)
		}

		follow := model.Follow{
			ID:         uuid.New().String(),
			FollowerID: userID,
			FolloweeID: toUserID,
		}

		if err := tx.Create(&follow).Error; err != nil {
			tx.Rollback()
			log.Printf("[SocialService.FollowAction] Failed to create follow: %v", err)
			return utils.Wrap(err, utils.CodeInternalError)
		}

		if err := tx.Commit().Error; err != nil {
			log.Printf("[SocialService.FollowAction] Failed to commit transaction: %v", err)
			return utils.Wrap(err, utils.CodeInternalError)
		}

		log.Printf("[SocialService.FollowAction] Successfully followed user: %s -> %s", userID, toUserID)
		return nil
	} else if actionType == 1 {
		tx := s.db.Begin()
		defer func() {
			if r := recover(); r != nil {
				tx.Rollback()
			}
		}()

		result := tx.Where("follower_id = ? AND followee_id = ?", userID, toUserID).Delete(&model.Follow{})
		if result.Error != nil {
			tx.Rollback()
			log.Printf("[SocialService.FollowAction] Failed to delete follow: %v", result.Error)
			return utils.Wrap(result.Error, utils.CodeInternalError)
		}

		if result.RowsAffected == 0 {
			tx.Rollback()
			log.Printf("[SocialService.FollowAction] Follow relationship not found: %s -> %s", userID, toUserID)
			return utils.New(utils.CodeNotFollowed)
		}

		if err := tx.Commit().Error; err != nil {
			tx.Rollback()
			log.Printf("[SocialService.FollowAction] Failed to commit transaction: %v", err)
			return utils.Wrap(err, utils.CodeInternalError)
		}

		log.Printf("[SocialService.FollowAction] Successfully unfollowed user: %s -> %s", userID, toUserID)
		return nil
	}

	log.Printf("[SocialService.FollowAction] Invalid action type: %d", actionType)
	return utils.New(utils.CodeInvalidAction)
}

func (s *SocialService) GetFollowList(userID string, pageNum, pageSize int) ([]model.User, int64, error) {
	var follows []model.Follow
	var total int64

	offset := (pageNum - 1) * pageSize

	if err := s.db.Model(&model.Follow{}).Where("follower_id = ?", userID).Count(&total).Error; err != nil {
		log.Printf("[SocialService.GetFollowList] Failed to count follows: %v", err)
		return nil, 0, utils.Wrap(err, utils.CodeDatabaseError)
	}

	if err := s.db.Where("follower_id = ?", userID).
		Order("created_at DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&follows).Error; err != nil {
		log.Printf("[SocialService.GetFollowList] Failed to get follows: %v", err)
		return nil, 0, utils.Wrap(err, utils.CodeDatabaseError)
	}

	var followeeIDs []string
	for _, follow := range follows {
		followeeIDs = append(followeeIDs, follow.FolloweeID)
	}

	var users []model.User
	if len(followeeIDs) > 0 {
		if err := s.db.Where("id IN ?", followeeIDs).Find(&users).Error; err != nil {
			log.Printf("[SocialService.GetFollowList] Failed to get users: %v", err)
			return nil, 0, utils.Wrap(err, utils.CodeDatabaseError)
		}
	}

	log.Printf("[SocialService.GetFollowList] Successfully got %d follows for user %s", len(users), userID)
	return users, total, nil
}

func (s *SocialService) GetFollowerList(userID string, pageNum, pageSize int) ([]model.User, int64, error) {
	var follows []model.Follow
	var total int64

	offset := (pageNum - 1) * pageSize

	if err := s.db.Model(&model.Follow{}).Where("followee_id = ?", userID).Count(&total).Error; err != nil {
		log.Printf("[SocialService.GetFollowerList] Failed to count followers: %v", err)
		return nil, 0, utils.Wrap(err, utils.CodeDatabaseError)
	}

	if err := s.db.Where("followee_id = ?", userID).
		Order("created_at DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&follows).Error; err != nil {
		log.Printf("[SocialService.GetFollowerList] Failed to get follows: %v", err)
		return nil, 0, utils.Wrap(err, utils.CodeDatabaseError)
	}

	var followerIDs []string
	for _, follow := range follows {
		followerIDs = append(followerIDs, follow.FollowerID)
	}

	var users []model.User
	if len(followerIDs) > 0 {
		if err := s.db.Where("id IN ?", followerIDs).Find(&users).Error; err != nil {
			log.Printf("[SocialService.GetFollowerList] Failed to get users: %v", err)
			return nil, 0, utils.Wrap(err, utils.CodeDatabaseError)
		}
	}

	log.Printf("[SocialService.GetFollowerList] Successfully got %d followers for user %s", len(users), userID)
	return users, total, nil
}

func (s *SocialService) GetFriendList(userID string, pageNum, pageSize int) ([]model.User, int64, error) {
	if pageNum < 1 {
		pageNum = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}

	offset := (pageNum - 1) * pageSize

	var friendIDs []string
	var total int64

	mutualSubQuery := s.db.Table("follows f2").
		Select("1").
		Where("f2.follower_id = ? AND f2.followee_id = f1.follower_id", userID)

	if err := s.db.Table("follows f1").
		Select("DISTINCT f1.follower_id").
		Where("f1.followee_id = ?", userID).
		Where("EXISTS (?)", mutualSubQuery).
		Count(&total).Error; err != nil {
		log.Printf("[SocialService.GetFriendList] Failed to count friends: %v", err)
		return nil, 0, utils.Wrap(err, utils.CodeDatabaseError)
	}

	if err := s.db.Table("follows f1").
		Select("DISTINCT f1.follower_id").
		Where("f1.followee_id = ?", userID).
		Where("EXISTS (?)", mutualSubQuery).
		Offset(offset).Limit(pageSize).
		Pluck("f1.follower_id", &friendIDs).Error; err != nil {
		log.Printf("[SocialService.GetFriendList] Failed to get friend IDs: %v", err)
		return nil, 0, utils.Wrap(err, utils.CodeDatabaseError)
	}

	var users []model.User
	if len(friendIDs) > 0 {
		if err := s.db.Where("id IN ?", friendIDs).Find(&users).Error; err != nil {
			log.Printf("[SocialService.GetFriendList] Failed to get users: %v", err)
			return nil, 0, utils.Wrap(err, utils.CodeDatabaseError)
		}
	}

	log.Printf("[SocialService.GetFriendList] Successfully got %d friends for user %s", len(users), userID)
	return users, total, nil
}
