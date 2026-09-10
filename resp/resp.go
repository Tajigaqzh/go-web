package resp

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Response struct {
	Success    bool        `json:"success" example:"true"` // 请求是否成功
	Data       any         `json:"data,omitempty"`         // 响应数据
	Error      *ErrorInfo  `json:"error,omitempty"`        // 请求失败时的错误信息
	Pagination *Pagination `json:"pagination,omitempty"`   // 列表接口的分页信息
}

type ErrorInfo struct {
	Code    string `json:"code" example:"INVALID_PARAM"` // 便于程序识别的错误码
	Message string `json:"message" example:"参数无效"`       // 便于阅读的错误信息
}

type Pagination struct {
	Page  int `json:"page,omitempty" example:"1"`   // 当前页码
	Size  int `json:"size,omitempty" example:"20"`  // 每页记录数
	Total int `json:"total,omitempty" example:"42"` // 符合条件的记录总数
}

func OK(c *gin.Context, data ...any) {
	var body any
	if len(data) > 0 {
		body = data[0]
	}
	c.JSON(http.StatusOK, Response{
		Success: true,
		Data:    body,
	})
}

func Created(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, Response{
		Success: true,
		Data:    data,
	})
}

func OKWithPagination(c *gin.Context, data any, pagination Pagination) {
	c.JSON(http.StatusOK, Response{
		Success:    true,
		Data:       data,
		Pagination: &pagination,
	})
}

func Fail(c *gin.Context, status int, code, message string) {
	c.JSON(status, Response{
		Success: false,
		Error:   &ErrorInfo{Code: code, Message: message},
	})
}
