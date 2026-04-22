package utils

import (
	"strconv"

	"github.com/cloudwego/hertz/pkg/app"
)

type BaseResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

type SuccessResponse struct {
	Base BaseResponse `json:"base"`
	Data interface{}  `json:"data,omitempty"`
}

func Success(c *app.RequestContext, data interface{}) {
	c.JSON(200, SuccessResponse{
		Base: BaseResponse{
			Code: CodeSuccess,
			Msg:  GetMsg(CodeSuccess),
		},
		Data: data,
	})
}

func Error(c *app.RequestContext, code int, msg string) {
	c.JSON(200, SuccessResponse{
		Base: BaseResponse{
			Code: code,
			Msg:  msg,
		},
	})
}

// ParseInt64 解析字符串为 int64
func ParseInt64(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}
