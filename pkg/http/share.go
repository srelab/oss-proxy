package http

import (
	"net/http"

	"time"

	"strconv"

	"github.com/labstack/echo"
	"github.com/srelab/ossproxy/pkg/sftp"
)

type ShareHandler struct{}

func (handler ShareHandler) Init(g *echo.Group) {
	g.GET("/*", handler.Get)
}

func (ShareHandler) Get(ctx echo.Context) error {
	prefix := ctx.Param("*")
	expire, err := strconv.Atoi(ctx.QueryParam("expire"))

	if err != nil || expire == 0 {
		expire = 20
	}

	if prefix == "" {
		prefix = "/"
	}

	return SuccessResponse(ctx, http.StatusOK, &BaseResult{
		Result:  sftp.Bucket.SignedURL(prefix, time.Now().Add(time.Duration(expire)*time.Minute)),
		Success: true,
	})
}
