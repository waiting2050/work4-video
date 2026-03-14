package auth

import (
	"context"
	"video/biz/utils"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/golang-jwt/jwt"
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

		// 验证 token 类型
		parsedToken, _, err := new(jwt.Parser).ParseUnverified(token, jwt.MapClaims{})
		if err != nil {
			utils.Error(c, -1, "Invalid token format")
			c.Abort()
			return
		}

		if claims, ok := parsedToken.Claims.(jwt.MapClaims); ok {
			if tokenType, exists := claims["type"].(string); !exists || tokenType != "access" {
				utils.Error(c, -1, "Invalid token type")
				c.Abort()
				return
			}
		}

		c.Set("user_id", userID)
		c.Next(ctx)
	}
}
