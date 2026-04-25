package handler

import (
	"context"
	"io"
	"video/biz/service"
	"video/biz/utils"

	"github.com/cloudwego/hertz/pkg/app"
)

type UploadHandler struct {
	uploadService *service.UploadService
	videoService  *service.VideoService
}

func NewUploadService(uploadService *service.UploadService, videoService *service.VideoService) *UploadHandler {
	return &UploadHandler{
		uploadService: uploadService,
		videoService:  videoService,
	}
}

func (h *UploadHandler) InitUpload(ctx context.Context, c *app.RequestContext) {
	userID, ok := utils.GetUserID(c)
	if !ok {
		return
	}

	var req service.InitUploadRequest
	if err := c.BindAndValidate(&req); err != nil {
		utils.Error(c, utils.CodeInvalidParam, "invalid request parameters")
		return
	}

	resp, err := h.uploadService.InitUpload(userID, &req)
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.Success(c, resp)
}

func (h *UploadHandler) UploadChunk(ctx context.Context, c *app.RequestContext) {
	userID, ok := utils.GetUserID(c)
	if !ok {
		return
	}

	var req service.UploadChunkRequest
	if err := c.BindAndValidate(&req); err != nil {
		utils.Error(c, utils.CodeInvalidParam, "invalid request parameters")
		return
	}

	file, err := c.FormFile("chunk")
	if err != nil {
		utils.Error(c, utils.CodeFileReadError, "failed to get chunk file")
		return
	}

	f, err := file.Open()
	if err != nil {
		utils.Error(c, utils.CodeFileReadError, "failed to open chunk file")
		return
	}
	defer f.Close()

	chunkData, err := io.ReadAll(f)
	if err != nil {
		utils.Error(c, utils.CodeFileReadError, "failed to read chunk data")
		return
	}

	if err := h.uploadService.UploadChunk(userID, &req, chunkData); err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.Success(c, map[string]interface{}{
		"chunk_index": req.ChunkIndex,
		"message":     "chunk uploaded successfully",
	})
}

func (h *UploadHandler) GetUploadStatus(ctx context.Context, c *app.RequestContext) {
	userID, ok := utils.GetUserID(c)
	if !ok {
		return
	}

	taskID := c.Query("task_id")
	if taskID == "" {
		utils.Error(c, utils.CodeMissingParam, "task_id is required")
		return
	}

	status, err := h.uploadService.GetUploadStatus(userID, taskID)
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.Success(c, status)
}

func (h *UploadHandler) MergeChunks(ctx context.Context, c *app.RequestContext) {
	userID, ok := utils.GetUserID(c)
	if !ok {
		return
	}

	var req service.MergeChunksRequest
	if err := c.BindAndValidate(&req); err != nil {
		utils.Error(c, utils.CodeInvalidParam, "invalid request parameters")
		return
	}

	videoURL, coverURL, taskID, err := h.uploadService.MergeChunks(userID, &req)
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	status, err := h.uploadService.GetUploadStatus(userID, req.TaskID)
	if err != nil {
		utils.HandleError(c, err)
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

	video, err := h.videoService.PublishVideo(userID, title, description, videoURL, coverURL)
	if err != nil {
		utils.HandleError(c, err)
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

func (h *UploadHandler) CancelUpload(ctx context.Context, c *app.RequestContext) {
	userID, ok := utils.GetUserID(c)
	if !ok {
		return
	}

	taskID := c.Query("task_id")
	if taskID == "" {
		utils.Error(c, utils.CodeMissingParam, "task_id is required")
		return
	}

	if err := h.uploadService.CancelUpload(userID, taskID); err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.Success(c, map[string]interface{}{
		"task_id": taskID,
		"message": "upload cancelled successfully",
	})
}
