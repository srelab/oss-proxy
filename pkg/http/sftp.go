package http

import (
	"net/http"

	"time"

	"strconv"

	"github.com/labstack/echo"
	"github.com/srelab/ossproxy/pkg/sftp"
)

type SftpHandler struct{}

func (handler SftpHandler) Init(g *echo.Group) {
	g.GET("/*", handler.Get)
}

func (SftpHandler) Get(ctx echo.Context) error {
	prefix := ctx.Param("*")
	share := ctx.QueryParam("share")
	expire, err := strconv.Atoi(ctx.QueryParam("expire"))

	if err != nil {
		expire = 10
	}

	if prefix == "" {
		prefix = "/"
	}

	files, err := sftp.FileSystem.FetchFiles(prefix)
	if err != nil {
		return FailureResponse(ctx, http.StatusInternalServerError, BaseError{
			Code:    10010,
			Message: "sftp internal error",
		}, nil)
	}

	if share != "" {
		for fp := range files {
			if files[fp].Isdir {
				files[fp].URL = "-"
				continue
			}

			files[fp].URL = sftp.Bucket.SignedURL(
				files[fp].OssPath(fp), time.Now().Add(time.Duration(expire)*time.Minute),
			)
		}
	}

	return SuccessResponse(ctx, http.StatusOK, &BaseResult{
		Result:  files,
		Success: true,
	})
}
