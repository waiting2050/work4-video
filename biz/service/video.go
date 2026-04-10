package service

import (
	"errors"
	"fmt"
	"log"
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

// PublishVideo 发布视频
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
		log.Printf("[VideoService.PublishVideo] Failed to create video: %v", err)
		return nil, fmt.Errorf("failed to create video: %w", err)
	}

	log.Printf("[VideoService.PublishVideo] Video published successfully: %s", video.ID)
	return &video, nil
}

// GetPublishList 获取用户发布的视频列表
// 参数：
//   - userID: 用户ID
//   - pageNum: 页码，从1开始
//   - pageSize: 每页数量
//
// 返回：
//   - []model.Video: 视频列表
//   - int64: 总数
//   - error: 错误信息
func (s *VideoService) GetPublishList(userID string, pageNum, pageSize int) ([]model.Video, int64, error) {
	var videos []model.Video
	var total int64

	offset := (pageNum - 1) * pageSize

	// 查询总数
	if err := s.db.Model(&model.Video{}).Where("user_id = ?", userID).Count(&total).Error; err != nil {
		log.Printf("[VideoService.GetPublishList] Failed to count videos: %v", err)
		return nil, 0, fmt.Errorf("failed to count videos: %w", err)
	}

	// 查询视频列表
	if err := s.db.Where("user_id = ?", userID).
		Order("created_at DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&videos).Error; err != nil {
		log.Printf("[VideoService.GetPublishList] Failed to get videos: %v", err)
		return nil, 0, fmt.Errorf("failed to get videos: %w", err)
	}

	log.Printf("[VideoService.GetPublishList] Successfully got %d videos for user %s", len(videos), userID)
	return videos, total, nil
}

// SearchVideo 搜索视频
// 参数：
//   - keywords: 关键词，搜索标题和描述
//   - username: 用户名，精确匹配
//   - fromDate: 开始时间戳（毫秒）
//   - toDate: 结束时间戳（毫秒）
//   - pageNum: 页码，从1开始
//   - pageSize: 每页数量
//
// 返回：
//   - []model.Video: 视频列表
//   - int64: 总数
//   - error: 错误信息
func (s *VideoService) SearchVideo(keywords, username string, fromDate, toDate int64, pageNum, pageSize int) ([]model.Video, int64, error) {
	var videos []model.Video
	var total int64

	offset := (pageNum - 1) * pageSize
	// 基础查询
	query := s.db.Model(&model.Video{})

	if keywords != "" {
		query = query.Where("title LIKE ? OR description LIKE ?", "%"+keywords+"%", "%"+keywords+"%")
	}

	if username != "" {
		var user model.User
		if err := s.db.Where("username = ?", username).First(&user).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				log.Printf("[VideoService.SearchVideo] User not found: %s", username)
				return []model.Video{}, 0, nil
			}
			log.Printf("[VideoService.SearchVideo] Failed to find user: %v", err)
			return nil, 0, fmt.Errorf("failed to find user: %w", err)
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
		log.Printf("[VideoService.SearchVideo] Failed to count videos: %v", err)
		return nil, 0, fmt.Errorf("failed to count videos: %w", err)
	}

	if err := query.Order("created_at DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&videos).Error; err != nil {
		log.Printf("[VideoService.SearchVideo] Failed to get videos: %v", err)
		return nil, 0, fmt.Errorf("failed to get videos: %w", err)
	}

	log.Printf("[VideoService.SearchVideo] Successfully found %d videos", len(videos))
	return videos, total, nil
}

// GetPopularVideos 获取热门视频列表
// 参数：
//   - pageNum: 页码，从1开始
//   - pageSize: 每页数量
//
// 返回：
//   - []model.Video: 视频列表
//   - error: 错误信息
func (s *VideoService) GetPopularVideos(pageNum, pageSize int) ([]model.Video, error) {
	// 尝试从缓存获取
	videos, err := cache.GetPopularVideosFromCache(pageNum, pageSize)
	if err != nil {
		if !cache.IsCacheMiss(err) {
			log.Printf("[VideoService.GetPopularVideos] Redis error: %v", err)
		}
	} else if len(videos) > 0 {
		log.Printf("[VideoService.GetPopularVideos] Successfully got %d videos from cache", len(videos))
		return videos, nil
	}

	var dbVideos []model.Video
	offset := (pageNum - 1) * pageSize

	// 从数据库查询
	if err := s.db.Order("visit_count DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&dbVideos).Error; err != nil {
		log.Printf("[VideoService.GetPopularVideos] Failed to get videos: %v", err)
		return nil, fmt.Errorf("failed to get videos: %w", err)
	}

	// 更新缓存
	if err := cache.SetPopularVideosCache(dbVideos, pageNum, pageSize); err != nil {
		log.Printf("[VideoService.GetPopularVideos] Failed to cache popular videos: %v", err)
	}

	log.Printf("[VideoService.GetPopularVideos] Successfully got %d videos from database", len(dbVideos))
	return dbVideos, nil
}

// IncrementVisitCount 增加视频播放量
// 参数：
//   - videoID: 视频ID
//
// 返回：
//   - error: 错误信息
func (s *VideoService) IncrementVisitCount(videoID string) error {
	result := s.db.Model(&model.Video{}).Where("id = ?", videoID).
		UpdateColumn("visit_count", gorm.Expr("visit_count + ?", 1))
	if result.Error != nil {
		log.Printf("[VideoService.IncrementVisitCount] Failed to increment visit count: %v", result.Error)
		return fmt.Errorf("failed to increment visit count: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		log.Printf("[VideoService.IncrementVisitCount] Video not found: %s", videoID)
		return errors.New("video not found")
	}
	return nil
}

// GetVideoByID 根据ID获取视频
// 参数：
//   - videoID: 视频ID
//
// 返回：
//   - *model.Video: 视频对象
//   - error: 错误信息
func (s *VideoService) GetVideoByID(videoID string) (*model.Video, error) {
	var video model.Video
	if err := s.db.Where("id = ?", videoID).First(&video).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Printf("[VideoService.GetVideoByID] Video not found: %s", videoID)
			return nil, errors.New("video not found")
		}
		log.Printf("[VideoService.GetVideoByID] Failed to get video: %v", err)
		return nil, fmt.Errorf("failed to get video: %w", err)
	}
	return &video, nil
}
