package auth

import (
	"context"
	"video/biz/utils"

	"github.com/cloudwego/hertz/pkg/app"
)

func AuthMiddleware() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		token := string(c.GetHeader("Access-Token"))
		if token == "" {
			utils.Error(c, -1, "Unauthorized")
			c.Abort()
			return
		}

		userID, err := ParseAccessToken(token)
		if err != nil {
			utils.Error(c, -1, "Invalid token")
			c.Abort()
			return
		}

		c.Set("user_id", userID)
		c.Next(ctx)
	}
}
