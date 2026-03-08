package handler

import (
	"context"
	"video/biz/utils"

	"github.com/cloudwego/hertz/pkg/app"
)

func Ping(ctx context.Context, c *app.RequestContext) {
	utils.Success(c, map[string]interface{}{
		"message": "pong",
		"status":  "ok",
	})
}
