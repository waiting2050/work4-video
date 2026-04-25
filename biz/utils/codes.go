package utils

// 错误码定义，用于API响应中的状态码
const (
	// 成功
	CodeSuccess = 0

	// 参数错误类（10000-10099）
	CodeParamError    = 10001 // 参数错误
	CodeMissingParam  = 10002 // 缺少必填参数
	CodeInvalidParam  = 10003 // 参数值无效
	CodeInvalidFormat = 10004 // 格式无效

	// 认证授权错误类（10100-10199）
	CodeUnauthorized     = 10101 // 未授权
	CodeTokenInvalid     = 10102 // 令牌无效
	CodeTokenExpired     = 10103 // 令牌过期
	CodeTokenTypeInvalid = 10104 // 令牌类型无效
	CodeForbidden        = 10105 // 禁止访问

	// 资源不存在错误类（10200-10299）
	CodeUserNotFound     = 10201 // 用户不存在
	CodeVideoNotFound    = 10202 // 视频不存在
	CodeCommentNotFound  = 10203 // 评论不存在
	CodeTaskNotFound     = 10204 // 上传任务不存在
	CodeResourceNotFound = 10209 // 资源不存在
	CodeMFARequired      = 10211 // 需要MFA认证
	CodeMFAInvalid       = 10212 // MFA认证无效
	CodeMFANotEnabled    = 10213 // MFA未启用
	CodeInvalidPassword  = 10214 // 密码无效

	// 业务逻辑错误类（10300-10399）
	CodeUserExists         = 10301 // 用户已存在
	CodeAlreadyFollowed    = 10302 // 已经关注该用户
	CodeAlreadyLiked       = 10303 // 已经点赞该视频
	CodeNotFollowed        = 10304 // 未关注该用户
	CodeNotLiked           = 10305 // 未点赞该视频
	CodeChunkAlreadyExists = 10306 // 分片已经上传

	// 文件操作错误类（10400-10499）
	CodeFileReadError     = 10401 // 文件读取失败
	CodeFileWriteError    = 10402 // 文件写入失败
	CodeFileSaveError     = 10403 // 文件保存失败
	CodeFileNotExist      = 10404 // 文件不存在
	CodeInvalidFileFormat = 10405 // 文件格式无效
	CodeFileTooLarge      = 10406 // 文件过大

	// 视频上传相关错误类（10500-10599）
	CodeAlreadyPublished = 10501 // 视频已经发布
	CodeUploadIncomplete = 10502 // 上传不完整
	CodeMergeFailed      = 10503 // 分片合并失败
	CodeInvalidAction    = 10504 // 无效操作
	CodeActionNotAllowed = 10505 // 操作不允许

	// 系统错误类（10600-10699）
	CodeInternalError  = 10601 // 内部服务器错误
	CodeDatabaseError  = 10602 // 数据库错误
	CodeServiceUnavail = 10603 // 服务不可用
	CodeUnknownError   = 10699 // 未知错误
)

// codeMessages 错误码对应的错误消息映射
var codeMessages = map[int]string{
	// 成功
	CodeSuccess: "success",

	// 参数错误类
	CodeParamError:    "invalid parameter",
	CodeMissingParam:  "missing required parameter",
	CodeInvalidParam:  "invalid parameter value",
	CodeInvalidFormat: "invalid format",

	// 认证授权错误类
	CodeUnauthorized:     "unauthorized",
	CodeTokenInvalid:     "invalid token",
	CodeTokenExpired:     "token expired",
	CodeTokenTypeInvalid: "invalid token type",
	CodeForbidden:        "forbidden",
	CodeMFARequired:      "MFA verification required",
	CodeMFAInvalid:       "invalid MFA code",
	CodeMFANotEnabled:    "MFA not enabled for this user",
	CodeInvalidPassword:  "invalid password",

	// 资源不存在错误类
	CodeUserNotFound:     "user not found",
	CodeVideoNotFound:    "video not found",
	CodeCommentNotFound:  "comment not found",
	CodeTaskNotFound:     "upload task not found",
	CodeResourceNotFound: "resource not found",

	// 业务逻辑错误类
	CodeUserExists:         "username already exists",
	CodeAlreadyFollowed:    "already following this user",
	CodeAlreadyLiked:       "already liked this video",
	CodeNotFollowed:        "not following this user",
	CodeNotLiked:           "not liked this video",
	CodeChunkAlreadyExists: "chunk already uploaded",

	// 文件操作错误类
	CodeFileReadError:     "failed to read file",
	CodeFileWriteError:    "failed to write file",
	CodeFileSaveError:     "failed to save file",
	CodeFileNotExist:      "file does not exist",
	CodeInvalidFileFormat: "invalid file format",
	CodeFileTooLarge:      "file too large",

	// 视频上传相关错误类
	CodeAlreadyPublished: "video already published",
	CodeUploadIncomplete: "upload incomplete",
	CodeMergeFailed:      "failed to merge chunks",
	CodeInvalidAction:    "invalid action",
	CodeActionNotAllowed: "action not allowed",

	// 系统错误类
	CodeInternalError:  "internal server error",
	CodeDatabaseError:  "database error",
	CodeServiceUnavail: "service unavailable",
	CodeUnknownError:   "unknown error",
}

func GetMsg(code int) string {
	if msg, ok := codeMessages[code]; ok {
		return msg
	}
	return "unknown error"
}
