package service

import (
	"errors"
	"fmt"
	"time"

	"video/biz/cache"
	"video/biz/model"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type VideoService struct {
	db *gorm.DB
}

func NewVideoService(db *gorm.DB) *VideoService {
	return &VideoService{db: db}
}

func (s *VideoService) PublishVideo(userID, title, description, videoURL, coverURL string) (*model.Video, error) {
	video := model.Video{
		ID:           uuid.New().String(),
		UserID:       userID,
		VideoURL:     videoURL,
		CoverURL:     coverURL,
		Title:        title,
		Description:  description,
		VisitCount:   0,
		LikeCount:    0,
		CommentCount: 0,
	}

	if err := s.db.Create(&video).Error; err != nil {
		return nil, fmt.Errorf("failed to create video: %w", err)
	}

	return &video, nil
}

func (s *VideoService) GetPublishList(userID string, pageNum, pageSize int) ([]model.Video, int64, error) {
	var videos []model.Video
	var total int64

	offset := (pageNum - 1) * pageSize

	if err := s.db.Model(&model.Video{}).Where("user_id = ?", userID).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := s.db.Where("user_id = ?", userID).
		Order("created_at DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&videos).Error; err != nil {
		return nil, 0, err
	}

	return videos, total, nil
}

func (s *VideoService) SearchVideo(keywords, username string, fromDate, toDate int64, pageNum, pageSize int) ([]model.Video, int64, error) {
	var videos []model.Video
	var total int64

	offset := (pageNum - 1) * pageSize
	query := s.db.Model(&model.Video{})

	if keywords != "" {
		query = query.Where("title LIKE ? OR description LIKE ?", "%"+keywords+"%", "%"+keywords+"%")
	}

	if username != "" {
		var user model.User
		if err := s.db.Where("username = ?", username).First(&user).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return []model.Video{}, 0, nil
			}
			return nil, 0, err
		}
		query = query.Where("user_id = ?", user.ID)
	}

	if fromDate > 0 {
		fromTime := time.Unix(fromDate/1000, (fromDate%1000)*1e6)
		query = query.Where("created_at >= ?", fromTime)
	}

	if toDate > 0 {
		toTime := time.Unix(toDate/1000, (toDate%1000)*1e6)
		query = query.Where("created_at <= ?", toTime)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := query.Order("created_at DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&videos).Error; err != nil {
		return nil, 0, err
	}

	return videos, total, nil
}

func (s *VideoService) GetPopularVideos(pageNum, pageSize int) ([]model.Video, error) {
	videos, err := cache.GetPopularVideosFromCache(pageNum, pageSize)
	if err == nil && videos != nil {
		return videos, nil
	}

	var dbVideos []model.Video
	offset := (pageNum - 1) * pageSize

	if err := s.db.Order("visit_count DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&dbVideos).Error; err != nil {
		return nil, err
	}

	if err := cache.SetPopularVideosCache(dbVideos, pageNum, pageSize); err != nil {
		fmt.Printf("Failed to cache popular videos: %v\n", err)
	}

	return dbVideos, nil
}

func (s *VideoService) IncrementVisitCount(videoID string) error {
	return s.db.Model(&model.Video{}).Where("id = ?", videoID).UpdateColumn("visit_count", gorm.Expr("visit_count + ?", 1)).Error // 更新播放量用UpdateColumn而不是Update
}

func (s *VideoService) GetVideoByID(videoID string) (*model.Video, error) {
	var video model.Video
	if err := s.db.Where("id = ?", videoID).First(&video).Error; err != nil {
		return nil, err
	}
	return &video, nil
}