package handler

import (
	"context"
	"strconv"
	"video/biz/service"
	"video/biz/utils"

	"github.com/cloudwego/hertz/pkg/app"
)

type SocialHandler struct {
	socialService *service.SocialService
}

func NewSocialHandler(socialService *service.SocialService) *SocialHandler {
	return &SocialHandler{
		socialService: socialService,
	}
}

func (h *SocialHandler) FollowAction(ctx context.Context, c *app.RequestContext) {
	userID, ok := utils.GetUserID(c)
	if !ok {
		return
	}

	toUserID := string(c.FormValue("to_user_id"))
	actionTypeStr := string(c.FormValue("action_type"))

	if toUserID == "" {
		utils.Error(c, utils.CodeMissingParam, "to_user_id is required")
		return
	}

	actionType, err := strconv.Atoi(actionTypeStr)
	if err != nil {
		utils.Error(c, utils.CodeInvalidParam, "invalid action type")
		return
	}

	if actionType != 0 && actionType != 1 {
		utils.Error(c, utils.CodeInvalidAction, "action_type must be 0 (follow) or 1 (unfollow)")
		return
	}

	if err := h.socialService.FollowAction(userID, toUserID, actionType); err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.Success(c, map[string]interface{}{
		"message": "success",
	})
}

func (h *SocialHandler) GetFollowList(ctx context.Context, c *app.RequestContext) {
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

	users, total, err := h.socialService.GetFollowList(userID, pageNum, pageSize)
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.Success(c, map[string]interface{}{
		"items": users,
		"total": total,
	})
}

func (h *SocialHandler) GetFollowerList(ctx context.Context, c *app.RequestContext) {
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

	users, total, err := h.socialService.GetFollowerList(userID, pageNum, pageSize)
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.Success(c, map[string]interface{}{
		"items": users,
		"total": total,
	})
}

func (h *SocialHandler) GetFriendList(ctx context.Context, c *app.RequestContext) {
	userID, ok := utils.GetUserID(c)
	if !ok {
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

	users, total, err := h.socialService.GetFriendList(userID, pageNum, pageSize)
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.Success(c, map[string]interface{}{
		"items": users,
		"total": total,
	})
}
