package handler

import (
	"context"
	"video/biz/service"
	"video/biz/utils"

	"github.com/cloudwego/hertz/pkg/app"
)

type UploadStrategyHandler struct {
	strategyService *service.UploadStrategyService
}

func NewUploadStrategyHandler(strategyService *service.UploadStrategyService) *UploadStrategyHandler {
	return &UploadStrategyHandler{strategyService: strategyService}
}

func (h *UploadStrategyHandler) GetUploadStrategy(ctx context.Context, c *app.RequestContext) {
	var req struct {
		FileName       string `form:"file_name" json:"file_name" binding:"required"`
		FileSize       int64  `form:"file_size" json:"file_size" binding:"required"`
		ContentType    string `form:"content_type" json:"content_type"`
		NetworkType    string `form:"network_type" json:"network_type"`
		UserPreference string `form:"user_preference" json:"user_preference"`
	}

	if err := c.BindAndValidate(&req); err != nil {
		utils.Error(c, utils.CodeInvalidParam, "invalid request parameters")
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

func (h *UploadStrategyHandler) GetUploadRecommendation(ctx context.Context, c *app.RequestContext) {
	fileSizeStr := c.Query("file_size")
	fileName := c.Query("file_name")

	if fileSizeStr == "" || fileName == "" {
		utils.Error(c, utils.CodeMissingParam, "file_size and file_name are required")
		return
	}

	fileSize, err := utils.ParseInt64(fileSizeStr)
	if err != nil || fileSize <= 0 {
		utils.Error(c, utils.CodeInvalidParam, "invalid file_size")
		return
	}

	recommendation := h.strategyService.GetUploadRecommendation(fileSize, fileName)

	utils.Success(c, recommendation)
}
