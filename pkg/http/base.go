package http

import (
	"fmt"

	"github.com/labstack/echo"
)

type BaseResult struct {
	Result     interface{} `json:"result"`
	Success    bool        `json:"success"`
	Error      BaseError   `json:"error,omitempty"`
	Pagination interface{} `json:"pagination,omitempty"`
}

type BaseError struct {
	Code    int         `json:"code,omitempty"`
	Message string      `json:"msg,omitempty"`
	Details interface{} `json:"details,omitempty"`
}

var ApiErrorParameter = BaseError{Code: 10008, Message: "Parameter error"}

func FailureResponse(ctx echo.Context, status int, baseError BaseError, err error, v ...interface{}) error {
	str := ""
	if err != nil {
		str = err.Error()
	}
	return ctx.JSON(status, BaseResult{
		Success: false,
		Error: BaseError{
			Code:    baseError.Code,
			Message: fmt.Sprintf(baseError.Message, v...),
			Details: str,
		},
	})
}

func SuccessResponse(ctx echo.Context, status int, result *BaseResult) error {
	if result == nil {
		result = new(BaseResult)
	}

	if result.Result == nil || result.Result == "" {
		result.Result = "request success"
	}

	result.Success = true
	return ctx.JSON(status, result)
}
