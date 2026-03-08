package service

import (
	"errors"
	"fmt"

	"video/biz/model"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type InteractionService struct {
	db *gorm.DB
}

func NewInteractionService(db *gorm.DB) *InteractionService {
	return &InteractionService{db: db}
}

func (s *InteractionService) LikeAction(userID, videoID string, actionType int) error {
	if actionType == 1 {
		var existingLike model.Like
		err := s.db.Where("user_id = ? AND video_id = ? AND deleted_at IS NULL", userID, videoID).First(&existingLike).Error
		if err == nil {
			return errors.New("already liked this video")
		}

		like := model.Like{
			ID:      uuid.New().String(),
			UserID:  userID,
			VideoID: videoID,
		}

		if err := s.db.Create(&like).Error; err != nil {
			return fmt.Errorf("failed to create like: %w", err)
		}

		if err := s.db.Model(&model.Video{}).Where("id = ?", videoID).UpdateColumn("like_count", gorm.Expr("like_count + 1")).Error; err != nil {
			return fmt.Errorf("failed to update video like count: %w", err)
		}

		return nil
	} else if actionType == 2 {
		result := s.db.Where("user_id = ? AND video_id = ? AND deleted_at IS NULL", userID, videoID).Delete(&model.Like{})
		if result.Error != nil {
			return fmt.Errorf("failed to delete like: %w", result.Error)
		}

		if result.RowsAffected == 0 {
			return errors.New("like not found")
		}

		if err := s.db.Model(&model.Video{}).Where("id = ? AND like_count > 0", videoID).UpdateColumn("like_count", gorm.Expr("like_count - 1")).Error; err != nil {
			return fmt.Errorf("failed to update video like count: %w", err)
		}

		return nil
	}

	return errors.New("invalid action type")
}

func (s *InteractionService) GetLikeList(userID string, pageNum, pageSize int) ([]model.Video, int64, error) {
	var likes []model.Like
	var total int64

	offset := (pageNum - 1) * pageSize

	if err := s.db.Model(&model.Like{}).Where("user_id = ? AND deleted_at IS NULL", userID).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count likes: %w", err)
	}

	if err := s.db.Where("user_id = ? AND deleted_at IS NULL", userID).
		Order("created_at DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&likes).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to get likes: %w", err)
	}

	var videoIDs []string
	for _, like := range likes {
		videoIDs = append(videoIDs, like.VideoID)
	}

	var videos []model.Video
	if len(videoIDs) > 0 {
		if err := s.db.Where("id IN ? AND deleted_at IS NULL", videoIDs).Find(&videos).Error; err != nil {
			return nil, 0, fmt.Errorf("failed to get videos: %w", err)
		}
	}

	return videos, total, nil
}

func (s *InteractionService) PublishComment(userID, videoID, content string) (*model.Comment, error) {
	var video model.Video
	if err := s.db.Where("id = ? AND deleted_at IS NULL", videoID).First(&video).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("video not found")
		}
		return nil, fmt.Errorf("failed to check video: %w", err)
	}

	comment := model.Comment{
		ID: uuid.New().String(),
		VideoID: videoID,
		UserID: userID,
		Content: content,
		ParentID: "",
	}

	if err := s.db.Create(&comment).Error; err != nil {
		return nil, fmt.Errorf("failed to create comment: %w", err)
	}

	if err := s.db.Model(&model.Video{}).Where("id = ?", videoID).UpdateColumn("comment_count", gorm.Expr("comment_count + 1")).Error; err != nil {
		return nil, fmt.Errorf("failed to update video comment count: %w", err)
	}
	
	return &comment, nil
}

func (s *InteractionService) GetCommentList(videoID string, pageNum, pageSize int) ([]model.Comment, int64, error) {
	var comments []model.Comment
	var total int64

	offset := (pageNum - 1) * pageSize

	if err := s.db.Model(&model.Comment{}).Where("video_id = ? AND deleted_at IS NULL", videoID).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count comments: %w", err)
	}

	if err := s.db.Where("video_id = ? AND deleted_at IS NULL", videoID).
	Order("created_at DESC").
	Offset(offset).
	Limit(pageSize).
	Find(&comments).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to get comments: %w", err)
	}

	return comments, total, nil
}

func (s *InteractionService) DeleteComment(userID, commentID string) error {
	var comment model.Comment
	if err := s.db.Where("id = ? AND deleted_at IS NULL", commentID).First(&comment).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("comment not found")
		}
		return fmt.Errorf("failed to find comment: %w", err)
	}

	if comment.UserID != userID {
		return errors.New("cannot delete other users' comments")
	}

	if err := s.db.Delete(&comment).Error; err != nil {
		return fmt.Errorf("failed to delete comment: %w", err)
	}

	if err := s.db.Model(&model.Video{}).Where("id = ? AND comment_count > 0", comment.VideoID).UpdateColumn("comment_count", gorm.Expr("comment_count - 1")).Error; err != nil {
		return fmt.Errorf("failed to update video comment count: %w", err)
	}

	return nil
}