package http

import (
	"net/http"

	"time"

	"strconv"

	"github.com/denverdino/aliyungo/oss"
	"github.com/labstack/echo"
	"github.com/srelab/ossproxy/pkg/sftp"
)

type SftpHandler struct{}

func (handler SftpHandler) Init(g *echo.Group) {
	g.GET("/*", handler.Get)
	g.DELETE("/*", handler.Delete)
}

func (SftpHandler) Get(ctx echo.Context) error {
	prefix := ctx.Param("*")
	share := ctx.QueryParam("share")
	expire, err := strconv.Atoi(ctx.QueryParam("expire"))
	recursive, err := strconv.ParseBool(ctx.QueryParam("recursive"))
	if err != nil {
		recursive = false
	}

	if err != nil {
		expire = 10
	}

	if prefix == "" {
		prefix = "/"
	}

	files, err := sftp.FileSystem.FetchFiles(prefix, recursive)
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

func (SftpHandler) Delete(ctx echo.Context) error {
	prefix := ctx.Param("*")
	recursive, err := strconv.ParseBool(ctx.QueryParam("recursive"))
	if err != nil {
		recursive = false
	}

	files, err := sftp.FileSystem.FetchFiles(prefix, recursive)
	foList := make([]oss.Object, 0) // need delete file object list
	doList := make([]oss.Object, 0) // need delete directory object list
	for fp, file := range files {
		key := file.OssPath(fp)

		if file.Isdir {
			doList = append(doList, oss.Object{Key: key})
		} else {
			foList = append(foList, oss.Object{Key: key})
		}
	}

	if len(foList) > 0 {
		if err := sftp.Bucket.DelMulti(oss.Delete{Quiet: true, Objects: foList}); err != nil {
			return FailureResponse(ctx, http.StatusInternalServerError, BaseError{
				Code:    10011,
				Message: "sftp internal error",
			}, err)
		}
	}

	if len(doList) > 0 {
		if err := sftp.Bucket.DelMulti(oss.Delete{Quiet: true, Objects: doList}); err != nil {
			return FailureResponse(ctx, http.StatusInternalServerError, BaseError{
				Code:    10011,
				Message: "sftp internal error",
			}, err)
		}
	}

	return SuccessResponse(ctx, http.StatusOK, &BaseResult{
		Success: true,
	})
}
