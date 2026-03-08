package handler

import (
	"context"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"video/biz/service"
	"video/biz/utils"

	"github.com/cloudwego/hertz/pkg/app"
)

type VideoHandler struct {
	videoService *service.VideoService
}

func NewVideoHandler(videoService *service.VideoService) *VideoHandler {
	return &VideoHandler{videoService: videoService}
}

func (h *VideoHandler) PublishVideo(ctx context.Context, c *app.RequestContext) {
	userID := c.GetString("user_id")
	if userID == "" {
		utils.Error(c, -1, "unauthorized")
		return
	}

	title := string(c.FormValue("title"))
	description := string(c.FormValue("description"))

	file, err := c.FormFile("data")
	if err != nil {
		utils.Error(c, -1, "failed to get video file")
		return
	}

	ext := strings.ToLower(filepath.Ext(file.Filename))
	if ext != ".mp4" && ext != ".mov" && ext != ".avi" {
		utils.Error(c, -1, "invalid video format")
		return
	}

	filename := userID + "_" + time.Now().Format("20060102150405") + ext
	uploadPath := filepath.Join("uploads/videos", filename) // 本地文件用相对路径

	if err := c.SaveUploadedFile(file, uploadPath); err != nil {
		utils.Error(c, -1, "failed to save video file")
		return
	}

	videoURL := "/uploads/videos/" + filename // 前端访问用绝对路径
	coverURL := "/uploads/videos/covers/" + strings.TrimSuffix(filename, ext) + ".jpg"

	video, err := h.videoService.PublishVideo(userID, title, description, videoURL, coverURL)
	if err != nil {
		utils.Error(c, -1, "failed to publish video")
		return
	}

	utils.Success(c, map[string]interface{}{
		"video_id":  video.ID,
		"video_url": video.VideoURL,
		"cover_url": video.CoverURL,
	})
}

func (h *VideoHandler) GetPublishList(ctx context.Context, c *app.RequestContext) {
	userID := c.Query("user_id")
	if userID == "" {
		utils.Error(c, -1, "user_id is required")
		return
	}

	pageNum, err := strconv.Atoi(c.DefaultQuery("page_num", "1"))
	if err != nil || pageNum < 1 {
		pageNum = 1
	}

	pageSize, err := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	if err != nil || pageSize < 1 {
		pageSize = 10
	}

	videos, total, err := h.videoService.GetPublishList(userID, pageNum, pageSize)
	if err != nil {
		utils.Error(c, -1, "failed to get video list")
		return
	}

	utils.Success(c, map[string]interface{}{
		"total": total,
		"items": videos,
	})
}

func (h *VideoHandler) SearchVideo(ctx context.Context, c *app.RequestContext) {
	var req struct {
		Keywords string `form:"keywords" json:"keywords"`
		Username string `form:"username" json:"username"`
		FromDate int64  `form:"from_date" json:"from_date"`
		ToDate   int64  `form:"to_date" json:"to_date"`
		PageNum  int    `form:"page_num" json:"page_num"`
		PageSize int    `form:"page_size" json:"page_size"`
	}

	if err := c.BindAndValidate(&req); err != nil {
		utils.Error(c, -1, "invalid request parameters")
		return
	}

	if req.PageNum <= 0 {
		req.PageNum = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}

	videos, total, err := h.videoService.SearchVideo(req.Keywords, req.Username, req.FromDate, req.ToDate, req.PageNum, req.PageSize)
	if err != nil {
		utils.Error(c, -1, "failed to search videos")
		return
	}

	utils.Success(c, map[string]interface{}{
		"items": videos,
		"total": total,
	})
}

func (h *VideoHandler) GetPopularVideos(ctx context.Context, c *app.RequestContext) {
	pageNum, err := strconv.Atoi(c.DefaultQuery("page_num", "1"))
	if err != nil || pageNum < 1 {
		pageNum = 1
	}

	pageSize, err := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	if err != nil || pageSize < 1 {
		pageSize = 10
	}

	videos, err := h.videoService.GetPopularVideos(pageNum, pageSize)
	if err != nil {
		utils.Error(c, -1, "failed to get popular videos")
		return
	}

	utils.Success(c, map[string]interface{}{
		"items": videos,
	})
}
