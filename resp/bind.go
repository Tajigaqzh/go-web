package resp

import (
	"errors"
	"net/http"
	"reflect"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
)

func Init() {
	engine, ok := binding.Validator.Engine().(*validator.Validate)
	if !ok {
		return
	}
	engine.RegisterTagNameFunc(func(fld reflect.StructField) string {
		for _, key := range []string{"json", "form", "uri", "header"} {
			name := strings.SplitN(fld.Tag.Get(key), ",", 2)[0]
			if name != "" && name != "-" {
				return name
			}
		}
		return fld.Name
	})
}

func BindJSON(c *gin.Context, dst any) error {
	if err := c.ShouldBindJSON(dst); err != nil {
		Fail(c, http.StatusBadRequest, CodeInvalidBody, validationMessage(err))
		return err
	}
	return nil
}

func BindXML(c *gin.Context, dst any) error {
	if err := c.ShouldBindXML(dst); err != nil {
		Fail(c, http.StatusBadRequest, CodeInvalidBody, validationMessage(err))
		return err
	}
	return nil
}

func BindBody(c *gin.Context, dst any) error {
	if err := c.ShouldBind(dst); err != nil {
		Fail(c, http.StatusBadRequest, CodeInvalidBody, validationMessage(err))
		return err
	}
	return nil
}

func BindQuery(c *gin.Context, dst any) error {
	if err := c.ShouldBindQuery(dst); err != nil {
		Fail(c, http.StatusBadRequest, CodeInvalidParam, validationMessage(err))
		return err
	}
	return nil
}

func BindUri(c *gin.Context, dst any) error {
	if err := c.ShouldBindUri(dst); err != nil {
		Fail(c, http.StatusBadRequest, CodeInvalidParam, validationMessage(err))
		return err
	}
	return nil
}

func BindHeader(c *gin.Context, dst any) error {
	if err := c.ShouldBindHeader(dst); err != nil {
		Fail(c, http.StatusBadRequest, CodeInvalidParam, validationMessage(err))
		return err
	}
	return nil
}

func validationMessage(err error) string {
	var ves validator.ValidationErrors
	if !errors.As(err, &ves) {
		return err.Error()
	}
	parts := make([]string, 0, len(ves))
	for _, fe := range ves {
		parts = append(parts, fieldMessage(fe))
	}
	return strings.Join(parts, "; ")
}

func fieldMessage(fe validator.FieldError) string {
	name := fe.Field()
	switch fe.Tag() {
	case "required":
		return name + " 不能为空"
	case "min", "gte":
		return name + " 不能小于 " + fe.Param()
	case "max", "lte":
		return name + " 不能大于 " + fe.Param()
	case "email":
		return name + " 不是有效邮箱"
	case "len":
		return name + " 长度必须是 " + fe.Param()
	case "oneof":
		return name + " 只能是: " + fe.Param()
	default:
		return name + " 校验失败: " + fe.Tag()
	}
}
