package errors

import (
	"fmt"

	"github.com/pkg/errors"
)

// Предопределенные типы ошибок
var (
	ErrNotFound      = errors.New("resource not found")
	ErrAlreadyExists = errors.New("resource already exists")
	ErrUnauthorized  = errors.New("unauthorized")
	ErrForbidden     = errors.New("forbidden")
	ErrBadRequest    = errors.New("bad request")
	ErrInternal      = errors.New("internal server error")
	ErrTimeout       = errors.New("operation timeout")
	ErrUnavailable   = errors.New("service unavailable")
)

// Wrap оборачивает ошибку с дополнительным контекстом
func Wrap(err error, message string) error {
	if err == nil {
		return nil
	}
	return errors.Wrap(err, message)
}

// Wrapf оборачивает ошибку с форматированным сообщением
func Wrapf(err error, format string, args ...interface{}) error {
	if err == nil {
		return nil
	}
	return errors.Wrapf(err, format, args...)
}

// New создает новую ошибку
func New(message string) error {
	return errors.New(message)
}

// Newf создает новую ошибку с форматированием
func Newf(format string, args ...interface{}) error {
	return fmt.Errorf(format, args...)
}

// Cause возвращает корневую причину ошибки
func Cause(err error) error {
	return errors.Cause(err)
}

// Is проверяет, является ли ошибка определенным типом
func Is(err, target error) bool {
	return errors.Is(err, target)
}

// WithStack добавляет stack trace к ошибке
func WithStack(err error) error {
	if err == nil {
		return nil
	}
	return errors.WithStack(err)
}
