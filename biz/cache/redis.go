package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
	"video/biz/model"

	"github.com/redis/go-redis/v9"
)

var RedisClient *redis.Client

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
	ctx := context.Background()
	key := fmt.Sprintf("popular_videos:%d:%d", pageNum, pageSize)

	data, err := RedisClient.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	
	var videos []model.Video
	err = json.Unmarshal([]byte(data), &videos)
	return videos, err
}

func SetPopularVideosCache(videos []model.Video, pageNum, pageSize int) error {
	ctx := context.Background()
	key := fmt.Sprintf("popular_videos:%d:%d", pageNum, pageSize)

	data, err := json.Marshal(videos)
	if err != nil {
		return err
	}

	return RedisClient.Set(ctx, key, data, 5*time.Minute).Err()
}

func CloseRedis() {
	if RedisClient != nil {
		RedisClient.Close()
	}
}
