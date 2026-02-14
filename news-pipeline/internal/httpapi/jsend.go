package httpapi

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

type jsendResponse struct {
	Status  string `json:"status"`
	Data    any    `json:"data,omitempty"`
	Message string `json:"message,omitempty"`
	Code    int    `json:"code,omitempty"`
}

func success(c echo.Context, data any) error {
	return c.JSON(http.StatusOK, jsendResponse{
		Status: "success",
		Data:   data,
	})
}

func successWithStatus(c echo.Context, code int, data any) error {
	return c.JSON(code, jsendResponse{
		Status: "success",
		Data:   data,
	})
}

func fail(c echo.Context, code int, message string, data any) error {
	resp := jsendResponse{
		Status:  "fail",
		Message: message,
	}
	if data != nil {
		resp.Data = data
	}
	return c.JSON(code, resp)
}

func failValidation(c echo.Context, fieldErrors map[string]string) error {
	return fail(c, http.StatusBadRequest, "Validation failed", map[string]any{
		"validation_errors": fieldErrors,
	})
}

func failNotFound(c echo.Context, message string) error {
	return fail(c, http.StatusNotFound, message, nil)
}

func internalError(c echo.Context, message string) error {
	return c.JSON(http.StatusInternalServerError, jsendResponse{
		Status:  "error",
		Message: message,
		Code:    http.StatusInternalServerError,
	})
}
