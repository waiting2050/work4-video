package handler

import (
	"context"
	"strconv"
	"video/biz/model"
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
		utils.Error(c, -1, "unauthorized")
		return
	}

	videoID := string(c.FormValue("video_id"))
	actionTypeStr := string(c.FormValue("action_type"))

	if videoID == "" {
		utils.Error(c, -1, "video_id is required")
		return
	}

	actionType, err := strconv.Atoi(actionTypeStr)
	if err != nil {
		utils.Error(c, -1, "invalid action_type")
		return
	}

	if actionType != 1 && actionType != 2 {
		utils.Error(c, -1, "action_type must be 1 (like) or 2 (unlike)")
		return
	}

	if err := h.interactionService.LikeAction(userID, videoID, actionType); err != nil {
		utils.Error(c, -1, err.Error())
		return
	}

	utils.Success(c, map[string]interface{}{
		"message": "success",
	})
}

func (h *InteractionHandler) GetLikeList(ctx context.Context, c *app.RequestContext) {
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

	videos, _, err := h.interactionService.GetLikeList(userID, pageNum, pageSize)
	if err != nil {
		utils.Error(c, -1, "failed to get like list")
		return
	}

	utils.Success(c, map[string]interface{}{
		"items": videos,
	})
}

func (h *InteractionHandler) PublishComment(ctx context.Context, c *app.RequestContext) {
	userID := c.GetString("user_id")
	if userID == "" {
		utils.Error(c, -1, "unauthorized")
		return
	}

	videoID := string(c.FormValue("video_id"))
	content := string(c.FormValue("content"))

	if videoID == "" {
		utils.Error(c, -1, "video_id is required")
		return
	}

	if content == "" {
		utils.Error(c, -1, "content is required")
		return
	}

	comment, err := h.interactionService.PublishComment(userID, videoID, content)
	if err != nil {
		utils.Error(c, -1, err.Error())
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
	commentID := c.Query("comment_id")

	if videoID == "" && commentID == "" {
		utils.Error(c, -1, "video_id or comment_id is required")
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

	var comments []interface{}
	var svcErr error

	if videoID != "" {
		var result []model.Comment
		result, _, svcErr = h.interactionService.GetCommentList(videoID, pageNum, pageSize)
		for _, c := range result {
			comments = append(comments, c)
		}
	} else {
		var result []model.Comment
		result, _, svcErr = h.interactionService.GetCommentList(commentID, pageNum, pageSize)
		for _, c := range result {
			comments = append(comments, c)
		}
	}

	if svcErr != nil {
		utils.Error(c, -1, "failed to get comment list")
		return
	}

	utils.Success(c, map[string]interface{}{
		"items": comments,
	})
}

func (h *InteractionHandler) DeleteComment(ctx context.Context, c *app.RequestContext) {
	userID := c.GetString("user_id")
	if userID == "" {
		utils.Error(c, -1, "unauthorized")
		return
	}

	commentID := string(c.FormValue("comment_id"))

	if commentID == "" {
		utils.Error(c, -1, "comment_id is required")
		return
	}

	if err := h.interactionService.DeleteComment(userID, commentID); err != nil {
		utils.Error(c, -1, err.Error())
		return
	}

	utils.Success(c, map[string]interface{}{
		"message": "success",
	})
}
