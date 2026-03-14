package handler

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"
	"video/biz/service"
	"video/biz/utils"

	"github.com/cloudwego/hertz/pkg/app"
)

func init() {
	// 确保头像上传目录存在
	if err := os.MkdirAll("uploads/avatars", 0755); err != nil {
		// 在init中无法处理错误，但会在首次上传时处理
	}
}

type UserHandler struct {
	userService *service.UserService
}

func NewUserHandler(userService *service.UserService) *UserHandler {
	return &UserHandler{userService: userService}
}

func (h *UserHandler) Register(ctx context.Context, c *app.RequestContext) {
	var req struct {
		Username string `form:"username" json:"username"`
		Password string `form:"password" json:"password"`
	}

	if err := c.BindAndValidate(&req); err != nil {
		utils.Error(c, -1, "invalid request parameters")
		return
	}

	// 用户可以处理的错误单独返回
	user, err := h.userService.Register(req.Username, req.Password)
	if err != nil {
		if strings.Contains(err.Error(), "username already exists") {
			utils.Error(c, -1, "username already exists")
		} else {
			utils.Error(c, -1, err.Error())
		}
		return
	}

	utils.Success(c, map[string]interface{}{
		"id":         user.ID,
		"username":   user.Username,
		"avatar_url": user.AvatarURL,
	})
}

func (h *UserHandler) Login(ctx context.Context, c *app.RequestContext) {
	var req struct {
		Username string `form:"username" json:"username"`
		Password string `form:"password" json:"password"`
		Code     string `form:"code" json:"code"`
	}

	if err := c.BindAndValidate(&req); err != nil {
		utils.Error(c, -1, "invalid request parameters")
		return
	}

	user, accessToken, refreshToken, err := h.userService.Login(req.Username, req.Password)
	if err != nil {
		if strings.Contains(err.Error(), "user not found") {
			utils.Error(c, -1, "user not found")
		} else if strings.Contains(err.Error(), "invalid password") {
			utils.Error(c, -1, "invalid password")
		} else {
			utils.Error(c, -1, err.Error())
		}
		return
	}

	c.Header("Access-Token", accessToken)
	c.Header("Refresh-Token", refreshToken)

	utils.Success(c, map[string]interface{}{
		"id":         user.ID,
		"username":   user.Username,
		"avatar_url": user.AvatarURL,
		"created_at": user.CreatedAt,
		"updated_at": user.UpdatedAt,
	})
}

func (h *UserHandler) GetUserInfo(ctx context.Context, c *app.RequestContext) {
	userID := c.Query("user_id")
	if userID == "" {
		utils.Error(c, -1, "user_id is required")
		return
	}

	user, err := h.userService.GetUserInfo(userID)
	if err != nil {
		utils.Error(c, -1, "user not found")
		return
	}

	utils.Success(c, map[string]interface{}{
		"id":         user.ID,
		"username":   user.Username,
		"avatar_url": user.AvatarURL,
		"created_at": user.CreatedAt,
		"updated_at": user.UpdatedAt,
	})
}

func (h *UserHandler) UploadAvatar(ctx context.Context, c *app.RequestContext) {
	userID := c.GetString("user_id")
	if userID == "" {
		utils.Error(c, -1, "unauthorized")
		return
	}

	file, err := c.FormFile("data")
	if err != nil {
		utils.Error(c, -1, "failed to get file")
		return
	}

	ext := strings.ToLower(filepath.Ext(file.Filename))
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" {
		utils.Error(c, -1, "invalid file format")
		return
	}

	filename := userID + "_" + time.Now().Format("20060102150405") + ext
	uploadPath := filepath.Join("uploads/avatars", filename)

	if err := c.SaveUploadedFile(file, uploadPath); err != nil {
		utils.Error(c, -1, "failed to save file")
		return
	}

	avatarURL := "uploads/avatars/" + filename

	user, err := h.userService.UpdateAvatar(userID, avatarURL)
	if err != nil {
		utils.Error(c, -1, "failed to update avatar")
		return
	}

	utils.Success(c, map[string]interface{}{
		"id":         user.ID,
		"username":   user.Username,
		"avatar_url": user.AvatarURL,
		"created_at": user.CreatedAt,
		"updated_at": user.UpdatedAt,
	})
}
