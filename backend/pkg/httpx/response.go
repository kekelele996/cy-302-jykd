package httpx

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/gbexam/online-exam/internal/constants"
)

// Body is the unified API envelope.
type Body struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

// OK writes a successful response.
func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Body{Code: constants.CodeOK, Message: "ok", Data: data})
}

// Created writes a successful creation response.
func Created(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, Body{Code: constants.CodeOK, Message: "ok", Data: data})
}

// Fail writes an error response.
func Fail(c *gin.Context, httpStatus, code int, message string) {
	c.JSON(httpStatus, Body{Code: code, Message: message, Data: nil})
}
