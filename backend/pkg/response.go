package pkg

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    data,
	})
}

func Error(c *gin.Context, httpCode int, message string) {
	c.JSON(httpCode, Response{
		Code:    -1,
		Message: message,
	})
}

func ErrorWithCode(c *gin.Context, httpCode int, code int, message string) {
	c.JSON(httpCode, Response{
		Code:    code,
		Message: message,
	})
}

// RunningHub-compatible response format
type RHResponse struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data,omitempty"`
}

func RHSuccess(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, RHResponse{
		Code: 0,
		Msg:  "success",
		Data: data,
	})
}

func RHError(c *gin.Context, code int, msg string) {
	c.JSON(http.StatusOK, RHResponse{
		Code: code,
		Msg:  msg,
	})
}
