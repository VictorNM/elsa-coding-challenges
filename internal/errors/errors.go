package errors

import (
	"errors"
	"fmt"
	"net/http"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Code codes.Code

const (
	CodeInvalidArgument = Code(codes.InvalidArgument)
	CodeNotFound        = Code(codes.NotFound)
	CodeAlreadyExists   = Code(codes.AlreadyExists)
	CodeInternal        = Code(codes.Internal)
	CodeUnauthenticated = Code(codes.Unauthenticated)
)

var code2http = map[Code]int{
	CodeInvalidArgument: http.StatusBadRequest,
	CodeNotFound:        http.StatusNotFound,
	CodeAlreadyExists:   http.StatusConflict,
	CodeInternal:        http.StatusInternalServerError,
	CodeUnauthenticated: http.StatusUnauthorized,
}

type Error struct {
	Code    Code   `json:"code"`
	Message string `json:"message"`
	err     error
}

func New(code Code, opts ...Option) *Error {
	e := &Error{
		Code:    code,
		Message: codes.Code(code).String(),
	}

	for _, opt := range opts {
		opt.apply(e)
	}

	return e
}

func (e *Error) Error() string {
	s := fmt.Sprintf("code: %d, message: %s", e.Code, e.Message)
	if e.err != nil {
		s += fmt.Sprintf(", err: %s", e.err)
	}

	return s
}

func (e *Error) Unwrap() error {
	return e.err
}

func (e *Error) GRPCStatus() *status.Status {
	return status.New(codes.Code(e.Code), e.Message)
}

func (e *Error) HTTPStatusCode() int {
	if c, ok := code2http[e.Code]; ok {
		return c
	}

	return http.StatusInternalServerError
}

func Convert(err error) *Error {
	var e *Error
	if !errors.As(err, &e) {
		return Internal(err)
	}

	return e
}

func Internal(err error) *Error {
	return New(CodeInternal, WithCause(err))
}

type Option interface {
	apply(*Error)
}

type optionFunc func(*Error)

func (f optionFunc) apply(e *Error) {
	f(e)
}

func WithCause(err error) Option {
	return optionFunc(func(e *Error) {
		e.err = err
	})
}

func WithMessagef(format string, args ...any) Option {
	return optionFunc(func(e *Error) {
		e.Message = fmt.Sprintf(format, args...)
	})
}
