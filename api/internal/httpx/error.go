package httpx

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/labstack/echo/v4"
)

// Code is a stable, machine-readable error identifier. The FE switches on this,
// never on the human-readable message, so messages stay free to change.
type Code string

const (
	CodeBadRequest   Code = "bad_request"
	CodeUnauthorized Code = "unauthorized"
	CodeForbidden    Code = "forbidden"
	CodeNotFound     Code = "not_found"
	CodeConflict     Code = "conflict"
	CodeValidation   Code = "validation_failed"
	CodeInternal     Code = "internal_error"
)

// Envelope is the single response shape for every failing request.
type Envelope struct {
	Error Body `json:"error"`
}

type Body struct {
	Code      Code              `json:"code"`
	Message   string            `json:"message"`
	Fields    map[string]string `json:"fields,omitempty"`
	RequestID string            `json:"request_id,omitempty"`
}

type Error struct {
	Status  int
	Code    Code
	Message string
	Fields  map[string]string
	cause   error
}

func (e *Error) Error() string {
	if e.cause != nil {
		return e.Message + ": " + e.cause.Error()
	}
	return e.Message
}

func (e *Error) Unwrap() error { return e.cause }

func New(status int, code Code, message string) *Error {
	return &Error{Status: status, Code: code, Message: message}
}

func (e *Error) WithCause(err error) *Error { e.cause = err; return e }

func (e *Error) WithFields(f map[string]string) *Error { e.Fields = f; return e }

func BadRequest(msg string) *Error   { return New(http.StatusBadRequest, CodeBadRequest, msg) }
func Unauthorized(msg string) *Error { return New(http.StatusUnauthorized, CodeUnauthorized, msg) }
func Forbidden(msg string) *Error    { return New(http.StatusForbidden, CodeForbidden, msg) }
func NotFound(msg string) *Error     { return New(http.StatusNotFound, CodeNotFound, msg) }
func Conflict(msg string) *Error     { return New(http.StatusConflict, CodeConflict, msg) }

func Validation(msg string, fields map[string]string) *Error {
	return New(http.StatusUnprocessableEntity, CodeValidation, msg).WithFields(fields)
}

// ErrorHandler converts anything returned by a handler into the envelope above.
// Unrecognised errors are logged in full but reported to the client as a bare
// internal_error, so internals never leak through the API.
func ErrorHandler(logger *slog.Logger) echo.HTTPErrorHandler {
	return func(err error, c echo.Context) {
		if c.Response().Committed {
			return
		}

		status, body := translate(err)
		body.RequestID = RequestIDFrom(c)

		if status >= http.StatusInternalServerError {
			logger.Error("request failed",
				slog.String("request_id", body.RequestID),
				slog.String("method", c.Request().Method),
				slog.String("path", c.Request().URL.Path),
				slog.String("error", err.Error()),
			)
		}

		if resErr := c.JSON(status, Envelope{Error: body}); resErr != nil {
			logger.Error("write error response", slog.String("error", resErr.Error()))
		}
	}
}

func translate(err error) (int, Body) {
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr.Status, Body{Code: appErr.Code, Message: appErr.Message, Fields: appErr.Fields}
	}

	var echoErr *echo.HTTPError
	if errors.As(err, &echoErr) {
		code := CodeBadRequest
		switch echoErr.Code {
		case http.StatusUnauthorized:
			code = CodeUnauthorized
		case http.StatusForbidden:
			code = CodeForbidden
		case http.StatusNotFound:
			code = CodeNotFound
		case http.StatusInternalServerError:
			code = CodeInternal
		}
		msg, ok := echoErr.Message.(string)
		if !ok {
			msg = http.StatusText(echoErr.Code)
		}
		return echoErr.Code, Body{Code: code, Message: msg}
	}

	return http.StatusInternalServerError, Body{
		Code:    CodeInternal,
		Message: "Terjadi kesalahan pada server.",
	}
}
