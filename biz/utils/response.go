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

// HandleError 统一处理请求错误
func HandleError(c *app.RequestContext, err error) {
	if appErr, ok := IsAppError(err); ok {
		Error(c, appErr.Code, appErr.Message)
	} else {
		Error(c, CodeInternalError, err.Error())
	}
}

// GetUserID 安全获取并验证user_id
func GetUserID(c *app.RequestContext) (string, bool) {
	userID := c.GetString("user_id")
	if userID == "" {
		Error(c, CodeUnauthorized, GetMsg(CodeUnauthorized))
		return "", false
	}
	return userID, true
}

// ParseInt64 解析字符串为 int64
func ParseInt64(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}
