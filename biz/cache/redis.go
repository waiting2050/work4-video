package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"video/biz/model"

	"github.com/redis/go-redis/v9"
)

var RedisClient *redis.Client

const (
	VideoLikeCountPrefix = "video:like:count:"
	UserLikeStatusPrefix = "user:like:status:"
	UserLikeZSetPrefix   = "user:like:zset:"
	LikeCountCacheTTL    = 30 * time.Minute
	LikeStatusCacheTTL   = 24 * time.Hour
	LikeZSetCacheTTL     = 24 * time.Hour
)

func InitRedis(cfg *model.Config) error {
	RedisClient = redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", cfg.Redis.Host, cfg.Redis.Port),
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := RedisClient.Ping(ctx).Result()
	if err != nil {
		return fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return nil
}

func GetPopularVideosFromCache(pageNum, pageSize int) (interface{}, error) {
	if RedisClient == nil {
		return nil, ErrRedisDown
	}

	ctx := context.Background()
	key := fmt.Sprintf("popular_videos:%d:%d", pageNum, pageSize)

	data, err := RedisClient.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, ErrCacheMiss
	}
	if err != nil {
		return nil, fmt.Errorf("redis get error: %w", err)
	}

	var videos interface{}
	err = json.Unmarshal([]byte(data), &videos)
	return videos, err
}

func SetPopularVideosCache(videos interface{}, pageNum, pageSize int) error {
	if RedisClient == nil {
		return ErrRedisDown
	}

	ctx := context.Background()
	key := fmt.Sprintf("popular_videos:%d:%d", pageNum, pageSize)

	data, err := json.Marshal(videos)
	if err != nil {
		return fmt.Errorf("json marshal error: %w", err)
	}

	return RedisClient.Set(ctx, key, data, 5*time.Minute).Err()
}

func IsUserLikedVideo(userID, videoID string) (bool, error) {
	if RedisClient == nil {
		return false, ErrRedisDown
	}

	ctx := context.Background()
	key := UserLikeStatusPrefix + userID

	result, err := RedisClient.SIsMember(ctx, key, videoID).Result()
	if err != nil {
		return false, fmt.Errorf("redis sismember error: %w", err)
	}

	return result, nil
}

func AddUserLikeStatus(userID, videoID string) error {
	if RedisClient == nil {
		return ErrRedisDown
	}

	ctx := context.Background()
	statusKey := UserLikeStatusPrefix + userID
	zsetKey := UserLikeZSetPrefix + userID
	score := float64(time.Now().UnixNano())

	pipe := RedisClient.Pipeline()
	pipe.SAdd(ctx, statusKey, videoID)
	pipe.Expire(ctx, statusKey, LikeStatusCacheTTL)
	pipe.ZAdd(ctx, zsetKey, redis.Z{Score: score, Member: videoID})
	pipe.Expire(ctx, zsetKey, LikeZSetCacheTTL)
	_, err := pipe.Exec(ctx)

	if err != nil {
		return fmt.Errorf("redis pipeline error: %w", err)
	}

	return nil
}

func RemoveUserLikeStatus(userID, videoID string) error {
	if RedisClient == nil {
		return ErrRedisDown
	}

	ctx := context.Background()
	statusKey := UserLikeStatusPrefix + userID
	zsetKey := UserLikeZSetPrefix + userID

	pipe := RedisClient.Pipeline()
	pipe.SRem(ctx, statusKey, videoID)
	pipe.ZRem(ctx, zsetKey, videoID)
	_, err := pipe.Exec(ctx)

	if err != nil {
		return fmt.Errorf("redis pipeline error: %w", err)
	}

	return nil
}

func IncrVideoLikeCount(videoID string) (int64, error) {
	if RedisClient == nil {
		return 0, ErrRedisDown
	}

	ctx := context.Background()
	key := VideoLikeCountPrefix + videoID

	result, err := RedisClient.Incr(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("redis incr error: %w", err)
	}

	if err := RedisClient.Expire(ctx, key, LikeCountCacheTTL).Err(); err != nil {
		log.Printf("[Redis] Failed to set expire for key %s: %v", key, err)
	}

	return result, nil
}

func DecrVideoLikeCount(videoID string) (int64, error) {
	if RedisClient == nil {
		return 0, ErrRedisDown
	}

	ctx := context.Background()
	key := VideoLikeCountPrefix + videoID

	result, err := RedisClient.Decr(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("redis decr error: %w", err)
	}

	if err := RedisClient.Expire(ctx, key, LikeCountCacheTTL).Err(); err != nil {
		log.Printf("[Redis] Failed to set expire for key %s: %v", key, err)
	}

	return result, nil
}

func GetUserLikeIDsFromZSet(userID string, pageNum, pageSize int) ([]string, int64, error) {
	if RedisClient == nil {
		return nil, 0, ErrRedisDown
	}

	ctx := context.Background()
	key := UserLikeZSetPrefix + userID

	total, err := RedisClient.ZCard(ctx, key).Result()
	if err != nil {
		return nil, 0, fmt.Errorf("redis zcard error: %w", err)
	}

	if total == 0 {
		return nil, 0, ErrCacheMiss
	}

	start := int64((pageNum - 1) * pageSize)
	end := start + int64(pageSize) - 1

	videoIDs, err := RedisClient.ZRevRange(ctx, key, start, end).Result()
	if err != nil {
		return nil, 0, fmt.Errorf("redis zrevrange error: %w", err)
	}

	return videoIDs, total, nil
}

const MFAPendingPrefix = "mfa:pending:"
const MFAPendingTTL = 5 * time.Minute

func SetMFAPending(userID, secret string) error {
	if RedisClient == nil {
		return ErrRedisDown
	}
	ctx := context.Background()
	key := MFAPendingPrefix + userID
	return RedisClient.Set(ctx, key, secret, MFAPendingTTL).Err()
}

func GetMFAPending(userID string) (string, error) {
	if RedisClient == nil {
		return "", ErrRedisDown
	}
	ctx := context.Background()
	key := MFAPendingPrefix + userID
	secret, err := RedisClient.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", ErrCacheMiss
	}
	if err != nil {
		return "", fmt.Errorf("redis get error: %w", err)
	}
	return secret, nil
}

func DeleteMFAPending(userID string) error {
	if RedisClient == nil {
		return ErrRedisDown
	}
	ctx := context.Background()
	key := MFAPendingPrefix + userID
	return RedisClient.Del(ctx, key).Err()
}

// 聊天相关常量
const (
	ChatRecentPrefix  = "chat:recent:"
	ChatUnreadPrefix  = "chat:unread:"
	ChatOfflinePrefix = "chat:offline:"
	ChatRecentTTL     = 24 * time.Hour
	ChatUnreadTTL     = 7 * 24 * time.Hour
	ChatOfflineTTL    = 7 * 24 * time.Hour
)

// AddRecentMessage 添加最新消息到缓存
func AddRecentMessage(roomID string, msg *model.ChatMessage) error {
	if RedisClient == nil {
		return ErrRedisDown
	}
	ctx := context.Background()
	key := ChatRecentPrefix + roomID

	msgData, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	// 使用List保留最新100条消息
	pipe := RedisClient.Pipeline()
	pipe.LPush(ctx, key, msgData)
	pipe.LTrim(ctx, key, 0, 99)
	pipe.Expire(ctx, key, ChatRecentTTL)
	_, err = pipe.Exec(ctx)

	return err
}

// GetRecentMessages 获取最近消息
func GetRecentMessages(roomID string, limit int) ([]model.ChatMessage, error) {
	if RedisClient == nil {
		return nil, ErrRedisDown
	}
	ctx := context.Background()
	key := ChatRecentPrefix + roomID

	dataList, err := RedisClient.LRange(ctx, key, 0, int64(limit-1)).Result()
	if err != nil {
		return nil, err
	}

	var messages []model.ChatMessage
	for _, data := range dataList {
		var msg model.ChatMessage
		if err := json.Unmarshal([]byte(data), &msg); err == nil {
			messages = append(messages, msg)
		}
	}
	return messages, nil
}

// IncrUnreadCount 增加未读计数
func IncrUnreadCount(roomID, userID string) error {
	if RedisClient == nil {
		return ErrRedisDown
	}
	ctx := context.Background()
	key := ChatUnreadPrefix + roomID + ":" + userID

	_, err := RedisClient.Incr(ctx, key).Result()
	if err == nil {
		_ = RedisClient.Expire(ctx, key, ChatUnreadTTL)
	}
	return err
}

// DecrUnreadCount 减少未读计数
func DecrUnreadCount(roomID, userID string) error {
	if RedisClient == nil {
		return ErrRedisDown
	}
	ctx := context.Background()
	key := ChatUnreadPrefix + roomID + ":" + userID

	count, err := RedisClient.Decr(ctx, key).Result()
	if err == nil {
		if count < 0 {
			_ = RedisClient.Set(ctx, key, 0, ChatUnreadTTL)
		} else {
			_ = RedisClient.Expire(ctx, key, ChatUnreadTTL)
		}
	}
	return err
}

// GetUnreadCount 获取未读计数
func GetUnreadCount(roomID, userID string) (int64, error) {
	if RedisClient == nil {
		return 0, ErrRedisDown
	}
	ctx := context.Background()
	key := ChatUnreadPrefix + roomID + ":" + userID

	count, err := RedisClient.Get(ctx, key).Int64()
	if err == redis.Nil {
		return 0, ErrCacheMiss
	}
	if err != nil {
		return 0, err
	}
	return count, nil
}

// ClearUnreadCount 清除未读计数
func ClearUnreadCount(roomID, userID string) error {
	if RedisClient == nil {
		return ErrRedisDown
	}
	ctx := context.Background()
	key := ChatUnreadPrefix + roomID + ":" + userID

	return RedisClient.Del(ctx, key).Err()
}

// AddOfflineMessage 添加离线消息
func AddOfflineMessage(userID string, msg *model.ChatMessage) error {
	if RedisClient == nil {
		return ErrRedisDown
	}
	ctx := context.Background()
	key := ChatOfflinePrefix + userID

	msgData, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	// 使用List保存离线消息，保留最多200条
	pipe := RedisClient.Pipeline()
	pipe.RPush(ctx, key, msgData)
	pipe.LTrim(ctx, key, -200, -1) // 保留最后200条
	pipe.Expire(ctx, key, ChatOfflineTTL)
	_, err = pipe.Exec(ctx)

	return err
}

// GetOfflineMessages 获取离线消息（并删除）
func GetOfflineMessages(userID string) ([]model.ChatMessage, error) {
	if RedisClient == nil {
		return nil, ErrRedisDown
	}
	ctx := context.Background()
	key := ChatOfflinePrefix + userID

	// 获取所有离线消息
	dataList, err := RedisClient.LRange(ctx, key, 0, -1).Result()
	if err != nil {
		return nil, err
	}

	// 删除已获取的离线消息
	_ = RedisClient.Del(ctx, key)

	var messages []model.ChatMessage
	for _, data := range dataList {
		var msg model.ChatMessage
		if err := json.Unmarshal([]byte(data), &msg); err == nil {
			messages = append(messages, msg)
		}
	}
	return messages, nil
}

// CheckUserOnline 检查用户是否在线
func CheckUserOnline(userID string) bool {
	if RedisClient == nil {
		return false
	}
	ctx := context.Background()
	key := "user:online:" + userID

	_, err := RedisClient.Get(ctx, key).Result()
	return err == nil
}

// SetUserOnline 设置用户在线
func SetUserOnline(userID string) error {
	if RedisClient == nil {
		return ErrRedisDown
	}
	ctx := context.Background()
	key := "user:online:" + userID
	return RedisClient.Set(ctx, key, "1", 24*time.Hour).Err()
}

// SetUserOffline 设置用户离线
func SetUserOffline(userID string) error {
	if RedisClient == nil {
		return ErrRedisDown
	}
	ctx := context.Background()
	key := "user:online:" + userID
	return RedisClient.Del(ctx, key).Err()
}

func CloseRedis() {
	if RedisClient != nil {
		RedisClient.Close()
	}
}
