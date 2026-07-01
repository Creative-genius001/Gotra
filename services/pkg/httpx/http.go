package httpx

import (
	"errors"
	"net/http"
	"strconv"

	errorMap "github.com/gotra/gotra/utils/error"

	"github.com/gin-gonic/gin"
)

// CachePublic marks a response as cacheable by browsers and shared caches/CDNs.
func CachePublic(c *gin.Context, seconds int) {
	c.Header("Cache-Control", "public, max-age="+strconv.Itoa(seconds))
}

// CachePrivate marks a response as cacheable by the requesting browser only.
func CachePrivate(c *gin.Context, seconds int) {
	c.Header("Cache-Control", "private, max-age="+strconv.Itoa(seconds))
}

func OK(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, gin.H{"data": data})
}

func Created(c *gin.Context, data interface{}) {
	c.JSON(http.StatusCreated, gin.H{"data": data})
}

func Error(c *gin.Context, status int, message string) {
	c.AbortWithStatusJSON(status, gin.H{"error": message, "code": status})
}

func InternalServerError(c *gin.Context, appErr *errorMap.AppError) {
	if appErr != nil {
		c.Error(appErr)
	}
	Error(c, http.StatusInternalServerError, "internal server error")
}

func BadRequest(c *gin.Context, appErr *errorMap.AppError) {
	c.Error(appErr)
	Error(c, http.StatusBadRequest, appErr.Message)
}

func Unauthorized(c *gin.Context, appErr *errorMap.AppError) {
	c.Error(appErr)
	Error(c, http.StatusUnauthorized, appErr.Message)
}

func Forbidden(c *gin.Context, appErr *errorMap.AppError) {
	c.Error(appErr)
	Error(c, http.StatusForbidden, appErr.Message)
}

func NotFound(c *gin.Context, appErr *errorMap.AppError) {
	c.Error(appErr)
	Error(c, http.StatusNotFound, appErr.Message)
}

func Conflict(c *gin.Context, appErr *errorMap.AppError) {
	c.Error(appErr)
	Error(c, http.StatusConflict, appErr.Message)
}

func ServiceUnavailable(c *gin.Context, appErr *errorMap.AppError) {
	c.Error(appErr)
	Error(c, http.StatusServiceUnavailable, appErr.Message)
}

func Timeout(c *gin.Context, appErr *errorMap.AppError) {
	c.Error(appErr)
	Error(c, http.StatusGatewayTimeout, appErr.Message)
}

func MapError(c *gin.Context, err error) {
	var appErr *errorMap.AppError
	if errors.As(err, &appErr) {
		switch appErr.Code {
		case errorMap.CodeInvalidInput:
			BadRequest(c, appErr)
			return
		case errorMap.CodeAlreadyExists:
			Conflict(c, appErr)
			return
		case errorMap.CodeUnauthorized:
			Unauthorized(c, appErr)
			return
		case errorMap.CodeUnavailable:
			ServiceUnavailable(c, appErr)
			return
		case errorMap.CodeForbidden:
			Forbidden(c, appErr)
			return
		case errorMap.CodeUnknown:
			BadRequest(c, appErr)
			return
		case errorMap.CodeTimeout:
			Timeout(c, appErr)
			return
		default:
			InternalServerError(c, appErr)
			return
		}
	}
	InternalServerError(c, errorMap.New(errorMap.CodeUnknown, "Internal Server Error", err.Error()))
}
