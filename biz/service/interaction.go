package service

import (
	"errors"
	"fmt"
	"log"

	"video/biz/cache"
	"video/biz/model"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type InteractionService struct {
	db *gorm.DB
}

// NewInteractionService 创建互动服务实例
// 参数：
//   - db: 数据库连接
// 返回：
//   - *InteractionService: 互动服务实例
func NewInteractionService(db *gorm.DB) *InteractionService {
	return &InteractionService{db: db}
}

// LikeAction 用户点赞/取消点赞操作
// 参数：
//   - userID: 用户ID
//   - videoID: 视频ID
//   - actionType: 操作类型，1=点赞，2=取消点赞
// 返回：
//   - error: 错误信息
func (s *InteractionService) LikeAction(userID, videoID string, actionType int) error {
	// 首先检查视频是否存在
	var video model.Video
	if err := s.db.Where("id = ? AND deleted_at IS NULL", videoID).First(&video).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("video not found")
		}
		return fmt.Errorf("failed to check video: %w", err)
	}

	isLiked, err := cache.IsUserLikedVideo(userID, videoID)
	if err != nil {
		if cache.IsRedisDown(err) {
			log.Printf("[LikeAction] Redis unavailable, fallback to database check: %v", err)
		} else {
			log.Printf("[LikeAction] Redis error when checking like status: %v", err)
		}
	}

	if actionType == 1 {
		if isLiked {
			return errors.New("already liked this video")
		}

		// 使用事务确保原子性操作
		tx := s.db.Begin()
		defer func() {
			if r := recover(); r != nil {
				tx.Rollback()
			}
		}()

		// 在事务内再次检查是否已点赞（防止并发问题）
		var existingLike model.Like
		err := tx.Where("user_id = ? AND video_id = ? AND deleted_at IS NULL", userID, videoID).First(&existingLike).Error
		if err == nil {
			tx.Rollback()
			// 数据库中已存在，异步更新缓存
			go cache.AddUserLikeStatus(userID, videoID)
			return errors.New("already liked this video")
		}

		like := model.Like{
			ID:      uuid.New().String(),
			UserID:  userID,
			VideoID: videoID,
		}

		if err := tx.Create(&like).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to create like: %w", err)
		}

		if err := tx.Model(&model.Video{}).Where("id = ?", videoID).UpdateColumn("like_count", gorm.Expr("like_count + 1")).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to update video like count: %w", err)
		}

		if err := tx.Commit().Error; err != nil {
			return fmt.Errorf("failed to commit transaction: %w", err)
		}

		// 异步更新缓存
		go func() {
			if err := cache.AddUserLikeStatus(userID, videoID); err != nil {
				if cache.IsRedisDown(err) {
					log.Printf("[LikeAction] Redis unavailable when adding like status: %v", err)
				} else {
					log.Printf("[LikeAction] Failed to add user like status to cache: %v", err)
				}
			}
			if _, err := cache.IncrVideoLikeCount(videoID); err != nil {
				log.Printf("[LikeAction] Failed to incr video like count in cache: %v", err)
			}
		}()

		return nil
	} else if actionType == 2 {
		// 使用事务确保原子性操作
		tx := s.db.Begin()
		defer func() {
			if r := recover(); r != nil {
				tx.Rollback()
			}
		}()

		result := tx.Where("user_id = ? AND video_id = ? AND deleted_at IS NULL", userID, videoID).Delete(&model.Like{})
		if result.Error != nil {
			tx.Rollback()
			return fmt.Errorf("failed to delete like: %w", result.Error)
		}

		if result.RowsAffected == 0 {
			tx.Rollback()
			return errors.New("like not found")
		}

		if err := tx.Model(&model.Video{}).Where("id = ? AND like_count > 0", videoID).UpdateColumn("like_count", gorm.Expr("like_count - 1")).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to update video like count: %w", err)
		}

		if err := tx.Commit().Error; err != nil {
			return fmt.Errorf("failed to commit transaction: %w", err)
		}

		// 异步更新缓存
		go func() {
			if err := cache.RemoveUserLikeStatus(userID, videoID); err != nil {
				if cache.IsRedisDown(err) {
					log.Printf("[LikeAction] Redis unavailable when removing like status: %v", err)
				} else {
					log.Printf("[LikeAction] Failed to remove user like status from cache: %v", err)
				}
			}
			if _, err := cache.DecrVideoLikeCount(videoID); err != nil {
				log.Printf("[LikeAction] Failed to decr video like count in cache: %v", err)
			}
		}()

		return nil
	}

	return errors.New("invalid action type")
}

// GetLikeList 获取用户点赞的视频列表
// 参数：
//   - userID: 用户ID
//   - pageNum: 页码，从1开始
//   - pageSize: 每页数量
// 返回：
//   - []model.Video: 视频列表
//   - int64: 总数
//   - error: 错误信息
func (s *InteractionService) GetLikeList(userID string, pageNum, pageSize int) ([]model.Video, int64, error) {
	videoIDs, total, err := cache.GetUserLikeIDsFromZSet(userID, pageNum, pageSize)
	if err != nil {
		if cache.IsRedisDown(err) {
			log.Printf("[GetLikeList] Redis unavailable, fallback to database: %v", err)
		} else if !cache.IsCacheMiss(err) {
			log.Printf("[GetLikeList] Redis error: %v", err)
		}
	}

	if len(videoIDs) > 0 {
		var videos []model.Video
		if err := s.db.Where("id IN ? AND deleted_at IS NULL", videoIDs).Find(&videos).Error; err != nil {
			return nil, 0, fmt.Errorf("failed to get videos: %w", err)
		}
		return videos, total, nil
	}

	var likes []model.Like

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

	if len(likes) == 0 {
		return []model.Video{}, 0, nil
	}

	videoIDs = make([]string, 0, len(likes))
	for _, like := range likes {
		videoIDs = append(videoIDs, like.VideoID)
	}

	var videos []model.Video
	if err := s.db.Where("id IN ? AND deleted_at IS NULL", videoIDs).Find(&videos).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to get videos: %w", err)
	}

	return videos, total, nil
}

// PublishComment 发布评论
// 参数：
//   - userID: 用户ID
//   - videoID: 视频ID
//   - content: 评论内容
// 返回：
//   - *model.Comment: 评论对象
//   - error: 错误信息
func (s *InteractionService) PublishComment(userID, videoID, content string) (*model.Comment, error) {
	var video model.Video
	if err := s.db.Where("id = ? AND deleted_at IS NULL", videoID).First(&video).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("video not found")
		}
		return nil, fmt.Errorf("failed to check video: %w", err)
	}

	comment := model.Comment{
		ID:       uuid.New().String(),
		VideoID:  videoID,
		UserID:   userID,
		Content:  content,
		ParentID: "",
	}

	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Create(&comment).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to create comment: %w", err)
	}

	if err := tx.Model(&model.Video{}).Where("id = ?", videoID).UpdateColumn("comment_count", gorm.Expr("comment_count + 1")).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to update video comment count: %w", err)
	}

	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return &comment, nil
}

// GetCommentList 获取视频评论列表
// 参数：
//   - videoID: 视频ID
//   - pageNum: 页码，从1开始
//   - pageSize: 每页数量
// 返回：
//   - []model.Comment: 评论列表
//   - int64: 总数
//   - error: 错误信息
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

// DeleteComment 删除评论
// 参数：
//   - userID: 用户ID
//   - commentID: 评论ID
// 返回：
//   - error: 错误信息
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

	// 使用事务确保原子性
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Delete(&comment).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to delete comment: %w", err)
	}

	if err := tx.Model(&model.Video{}).Where("id = ? AND comment_count > 0", comment.VideoID).UpdateColumn("comment_count", gorm.Expr("comment_count - 1")).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to update video comment count: %w", err)
	}

	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}