package handler

import (
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
)

var fieldNameMap = map[string]string{
	"Username":  "用户名",
	"Password":  "密码",
	"Email":     "邮箱",
	"Phone":     "手机号",
	"Nickname":  "昵称",
	"Avatar":    "头像",
	"Status":    "状态",
	"LoginAt":   "登录时间",
	"ClientID":  "客户端ID",
	"ClientSecret": "客户端密钥",
	"Name":      "名称",
	"RedirectURIs": "重定向地址",
	"AllowedGrants": "授权类型",
	"AllowedScopes": "授权范围",
	"UserID":    "用户ID",
	"Code":      "验证码",
	"NewPassword": "新密码",
}

func formatValidationError(err error) string {
	if ve, ok := err.(validator.ValidationErrors); ok {
		var msgs []string
		for _, e := range ve {
			field := e.Field()
			if cn, ok := fieldNameMap[field]; ok {
				field = cn
			}
			msgs = append(msgs, formatFieldError(field, e))
		}
		return strings.Join(msgs, "；")
	}
	return err.Error()
}

func formatFieldError(field string, e validator.FieldError) string {
	switch e.Tag() {
	case "required":
		return fmt.Sprintf("%s 不能为空", field)
	case "min":
		if e.Kind() == 1 { // string
			return fmt.Sprintf("%s 长度不能少于 %s 个字符", field, e.Param())
		}
		return fmt.Sprintf("%s 不能小于 %s", field, e.Param())
	case "max":
		if e.Kind() == 1 { // string
			return fmt.Sprintf("%s 长度不能超过 %s 个字符", field, e.Param())
		}
		return fmt.Sprintf("%s 不能大于 %s", field, e.Param())
	case "email":
		return fmt.Sprintf("%s 格式不正确", field)
	case "gte":
		return fmt.Sprintf("%s 不能小于 %s", field, e.Param())
	case "lte":
		return fmt.Sprintf("%s 不能大于 %s", field, e.Param())
	case "gt":
		return fmt.Sprintf("%s 必须大于 %s", field, e.Param())
	case "lt":
		return fmt.Sprintf("%s 必须小于 %s", field, e.Param())
	case "oneof":
		return fmt.Sprintf("%s 必须是 %s 中的一个", field, e.Param())
	case "len":
		return fmt.Sprintf("%s 长度必须为 %s", field, e.Param())
	default:
		return fmt.Sprintf("%s 校验失败 (%s=%s)", field, e.Tag(), e.Param())
	}
}
