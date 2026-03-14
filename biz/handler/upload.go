package handler

import (
	"context"
	"io"
	"video/biz/service"
	"video/biz/utils"

	"github.com/cloudwego/hertz/pkg/app"
)

// UploadHandler 分片上传处理器
type UploadHandler struct {
	uploadService *service.UploadService
	videoService *service.VideoService
}

// 创建上传处理器
func NewUploadService(uploadService *service.UploadService, videoService *service.VideoService) *UploadHandler {
	return &UploadHandler{
		uploadService: uploadService,
		videoService: videoService,
	}
}

// 初始化分片上传
func (h *UploadHandler) InitUpload(ctx context.Context, c *app.RequestContext) {
	userID := c.GetString("user_id")
	if userID == "" {
		utils.Error(c, -1, "unauthorized")
		return 
	}

	var req service.InitUploadRequest
	if err := c.BindAndValidate(&req); err != nil {
		utils.Error(c, -1, "invalid request parameters")
		return
	}

	resp, err := h.uploadService.InitUpload(userID, &req)
	if err != nil {
		utils.Error(c, -1, err.Error())
		return
	}

	utils.Success(c, resp)
}

// 上传分片
func (h *UploadHandler) UploadChunk(ctx context.Context, c *app.RequestContext) {
	userID := c.GetString("user_id")
	if userID == "" {
		utils.Error(c, -1, "unauthorized")
		return
	}

	var req service.UploadChunkRequest
	if err := c.BindAndValidate(&req); err != nil {
		utils.Error(c, -1, "invalid request parameters")
		return
	}

	// 获取分片数据
	file, err := c.FormFile("chunk")
	if err != nil {
		utils.Error(c, -1, "failed to get chunk file")
		return
	}

	f, err := file.Open()
	if err != nil {
		utils.Error(c, -1, "failed to open chunk file")
		return
	}
	defer f.Close()

	chunkData, err := io.ReadAll(f)
	if err != nil {
		utils.Error(c, -1, "failed to read chunk data")
		return
	}

	if err := h.uploadService.UploadChunk(userID, &req, chunkData); err != nil {
		utils.Error(c, -1, err.Error())
		return
	}

	utils.Success(c, map[string]interface{}{
		"chunk_index": req.ChunkIndex,
		"message":     "chunk uploaded successfully",
	})
}

// 获取上传状态
func (h *UploadHandler) GetUploadStatus(ctx context.Context, c *app.RequestContext) {
	userID := c.GetString("user_id")
	if userID == "" {
		utils.Error(c, -1, "unauthorized")
		return
	}

	taskID := c.Query("task_id")
	if taskID == "" {
		utils.Error(c, -1, "task_id is required")
		return
	}

	status, err := h.uploadService.GetUploadStatus(userID, taskID)
	if err != nil {
		utils.Error(c, -1, err.Error())
		return
	}

	utils.Success(c, status)
}

// 合并分片
func (h *UploadHandler) MergeChunks(ctx context.Context, c *app.RequestContext) {
	userID := c.GetString("user_id")
	if userID == "" {
		utils.Error(c, -1, "unauthorized")
		return
	}

	var req service.MergeChunksRequest
	if err := c.BindAndValidate(&req); err != nil {
		utils.Error(c, -1, "invalid request parameters")
		return
	}

	videoURL, coverURL, taskID, err := h.uploadService.MergeChunks(userID, &req)
	if err != nil {
		utils.Error(c, -1, err.Error())
		return
	}

	// 获取上传任务信息以创建视频记录
	status, err := h.uploadService.GetUploadStatus(userID, req.TaskID)
	if err != nil {
		utils.Error(c, -1, err.Error())
		return
	}

	title := ""
	description := ""
	if t, ok := status["title"].(string); ok {
		title = t
	}
	if d, ok := status["description"].(string); ok {
		description = d
	}

	// 创建视频记录
	video, err := h.videoService.PublishVideo(userID, title, description, videoURL, coverURL)
	if err != nil {
		utils.Error(c, -1, "failed to publish video: "+err.Error())
		return
	}

	utils.Success(c, map[string]interface{}{
		"video_id":  video.ID,
		"video_url": video.VideoURL,
		"cover_url": video.CoverURL,
		"task_id":   taskID,
		"message":   "video uploaded and merged successfully",
	})
}

// 取消上传
func (h *UploadHandler) CancelUpload(ctx context.Context, c *app.RequestContext) {
	userID := c.GetString("user_id")
	if userID == "" {
		utils.Error(c, -1, "unauthorized")
		return
	}

	taskID := c.Query("task_id")
	if taskID == "" {
		utils.Error(c, -1, "task_id is required")
		return
	}

	if err := h.uploadService.CancelUpload(userID, taskID); err != nil {
		utils.Error(c, -1, err.Error())
		return
	}

	utils.Success(c, map[string]interface{}{
		"task_id": taskID,
		"message": "upload cancelled successfully",
	})
}
