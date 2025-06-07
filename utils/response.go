package utils

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Response 统一API响应结构
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// ResponseWithPagination 带分页的API响应结构
type ResponseWithPagination struct {
	Code        int         `json:"code"`
	Message     string      `json:"message"`
	Data        interface{} `json:"data,omitempty"`
	TotalCount  int         `json:"totalCount"`
	CurrentPage int         `json:"currentPage"`
	PageSize    int         `json:"pageSize"`
}

// Success 返回成功响应
func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    http.StatusOK,
		Message: "success",
		Data:    data,
	})
}

// SuccessWithPagination 返回带分页的成功响应
func SuccessWithPagination(c *gin.Context, data interface{}, totalCount, currentPage, pageSize int) {
	c.JSON(http.StatusOK, ResponseWithPagination{
		Code:        http.StatusOK,
		Message:     "success",
		Data:        data,
		TotalCount:  totalCount,
		CurrentPage: currentPage,
		PageSize:    pageSize,
	})
}

// Created 返回创建成功响应
func Created(c *gin.Context, data interface{}) {
	c.JSON(http.StatusCreated, Response{
		Code:    http.StatusCreated,
		Message: "created",
		Data:    data,
	})
}

// BadRequest 返回请求错误响应
func BadRequest(c *gin.Context, message string) {
	c.JSON(http.StatusBadRequest, Response{
		Code:    http.StatusBadRequest,
		Message: message,
	})
}

// Unauthorized 返回未授权响应
func Unauthorized(c *gin.Context, message string) {
	c.JSON(http.StatusUnauthorized, Response{
		Code:    http.StatusUnauthorized,
		Message: message,
	})
}

// NotFound 返回资源未找到响应
func NotFound(c *gin.Context, message string) {
	c.JSON(http.StatusNotFound, Response{
		Code:    http.StatusNotFound,
		Message: message,
	})
}

// InternalServerError 返回服务器内部错误响应
func InternalServerError(c *gin.Context, message string) {
	c.JSON(http.StatusInternalServerError, Response{
		Code:    http.StatusInternalServerError,
		Message: message,
	})
}

// NoContent 返回无内容响应
func NoContent(c *gin.Context) {
	c.Status(http.StatusNoContent)
}