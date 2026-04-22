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
			utils.Error(c, utils.CodeUnauthorized, "Unauthorized")
			c.Abort()
			return
		}

		userID, err := ParseAccessToken(token)
		if err != nil {
			utils.Error(c, utils.CodeTokenInvalid, "Invalid token")
			c.Abort()
			return
		}

		c.Set("user_id", userID)
		c.Next(ctx)
	}
}
