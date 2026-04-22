package middleware

import (
	"context"
	"log"
	"runtime/debug"

	"github.com/cloudwego/hertz/pkg/app"
)

func Recovery() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[Panic Recovered] %v\n%s", r, debug.Stack())
				c.Abort()
				c.JSON(500, map[string]interface{}{
					"base": map[string]interface{}{
						"code": 10601,
						"msg":  "internal server error",
					},
				})
			}
		}()
		c.Next(ctx)
	}
}