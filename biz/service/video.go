package service

import (
	"encoding/json"
	"log"
	"time"

	"video/biz/cache"
	"video/biz/model"
	"video/biz/utils"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type VideoService struct {
	db *gorm.DB
}

func NewVideoService(db *gorm.DB) *VideoService {
	return &VideoService{db: db}
}

type VideoWithUser struct {
	model.Video
	Username  string `json:"username"`
	AvatarURL string `json:"user_avatar"`
}

type CachedVideo struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	VideoURL     string    `json:"video_url"`
	CoverURL     string    `json:"cover_url"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	VisitCount   int       `json:"visit_count"`
	LikeCount    int       `json:"like_count"`
	CommentCount int       `json:"comment_count"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	Username     string    `json:"username"`
	AvatarURL    string    `json:"user_avatar"`
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
		log.Printf("[VideoService.PublishVideo] Failed to create video: %v", err)
		return nil, utils.Wrap(err, utils.CodeInternalError)
	}

	log.Printf("[VideoService.PublishVideo] Video published successfully: %s", video.ID)
	return &video, nil
}

func (s *VideoService) GetPublishList(userID string, pageNum, pageSize int) ([]VideoWithUser, int64, error) {
	var videos []VideoWithUser
	var total int64

	offset := (pageNum - 1) * pageSize

	if err := s.db.Model(&model.Video{}).Where("videos.user_id = ?", userID).Count(&total).Error; err != nil {
		log.Printf("[VideoService.GetPublishList] Failed to count videos: %v", err)
		return nil, 0, utils.Wrap(err, utils.CodeDatabaseError)
	}

	if err := s.db.Table("videos").
		Select("videos.*, users.username, users.avatar_url as user_avatar").
		Joins("LEFT JOIN users ON videos.user_id = users.id").
		Where("videos.user_id = ?", userID).
		Order("videos.created_at DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&videos).Error; err != nil {
		log.Printf("[VideoService.GetPublishList] Failed to get videos: %v", err)
		return nil, 0, utils.Wrap(err, utils.CodeDatabaseError)
	}

	log.Printf("[VideoService.GetPublishList] Successfully got %d videos for user %s", len(videos), userID)
	return videos, total, nil
}

func (s *VideoService) SearchVideo(keywords, username string, fromDate, toDate int64, pageNum, pageSize int) ([]VideoWithUser, int64, error) {
	var videos []VideoWithUser
	var total int64

	offset := (pageNum - 1) * pageSize
	query := s.db.Table("videos").
		Select("videos.*, users.username, users.avatar_url as user_avatar").
		Joins("LEFT JOIN users ON videos.user_id = users.id")

	if keywords != "" {
		query = query.Where("videos.title LIKE ? OR videos.description LIKE ?", "%"+keywords+"%", "%"+keywords+"%")
	}

	if username != "" {
		var user model.User
		if err := s.db.Where("username = ?", username).First(&user).Error; err != nil {
			log.Printf("[VideoService.SearchVideo] User not found: %s", username)
			return []VideoWithUser{}, 0, nil
		}
		query = query.Where("videos.user_id = ?", user.ID)
	}

	if fromDate > 0 {
		fromTime := time.Unix(fromDate/1000, (fromDate%1000)*1e6)
		query = query.Where("videos.created_at >= ?", fromTime)
	}

	if toDate > 0 {
		toTime := time.Unix(toDate/1000, (toDate%1000)*1e6)
		query = query.Where("videos.created_at <= ?", toTime)
	}

	if err := query.Count(&total).Error; err != nil {
		log.Printf("[VideoService.SearchVideo] Failed to count videos: %v", err)
		return nil, 0, utils.Wrap(err, utils.CodeDatabaseError)
	}

	if err := query.Order("videos.created_at DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&videos).Error; err != nil {
		log.Printf("[VideoService.SearchVideo] Failed to get videos: %v", err)
		return nil, 0, utils.Wrap(err, utils.CodeDatabaseError)
	}

	log.Printf("[VideoService.SearchVideo] Successfully found %d videos", len(videos))
	return videos, total, nil
}

func (s *VideoService) GetPopularVideos(pageNum, pageSize int) ([]VideoWithUser, error) {
	cachedData, err := cache.GetPopularVideosFromCache(pageNum, pageSize)
	if err == nil && cachedData != nil {
		var cachedVideos []CachedVideo
		if data, ok := cachedData.([]interface{}); ok {
			jsonData, _ := json.Marshal(data)
			json.Unmarshal(jsonData, &cachedVideos)
		}
		if len(cachedVideos) > 0 {
			videos := make([]VideoWithUser, len(cachedVideos))
			for i, cv := range cachedVideos {
				videos[i] = VideoWithUser{
					Video: model.Video{
						ID:           cv.ID,
						UserID:       cv.UserID,
						VideoURL:     cv.VideoURL,
						CoverURL:     cv.CoverURL,
						Title:        cv.Title,
						Description:  cv.Description,
						VisitCount:   cv.VisitCount,
						LikeCount:    cv.LikeCount,
						CommentCount: cv.CommentCount,
						CreatedAt:    cv.CreatedAt,
						UpdatedAt:    cv.UpdatedAt,
					},
					Username:  cv.Username,
					AvatarURL: cv.AvatarURL,
				}
			}
			log.Printf("[VideoService.GetPopularVideos] Successfully got %d videos from cache", len(videos))
			return videos, nil
		}
	} else if err != nil && !cache.IsCacheMiss(err) {
		log.Printf("[VideoService.GetPopularVideos] Redis error: %v", err)
	}

	var dbVideos []VideoWithUser
	offset := (pageNum - 1) * pageSize

	if err := s.db.Table("videos").
		Select("videos.*, users.username, users.avatar_url as user_avatar").
		Joins("LEFT JOIN users ON videos.user_id = users.id").
		Order("videos.visit_count DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&dbVideos).Error; err != nil {
		log.Printf("[VideoService.GetPopularVideos] Failed to get videos: %v", err)
		return nil, utils.Wrap(err, utils.CodeDatabaseError)
	}

	cachedVideos := make([]CachedVideo, len(dbVideos))
	for i, v := range dbVideos {
		cachedVideos[i] = CachedVideo{
			ID:           v.ID,
			UserID:       v.UserID,
			VideoURL:     v.VideoURL,
			CoverURL:     v.CoverURL,
			Title:        v.Title,
			Description:  v.Description,
			VisitCount:   v.VisitCount,
			LikeCount:    v.LikeCount,
			CommentCount: v.CommentCount,
			CreatedAt:    v.CreatedAt,
			UpdatedAt:    v.UpdatedAt,
			Username:     v.Username,
			AvatarURL:    v.AvatarURL,
		}
	}
	cache.SetPopularVideosCache(cachedVideos, pageNum, pageSize)

	log.Printf("[VideoService.GetPopularVideos] Successfully got %d videos from database", len(dbVideos))
	return dbVideos, nil
}

func (s *VideoService) IncrementVisitCount(videoID string) error {
	result := s.db.Model(&model.Video{}).Where("id = ?", videoID).
		UpdateColumn("visit_count", gorm.Expr("visit_count + ?", 1))
	if result.Error != nil {
		log.Printf("[VideoService.IncrementVisitCount] Failed to increment visit count: %v", result.Error)
		return utils.Wrap(result.Error, utils.CodeInternalError)
	}
	if result.RowsAffected == 0 {
		log.Printf("[VideoService.IncrementVisitCount] Video not found: %s", videoID)
		return utils.New(utils.CodeVideoNotFound)
	}
	return nil
}

func (s *VideoService) GetVideoByID(videoID string) (*model.Video, error) {
	var video model.Video
	if err := s.db.Where("id = ?", videoID).First(&video).Error; err != nil {
		log.Printf("[VideoService.GetVideoByID] Video not found: %s", videoID)
		return nil, utils.New(utils.CodeVideoNotFound)
	}
	return &video, nil
}

func (s *VideoService) GetVideoFeed(latestTime int64) ([]VideoWithUser, error) {
	var videos []VideoWithUser
	query := s.db.Table("videos").
		Select("videos.*, users.username, users.avatar_url as user_avatar").
		Joins("LEFT JOIN users ON videos.user_id = users.id")

	if latestTime > 0 {
		feedTime := time.Unix(latestTime/1000, (latestTime%1000)*1e6)
		query = query.Where("videos.created_at >= ?", feedTime)
	}

	if err := query.Order("videos.created_at DESC").
		Limit(30).
		Find(&videos).Error; err != nil {
		log.Printf("[VideoService.GetVideoFeed] Failed to get videos: %v", err)
		return nil, utils.Wrap(err, utils.CodeDatabaseError)
	}

	log.Printf("[VideoService.GetVideoFeed] Successfully got %d videos", len(videos))
	return videos, nil
}
