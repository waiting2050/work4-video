package handler

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
	"video/biz/service"
	"video/biz/utils"

	"github.com/cloudwego/hertz/pkg/app"
)

func init() {
	if err := os.MkdirAll("uploads/avatars", 0755); err != nil {
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
		utils.Error(c, utils.CodeInvalidParam, "invalid request parameters")
		return
	}

	user, err := h.userService.Register(req.Username, req.Password)
	if err != nil {
		utils.HandleError(c, err)
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
		utils.Error(c, utils.CodeInvalidParam, "invalid request parameters")
		return
	}

	user, accessToken, refreshToken, err := h.userService.Login(req.Username, req.Password, req.Code)
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	c.Header("Access-Token", accessToken)
	c.Header("Refresh-Token", refreshToken)

	utils.Success(c, map[string]interface{}{
		"id":          user.ID,
		"username":    user.Username,
		"avatar_url":  user.AvatarURL,
		"mfa_enabled": user.MFAEnabled,
		"created_at":  user.CreatedAt,
		"updated_at":  user.UpdatedAt,
	})
}

func (h *UserHandler) GetMFAQRCode(ctx context.Context, c *app.RequestContext) {
	userID, ok := utils.GetUserID(c)
	if !ok {
		return
	}

	secret, qrCodeBase64, err := h.userService.GenerateMFASecret(userID)
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.Success(c, map[string]interface{}{
		"secret": secret,
		"qrcode": "data:image/png;base64," + qrCodeBase64,
	})
}

func (h *UserHandler) BindMFA(ctx context.Context, c *app.RequestContext) {
	userID, ok := utils.GetUserID(c)
	if !ok {
		return
	}

	var req struct {
		Code   string `form:"code" json:"code" binding:"required"`
		Secret string `form:"secret" json:"secret" binding:"required"`
	}

	if err := c.BindAndValidate(&req); err != nil {
		utils.Error(c, utils.CodeMissingParam, "MFA code and secret are required")
		return
	}

	if err := h.userService.EnableMFA(userID, req.Code, req.Secret); err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.Success(c, nil)
}



func (h *UserHandler) GetUserInfo(ctx context.Context, c *app.RequestContext) {
	userID := c.Query("user_id")
	if userID == "" {
		utils.Error(c, utils.CodeMissingParam, "user_id is required")
		return
	}

	user, err := h.userService.GetUserInfo(userID)
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.Success(c, map[string]interface{}{
		"id":          user.ID,
		"username":    user.Username,
		"avatar_url":  user.AvatarURL,
		"mfa_enabled": user.MFAEnabled,
		"created_at":  user.CreatedAt,
		"updated_at":  user.UpdatedAt,
	})
}

func (h *UserHandler) UploadAvatar(ctx context.Context, c *app.RequestContext) {
	userID, ok := utils.GetUserID(c)
	if !ok {
		return
	}

	file, err := c.FormFile("data")
	if err != nil {
		utils.Error(c, utils.CodeFileReadError, "failed to get file")
		return
	}

	ext := strings.ToLower(filepath.Ext(file.Filename))
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" {
		utils.Error(c, utils.CodeInvalidFileFormat, "invalid file format")
		return
	}

	filename := userID + "_" + time.Now().Format("20060102150405") + ext
	uploadPath := filepath.Join("uploads/avatars", filename)

	log.Printf("[UploadAvatar] Saving file to: %s, original filename: %s", uploadPath, file.Filename)

	if err := c.SaveUploadedFile(file, uploadPath); err != nil {
		log.Printf("[UploadAvatar] Failed to save file: %v", err)
		utils.Error(c, utils.CodeFileSaveError, "failed to save file")
		return
	}

	if _, err := os.Stat(uploadPath); os.IsNotExist(err) {
		log.Printf("[UploadAvatar] File does not exist after save: %s", uploadPath)
		utils.Error(c, utils.CodeFileNotExist, "file not found after save")
		return
	}
	log.Printf("[UploadAvatar] File saved successfully: %s", uploadPath)

	avatarURL := "uploads/avatars/" + filename

	user, err := h.userService.UpdateAvatar(userID, avatarURL)
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.Success(c, map[string]interface{}{
		"id":          user.ID,
		"username":    user.Username,
		"avatar_url":  user.AvatarURL,
		"mfa_enabled": user.MFAEnabled,
		"created_at":  user.CreatedAt,
		"updated_at":  user.UpdatedAt,
	})
}
