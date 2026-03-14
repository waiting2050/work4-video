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

func GetPopularVideosFromCache(pageNum, pageSize int) ([]model.Video, error) {
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

	var videos []model.Video
	err = json.Unmarshal([]byte(data), &videos)
	return videos, err
}

func SetPopularVideosCache(videos []model.Video, pageNum, pageSize int) error {
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

func CloseRedis() {
	if RedisClient != nil {
		RedisClient.Close()
	}
}
