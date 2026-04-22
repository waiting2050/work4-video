package handler

import (
	"context"
	"strconv"
	"video/biz/service"
	"video/biz/utils"

	"github.com/cloudwego/hertz/pkg/app"
)

type InteractionHandler struct {
	interactionService *service.InteractionService
}

func NewInteractionHandler(InteractionService *service.InteractionService) *InteractionHandler {
	return &InteractionHandler{
		interactionService: InteractionService,
	}
}

func (h *InteractionHandler) LikeAction(ctx context.Context, c *app.RequestContext) {
	userID := c.GetString("user_id")
	if userID == "" {
		utils.Error(c, utils.CodeUnauthorized, "unauthorized")
		return
	}

	videoID := string(c.FormValue("video_id"))
	actionTypeStr := string(c.FormValue("action_type"))

	if videoID == "" {
		utils.Error(c, utils.CodeMissingParam, "video_id is required")
		return
	}

	actionType, err := strconv.Atoi(actionTypeStr)
	if err != nil {
		utils.Error(c, utils.CodeInvalidParam, "invalid action_type")
		return
	}

	if actionType != 1 && actionType != 2 {
		utils.Error(c, utils.CodeInvalidAction, "action_type must be 1 (like) or 2 (unlike)")
		return
	}

	if err := h.interactionService.LikeAction(userID, videoID, actionType); err != nil {
		if appErr, ok := utils.IsAppError(err); ok {
			utils.Error(c, appErr.Code, appErr.Message)
		} else {
			utils.Error(c, utils.CodeInternalError, err.Error())
		}
		return
	}

	utils.Success(c, map[string]interface{}{
		"message": "success",
	})
}

func (h *InteractionHandler) GetLikeList(ctx context.Context, c *app.RequestContext) {
	userID := c.Query("user_id")
	if userID == "" {
		utils.Error(c, utils.CodeMissingParam, "user_id is required")
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

	videos, total, err := h.interactionService.GetLikeList(userID, pageNum, pageSize)
	if err != nil {
		if appErr, ok := utils.IsAppError(err); ok {
			utils.Error(c, appErr.Code, appErr.Message)
		} else {
			utils.Error(c, utils.CodeDatabaseError, err.Error())
		}
		return
	}

	utils.Success(c, map[string]interface{}{
		"items": videos,
		"total": total,
	})
}

func (h *InteractionHandler) PublishComment(ctx context.Context, c *app.RequestContext) {
	userID := c.GetString("user_id")
	if userID == "" {
		utils.Error(c, utils.CodeUnauthorized, "unauthorized")
		return
	}

	videoID := string(c.FormValue("video_id"))
	content := string(c.FormValue("content"))

	if videoID == "" {
		utils.Error(c, utils.CodeMissingParam, "video_id is required")
		return
	}

	if content == "" {
		utils.Error(c, utils.CodeMissingParam, "content is required")
		return
	}

	comment, err := h.interactionService.PublishComment(userID, videoID, content)
	if err != nil {
		if appErr, ok := utils.IsAppError(err); ok {
			utils.Error(c, appErr.Code, appErr.Message)
		} else {
			utils.Error(c, utils.CodeInternalError, err.Error())
		}
		return
	}

	utils.Success(c, map[string]interface{}{
		"comment_id": comment.ID,
		"content":    comment.Content,
		"created_at": comment.CreatedAt,
	})
}

func (h *InteractionHandler) GetCommentList(ctx context.Context, c *app.RequestContext) {
	videoID := c.Query("video_id")

	if videoID == "" {
		utils.Error(c, utils.CodeMissingParam, "video_id is required")
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

	comments, total, svcErr := h.interactionService.GetCommentList(videoID, pageNum, pageSize)
	if svcErr != nil {
		if appErr, ok := utils.IsAppError(svcErr); ok {
			utils.Error(c, appErr.Code, appErr.Message)
		} else {
			utils.Error(c, utils.CodeDatabaseError, svcErr.Error())
		}
		return
	}

	utils.Success(c, map[string]interface{}{
		"items": comments,
		"total": total,
	})
}

func (h *InteractionHandler) DeleteComment(ctx context.Context, c *app.RequestContext) {
	userID := c.GetString("user_id")
	if userID == "" {
		utils.Error(c, utils.CodeUnauthorized, "unauthorized")
		return
	}

	commentID := string(c.FormValue("comment_id"))

	if commentID == "" {
		utils.Error(c, utils.CodeMissingParam, "comment_id is required")
		return
	}

	if err := h.interactionService.DeleteComment(userID, commentID); err != nil {
		if appErr, ok := utils.IsAppError(err); ok {
			utils.Error(c, appErr.Code, appErr.Message)
		} else {
			utils.Error(c, utils.CodeInternalError, err.Error())
		}
		return
	}

	utils.Success(c, map[string]interface{}{
		"message": "success",
	})
}
