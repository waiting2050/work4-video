package handler

import (
	"context"
	"video/biz/service"
	"video/biz/utils"

	"github.com/cloudwego/hertz/pkg/app"
)

// UploadStrategyHandler 上传策略处理器
type UploadStrategyHandler struct {
	strategyService *service.UploadStrategyService
}

// NewUploadStrategyHandler 创建上传策略处理器
func NewUploadStrategyHandler(strategyService *service.UploadStrategyService) *UploadStrategyHandler {
	return &UploadStrategyHandler{strategyService: strategyService}
}

// GetUploadStrategy 获取上传策略建议
func (h *UploadStrategyHandler) GetUploadStrategy(ctx context.Context, c *app.RequestContext) {
	var req struct {
		FileName       string `form:"file_name" json:"file_name" binding:"required"`
		FileSize       int64  `form:"file_size" json:"file_size" binding:"required"`
		ContentType    string `form:"content_type" json:"content_type"`
		NetworkType    string `form:"network_type" json:"network_type"`
		UserPreference string `form:"user_preference" json:"user_preference"`
	}

	if err := c.BindAndValidate(&req); err != nil {
		utils.Error(c, -1, "invalid request parameters")
		return
	}

	decisionReq := &service.UploadDecisionRequest{
		FileName:       req.FileName,
		FileSize:       req.FileSize,
		ContentType:    req.ContentType,
		NetworkType:    req.NetworkType,
		UserPreference: req.UserPreference,
	}

	if decisionReq.NetworkType == "" {
		decisionReq.NetworkType = "unknown"
	}

	decision := h.strategyService.DecideUploadStrategy(decisionReq)
	h.strategyService.LogUploadDecision(decisionReq, decision)

	utils.Success(c, decision)
}

// GetUploadRecommendation 获取上传建议
func (h *UploadStrategyHandler) GetUploadRecommendation(ctx context.Context, c *app.RequestContext) {
	fileSizeStr := c.Query("file_size")
	fileName := c.Query("file_name")

	if fileSizeStr == "" || fileName == "" {
		utils.Error(c, -1, "file_size and file_name are required")
		return
	}

	fileSize, err := utils.ParseInt64(fileSizeStr)
	if err != nil || fileSize <= 0 {
		utils.Error(c, -1, "invalid file_size")
		return
	}

	recommendation := h.strategyService.GetUploadRecommendation(fileSize, fileName)

	utils.Success(c, recommendation)
}
