package utils

const (
	CodeSuccess = 0

	CodeParamError       = 10001
	CodeMissingParam     = 10002
	CodeInvalidParam     = 10003
	CodeInvalidFormat    = 10004

	CodeUnauthorized     = 10101
	CodeTokenInvalid     = 10102
	CodeTokenExpired     = 10103
	CodeTokenTypeInvalid = 10104
	CodeForbidden        = 10105

	CodeUserNotFound      = 10201
	CodeVideoNotFound     = 10202
	CodeCommentNotFound   = 10203
	CodeTaskNotFound      = 10204
	CodeResourceNotFound  = 10209

	CodeUserExists        = 10301
	CodeAlreadyFollowed   = 10302
	CodeAlreadyLiked      = 10303
	CodeNotFollowed       = 10304
	CodeNotLiked          = 10305
	CodeChunkAlreadyExists= 10306

	CodeFileReadError     = 10401
	CodeFileWriteError    = 10402
	CodeFileSaveError     = 10403
	CodeFileNotExist      = 10404
	CodeInvalidFileFormat = 10405
	CodeFileTooLarge      = 10406

	CodeAlreadyPublished  = 10501
	CodeUploadIncomplete  = 10502
	CodeMergeFailed       = 10503
	CodeInvalidAction     = 10504
	CodeActionNotAllowed  = 10505

	CodeInternalError     = 10601
	CodeDatabaseError     = 10602
	CodeServiceUnavail    = 10603
	CodeUnknownError      = 10699
)

var codeMessages = map[int]string{
	CodeSuccess: "success",

	CodeParamError:       "invalid parameter",
	CodeMissingParam:     "missing required parameter",
	CodeInvalidParam:     "invalid parameter value",
	CodeInvalidFormat:    "invalid format",

	CodeUnauthorized:     "unauthorized",
	CodeTokenInvalid:     "invalid token",
	CodeTokenExpired:     "token expired",
	CodeTokenTypeInvalid: "invalid token type",
	CodeForbidden:         "forbidden",

	CodeUserNotFound:      "user not found",
	CodeVideoNotFound:     "video not found",
	CodeCommentNotFound:   "comment not found",
	CodeTaskNotFound:      "upload task not found",
	CodeResourceNotFound:  "resource not found",

	CodeUserExists:        "username already exists",
	CodeAlreadyFollowed:   "already following this user",
	CodeAlreadyLiked:      "already liked this video",
	CodeNotFollowed:       "not following this user",
	CodeNotLiked:          "not liked this video",
	CodeChunkAlreadyExists:"chunk already uploaded",

	CodeFileReadError:     "failed to read file",
	CodeFileWriteError:    "failed to write file",
	CodeFileSaveError:     "failed to save file",
	CodeFileNotExist:      "file does not exist",
	CodeInvalidFileFormat: "invalid file format",
	CodeFileTooLarge:      "file too large",

	CodeAlreadyPublished:  "video already published",
	CodeUploadIncomplete:  "upload incomplete",
	CodeMergeFailed:       "failed to merge chunks",
	CodeInvalidAction:     "invalid action",
	CodeActionNotAllowed:  "action not allowed",

	CodeInternalError:     "internal server error",
	CodeDatabaseError:     "database error",
	CodeServiceUnavail:    "service unavailable",
	CodeUnknownError:      "unknown error",
}

func GetMsg(code int) string {
	if msg, ok := codeMessages[code]; ok {
		return msg
	}
	return "unknown error"
}
