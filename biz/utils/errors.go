package utils

import "errors"

type AppError struct {
	Code    int
	Message string
}

func (e *AppError) Error() string {
	return e.Message
}

func (e *AppError) Is(target error) bool {
	if t, ok := target.(*AppError); ok {
		return e.Code == t.Code
	}
	return false
}

func New(code int, message ...string) error {
	msg := ""
	if len(message) > 0 {
		msg = message[0]
	} else {
		msg = GetMsg(code)
	}
	return &AppError{Code: code, Message: msg}
}

func IsAppError(err error) (*AppError, bool) {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr, true
	}
	return nil, false
}



func Wrap(err error, code int, message ...string) error {
	if err == nil {
		return nil
	}
	msg := ""
	if len(message) > 0 {
		msg = message[0]
	} else {
		msg = GetMsg(code)
	}
	return &AppError{
		Code:    code,
		Message: msg,
	}
}
