package service

import (
	"fmt"
	"log"

	"video/biz/auth"
	"video/biz/model"
	"video/biz/utils"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type UserService struct {
	db *gorm.DB
}

func NewUserService(db *gorm.DB) *UserService {
	return &UserService{db: db}
}

func (s *UserService) Register(username, password string) (*model.User, error) {
	var existingUser model.User
	if err := s.db.Where("username = ?", username).First(&existingUser).Error; err == nil {
		log.Printf("[UserService.Register] Username already exists: %s", username)
		return nil, utils.New(utils.CodeUserExists, "username already exists")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("[UserService.Register] Failed to hash password: %v", err)
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

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

func (s *UserService) Login(username, password string) (*model.User, string, string, error) {
	var user model.User
	if err := s.db.Where("username = ?", username).First(&user).Error; err != nil {
		log.Printf("[UserService.Login] User not found: %s", username)
		return nil, "", "", utils.New(utils.CodeUserNotFound, "user not found")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		log.Printf("[UserService.Login] Invalid password for user: %s", username)
		return nil, "", "", utils.New(utils.CodeUnauthorized, "invalid password")
	}

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

func (s *UserService) GetUserInfo(userID string) (*model.User, error) {
	var user model.User
	if err := s.db.Where("id = ?", userID).First(&user).Error; err != nil {
		log.Printf("[UserService.GetUserInfo] User not found: %s", userID)
		return nil, utils.New(utils.CodeUserNotFound, "user not found")
	}
	return &user, nil
}

func (s *UserService) UpdateAvatar(userID, avatarURL string) (*model.User, error) {
	result := s.db.Model(&model.User{}).Where("id = ?", userID).Update("avatar_url", avatarURL)
	if result.Error != nil {
		log.Printf("[UserService.UpdateAvatar] Failed to update avatar: %v", result.Error)
		return nil, fmt.Errorf("failed to update avatar: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		log.Printf("[UserService.UpdateAvatar] User not found: %s", userID)
		return nil, utils.New(utils.CodeUserNotFound, "user not found")
	}

	var user model.User
	if err := s.db.Where("id = ?", userID).First(&user).Error; err != nil {
		log.Printf("[UserService.UpdateAvatar] Failed to get updated user: %v", err)
		return nil, fmt.Errorf("failed to get updated user: %w", err)
	}

	log.Printf("[UserService.UpdateAvatar] Successfully updated avatar for user: %s", userID)
	return &user, nil
}
