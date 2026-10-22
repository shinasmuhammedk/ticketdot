package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5/middleware"
)

type ErrorCode string

const (
	CodeNotFound        ErrorCode = "NOT_FOUND"
	CodeConflict        ErrorCode = "CONFLICT"
	CodeValidation      ErrorCode = "VALIDATION_ERROR"
	CodeBadRequest      ErrorCode = "BAD_REQUEST"
	CodeUnauthorized    ErrorCode = "UNAUTHORIZED"
	CodeForbidden       ErrorCode = "FORBIDDEN"
	CodeInternal        ErrorCode = "INTERNAL_SERVER_ERROR"
	CodeRateLimit       ErrorCode = "RATE_LIMIT_EXCEEDED"
	CodePayloadTooLarge ErrorCode = "PAYLOAD_TOO_LARGE"
	CodeBadGateway      ErrorCode = "BAD_GATEWAY"
)

type AppError struct {
	StatusCode int       `json:"-"`
	Code       ErrorCode `json:"code"`
	Message    string    `json:"message"`
	Err        error     `json:"-"`
}

func (e *AppError) Error() string {
	return e.Message
}

func (e *AppError) Unwrap() error {
	return e.Err
}

func NotFound(msg string) *AppError {
	return &AppError{StatusCode: http.StatusNotFound, Code: CodeNotFound, Message: msg}
}

func Conflict(msg string) *AppError {
	return &AppError{StatusCode: http.StatusConflict, Code: CodeConflict, Message: msg}
}

func Validation(msg string) *AppError {
	return &AppError{StatusCode: http.StatusBadRequest, Code: CodeValidation, Message: msg}
}

func BadRequest(msg string) *AppError {
	return &AppError{StatusCode: http.StatusBadRequest, Code: CodeBadRequest, Message: msg}
}

func Unauthorized(msg string) *AppError {
	return &AppError{StatusCode: http.StatusUnauthorized, Code: CodeUnauthorized, Message: msg}
}

func Forbidden(msg string) *AppError {
	return &AppError{StatusCode: http.StatusForbidden, Code: CodeForbidden, Message: msg}
}

func PayloadTooLarge(msg string) *AppError {
	return &AppError{StatusCode: http.StatusRequestEntityTooLarge, Code: CodePayloadTooLarge, Message: msg}
}

func RateLimit(msg string) *AppError {
	return &AppError{StatusCode: http.StatusTooManyRequests, Code: CodeRateLimit, Message: msg}
}

func BadGateway(msg string) *AppError {
	return &AppError{StatusCode: http.StatusBadGateway, Code: CodeBadGateway, Message: msg}
}

func Internal(err error) *AppError {
	return &AppError{
		StatusCode: http.StatusInternalServerError,
		Code:       CodeInternal,
		Message:    "internal server error",
		Err:        err,
	}
}

type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func DecodeJSON(r *http.Request, dst interface{}, disallowUnknown bool) error {
	if r.Body == nil {
		return Validation("request body cannot be empty")
	}
	dec := json.NewDecoder(r.Body)
	if disallowUnknown {
		dec.DisallowUnknownFields()
	}
	if err := dec.Decode(dst); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			return PayloadTooLarge("request body exceeds maximum allowed size")
		}
		if strings.Contains(err.Error(), "unknown field") {
			return Validation(err.Error())
		}
		if errors.Is(err, io.EOF) {
			return Validation("request body cannot be empty")
		}
		return Validation("invalid request payload: " + err.Error())
	}
	return nil
}

func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	reqID := middleware.GetReqID(r.Context())

	var appErr *AppError
	if !errors.As(err, &appErr) {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			appErr = PayloadTooLarge("request body exceeds maximum allowed size")
		} else {
			appErr = Internal(err)
		}
	}

	if appErr.StatusCode >= 500 {
		if appErr.Err != nil {
			log.Printf("[ERROR] [request_id: %s] %v", reqID, appErr.Err)
		} else {
			log.Printf("[ERROR] [request_id: %s] %s", reqID, appErr.Message)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(appErr.StatusCode)
	_ = json.NewEncoder(w).Encode(ErrorResponse{
		Error: ErrorDetail{
			Code:    string(appErr.Code),
			Message: appErr.Message,
		},
	})
}
