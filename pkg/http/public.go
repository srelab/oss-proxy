package http

import (
	"net/http"

	"github.com/labstack/echo"
)

type PublicHandler struct{}

func (handler PublicHandler) Init(g *echo.Group) {
	g.GET("/", handler.Get)
}

func (PublicHandler) Get(ctx echo.Context) error {
	return SuccessResponse(ctx, http.StatusOK, nil)
}
