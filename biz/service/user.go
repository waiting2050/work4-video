package service

import (
	"encoding/base64"
	"log"
	"time"

	"video/biz/auth"
	"video/biz/model"
	"video/biz/utils"

	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"
	"github.com/skip2/go-qrcode"
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
		return nil, utils.New(utils.CodeUserExists)
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("[UserService.Register] Failed to hash password: %v", err)
		return nil, utils.Wrap(err, utils.CodeInternalError)
	}

	user := model.User{
		ID:           uuid.New().String(),
		Username:     username,
		Password:     string(hashedPassword),
		AvatarURL: "https://api.dicebear.com/7.x/avataaars/svg?seed=" + username,
	}

	if err := s.db.Create(&user).Error; err != nil {
		log.Printf("[UserService.Register] Failed to create user: %v", err)
		return nil, utils.Wrap(err, utils.CodeInternalError)
	}

	log.Printf("[UserService.Register] Successfully created user: %s", username)
	return &user, nil
}

func (s *UserService) Login(username, password, code string) (*model.User, string, string, error) {
	var user model.User
	if err := s.db.Where("username = ?", username).First(&user).Error; err != nil {
		log.Printf("[UserService.Login] User not found: %s", username)
		return nil, "", "", utils.New(utils.CodeUserNotFound)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		log.Printf("[UserService.Login] Invalid password for user: %s", username)
		return nil, "", "", utils.New(utils.CodeInvalidPassword)
	}

	if user.MFAEnabled {
		if code == "" {
			return nil, "", "", utils.New(utils.CodeMFARequired)
		}
		if err := s.ValidateMFACode(user.ID, code); err != nil {
			return nil, "", "", err
		}
	}

	accessToken, err := auth.GenerateAccessToken(user.ID)
	if err != nil {
		log.Printf("[UserService.Login] Failed to generate access token: %v", err)
		return nil, "", "", utils.Wrap(err, utils.CodeInternalError)
	}

	refreshToken, err := auth.GenerateRefreshToken(user.ID)
	if err != nil {
		log.Printf("[UserService.Login] Failed to generate refresh token: %v", err)
		return nil, "", "", utils.Wrap(err, utils.CodeInternalError)
	}

	log.Printf("[UserService.Login] Successfully logged in user: %s", username)
	return &user, accessToken, refreshToken, nil
}

func (s *UserService) GetUserInfo(userID string) (*model.User, error) {
	var user model.User
	if err := s.db.Where("id = ?", userID).First(&user).Error; err != nil {
		log.Printf("[UserService.GetUserInfo] User not found: %s", userID)
		return nil, utils.New(utils.CodeUserNotFound)
	}
	return &user, nil
}

func (s *UserService) UpdateAvatar(userID, avatarURL string) (*model.User, error) {
	result := s.db.Model(&model.User{}).Where("id = ?", userID).Update("avatar_url", avatarURL)
	if result.Error != nil {
		log.Printf("[UserService.UpdateAvatar] Failed to update avatar: %v", result.Error)
		return nil, utils.Wrap(result.Error, utils.CodeInternalError)
	}
	if result.RowsAffected == 0 {
		log.Printf("[UserService.UpdateAvatar] User not found: %s", userID)
		return nil, utils.New(utils.CodeUserNotFound)
	}

	var user model.User
	if err := s.db.Where("id = ?", userID).First(&user).Error; err != nil {
		log.Printf("[UserService.UpdateAvatar] Failed to get updated user: %v", err)
		return nil, utils.Wrap(err, utils.CodeInternalError)
	}

	log.Printf("[UserService.UpdateAvatar] Successfully updated avatar for user: %s", userID)
	return &user, nil
}

func (s *UserService) GenerateMFASecret(userID string) (string, string, error) {
	user, err := s.getUserByID(userID)
	if err != nil {
		log.Printf("[UserService.GenerateMFASecret] User not found: %s", userID)
		return "", "", utils.New(utils.CodeUserNotFound)
	}

	if user.MFAEnabled {
		return "", "", utils.New(utils.CodeInvalidAction)
	}

	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "SilunVideo",
		AccountName: user.Username,
		SecretSize:  20,
	})
	if err != nil {
		log.Printf("[UserService.GenerateMFASecret] Failed to generate MFA secret: %v", err)
		return "", "", utils.Wrap(err, utils.CodeInternalError)
	}

	secretBase32 := key.Secret()
	otpURL := key.URL()

	qrPNG, err := qrcode.Encode(otpURL, qrcode.Medium, 256)
	if err != nil {
		log.Printf("[UserService.GenerateMFASecret] Failed to generate QR code: %v", err)
		return "", "", utils.Wrap(err, utils.CodeInternalError)
	}
	qrCodeBase64 := base64.StdEncoding.EncodeToString(qrPNG)

	log.Printf("[UserService.GenerateMFASecret] Successfully generated MFA secret for user: %s", userID)
	return secretBase32, qrCodeBase64, nil
}

func (s *UserService) EnableMFA(userID, code, secret string) error {
	user, err := s.getUserByID(userID)
	if err != nil {
		log.Printf("[UserService.EnableMFA] User not found: %s", userID)
		return utils.New(utils.CodeUserNotFound)
	}

	if user.MFAEnabled {
		return utils.New(utils.CodeInvalidAction)
	}

	if !s.validateTOTP(code, secret) {
		log.Printf("[UserService.EnableMFA] Invalid MFA code for user: %s", userID)
		return utils.New(utils.CodeMFAInvalid)
	}

	if err := s.db.Model(&model.User{}).Where("id = ?", userID).Updates(map[string]interface{}{
		"mfa_secret": secret,
		"mfa_enabled": true,
	}).Error; err != nil {
		return utils.Wrap(err, utils.CodeInternalError)
	}

	log.Printf("[UserService.EnableMFA] Successfully enabled MFA for user: %s", userID)
	return nil
}

func (s *UserService) ValidateMFACode(userID, code string) error {
	user, err := s.getUserByID(userID)
	if err != nil {
		log.Printf("[UserService.ValidateMFACode] User not found: %s", userID)
		return utils.New(utils.CodeUserNotFound)
	}

	if !user.MFAEnabled {
		return utils.New(utils.CodeMFANotEnabled)
	}

	if !s.validateTOTP(code, user.MFASecret) {
		log.Printf("[UserService.ValidateMFACode] Invalid MFA code for user: %s", userID)
		return utils.New(utils.CodeMFAInvalid)
	}

	return nil
}

func (s *UserService) getUserByID(userID string) (*model.User, error) {
	var user model.User
	if err := s.db.Where("id = ?", userID).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (s *UserService) validateTOTP(code, secret string) bool {
	if totp.Validate(code, secret) {
		return true
	}
	t := time.Now()
	opts := totp.ValidateOpts{
		Period: 30,
		Skew:   1,
	}
	valid, _ := totp.ValidateCustom(code, secret, t, opts)
	return valid
}
