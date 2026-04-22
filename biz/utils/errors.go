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

func New(code int, message string) error {
	return &AppError{Code: code, Message: message}
}

func IsAppError(err error) (*AppError, bool) {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr, true
	}
	return nil, false
}

func AsAppError(err error) *AppError {
	if appErr, ok := IsAppError(err); ok {
		return appErr
	}
	return nil
}

func Wrap(err error, code int, message string) error {
	if err == nil {
		return nil
	}
	return &AppError{
		Code:    code,
		Message: message,
	}
}
