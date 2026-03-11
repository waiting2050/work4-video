package service

import (
	"errors"
	"fmt"
	"log"

	"video/biz/auth"
	"video/biz/model"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type UserService struct {
	db *gorm.DB
}

// NewUserService 创建用户服务实例
// 参数：
//   - db: 数据库连接
// 返回：
//   - *UserService: 用户服务实例
func NewUserService(db *gorm.DB) *UserService {
	return &UserService{db: db}
}

// Register 用户注册
// 参数：
//   - username: 用户名
//   - password: 密码
// 返回：
//   - *model.User: 用户对象
//   - error: 错误信息
func (s *UserService) Register(username, password string) (*model.User, error) {
	// 检查用户名是否已存在
	var existingUser model.User
	if err := s.db.Where("username = ? AND deleted_at IS NULL", username).First(&existingUser).Error; err == nil {
		log.Printf("[UserService.Register] Username already exists: %s", username)
		return nil, errors.New("username already exists")
	}

	// 密码加密
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("[UserService.Register] Failed to hash password: %v", err)
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// 创建用户
	user := model.User{
		ID:        uuid.New().String(),
		Username:  username,
		Password:  string(hashedPassword),
		AvatarURL: "https://api.dicebear.com/7.x/avataaars/svg?seed=" + username,
	}

	if err := s.db.Create(&user).Error; err != nil {
		log.Printf("[UserService.Register] Failed to create user: %v", err)
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	log.Printf("[UserService.Register] Successfully created user: %s", username)
	return &user, nil
}

// Login 用户登录
// 参数：
//   - username: 用户名
//   - password: 密码
// 返回：
//   - *model.User: 用户对象
//   - string: Access Token
//   - string: Refresh Token
//   - error: 错误信息
func (s *UserService) Login(username, password string) (*model.User, string, string, error) {
	// 查询用户
	var user model.User
	if err := s.db.Where("username = ? AND deleted_at IS NULL", username).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Printf("[UserService.Login] User not found: %s", username)
			return nil, "", "", errors.New("user not found")
		}
		log.Printf("[UserService.Login] Failed to find user: %v", err)
		return nil, "", "", fmt.Errorf("failed to find user: %w", err)
	}

	// 验证密码
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		log.Printf("[UserService.Login] Invalid password for user: %s", username)
		return nil, "", "", errors.New("invalid password")
	}

	// 生成Token
	accessToken, err := auth.GenerateAccessToken(user.ID)
	if err != nil {
		log.Printf("[UserService.Login] Failed to generate access token: %v", err)
		return nil, "", "", fmt.Errorf("failed to generate access token: %w", err)
	}

	refreshToken, err := auth.GenerateRefreshToken(user.ID)
	if err != nil {
		log.Printf("[UserService.Login] Failed to generate refresh token: %v", err)
		return nil, "", "", fmt.Errorf("failed to generate refresh token: %w", err)
	}

	log.Printf("[UserService.Login] Successfully logged in user: %s", username)
	return &user, accessToken, refreshToken, nil
}

// GetUserInfo 获取用户信息
// 参数：
//   - userID: 用户ID
// 返回：
//   - *model.User: 用户对象
//   - error: 错误信息
func (s *UserService) GetUserInfo(userID string) (*model.User, error) {
	var user model.User
	if err := s.db.Where("id = ? AND deleted_at IS NULL", userID).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Printf("[UserService.GetUserInfo] User not found: %s", userID)
			return nil, errors.New("user not found")
		}
		log.Printf("[UserService.GetUserInfo] Failed to get user: %v", err)
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	return &user, nil
}

// UpdateAvatar 更新用户头像
// 参数：
//   - userID: 用户ID
//   - avatarURL: 头像URL
// 返回：
//   - *model.User: 用户对象
//   - error: 错误信息
func (s *UserService) UpdateAvatar(userID, avatarURL string) (*model.User, error) {
	// 更新头像
	result := s.db.Model(&model.User{}).Where("id = ? AND deleted_at IS NULL", userID).Update("avatar_url", avatarURL)
	if result.Error != nil {
		log.Printf("[UserService.UpdateAvatar] Failed to update avatar: %v", result.Error)
		return nil, fmt.Errorf("failed to update avatar: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		log.Printf("[UserService.UpdateAvatar] User not found: %s", userID)
		return nil, errors.New("user not found")
	}

	// 查询更新后的用户信息
	var user model.User
	if err := s.db.Where("id = ? AND deleted_at IS NULL", userID).First(&user).Error; err != nil {
		log.Printf("[UserService.UpdateAvatar] Failed to get updated user: %v", err)
		return nil, fmt.Errorf("failed to get updated user: %w", err)
	}

	log.Printf("[UserService.UpdateAvatar] Successfully updated avatar for user: %s", userID)
	return &user, nil
}
