package http

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"strconv"

	"github.com/denverdino/aliyungo/oss"
	"github.com/labstack/echo"
	"github.com/mholt/archiver"
	"github.com/srelab/ossproxy/pkg/sftp"
)

type SftpHandler struct{}

func (handler SftpHandler) Init(g *echo.Group) {
	g.GET("/*", handler.Get)
	g.DELETE("/*", handler.Delete)

	g.POST("/files/__archive", handler.Archive)
}

func (SftpHandler) Get(ctx echo.Context) error {
	prefix := ctx.Param("*")
	share := ctx.QueryParam("share")

	expire, err := strconv.Atoi(ctx.QueryParam("expire"))
	if err != nil {
		expire = 10
	}

	recursive, err := strconv.ParseBool(ctx.QueryParam("recursive"))
	if err != nil {
		recursive = false
	}

	if prefix == "" {
		prefix = "/"
	}

	files, err := sftp.FileSystem.FetchFiles(prefix, recursive)
	if err != nil {
		return FailureResponse(ctx, http.StatusInternalServerError, BaseError{
			Code:    10010,
			Message: "sftp internal error",
		}, err)
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

func (SftpHandler) Archive(ctx echo.Context) error {
	prefixes := make([]string, 0)
	archivePaths := make([]string, 0)
	archiveName := fmt.Sprintf("archive-%d.zip", int32(time.Now().Unix()))

	remoteArchiveRoot := "archives"
	localArchiveRoot := "/tmp/archives"
	if _, err := os.Stat(localArchiveRoot); !os.IsNotExist(err) {
		os.RemoveAll(localArchiveRoot)
	}

	if err := ctx.Bind(&prefixes); err != nil {
		return FailureResponse(ctx, http.StatusBadRequest, ApiErrorParameter, err)
	}

	for _, prefix := range prefixes {
		files, err := sftp.FileSystem.FetchFiles(prefix, true)
		if err != nil {
			return FailureResponse(ctx, http.StatusInternalServerError, BaseError{
				Code:    10010,
				Message: "sftp internal error",
			}, err)
		}

		for fp, file := range files {
			if file.Isdir {
				continue
			}

			fprefix, fname := filepath.Split(fp)
			tmpfp := filepath.Join(filepath.Join(localArchiveRoot, fprefix, fname))

			_ = os.MkdirAll(filepath.Join(localArchiveRoot, fprefix), 0755)
			if fd, err := sftp.Bucket.Get(fp); err == nil {
				if err := ioutil.WriteFile(tmpfp, fd, 0644); err != nil {
					fmt.Println(err)
					continue
				}
			} else {
				continue
			}
		}

		if _, err := os.Stat(filepath.Join(localArchiveRoot, prefix)); !os.IsNotExist(err) {
			archivePaths = append(archivePaths, filepath.Join(localArchiveRoot, prefix))
		}
	}

	if err := archiver.Zip.Make(filepath.Join(localArchiveRoot, archiveName), archivePaths); err != nil {
		return FailureResponse(ctx, http.StatusInternalServerError, BaseError{
			Code:    10012,
			Message: err.Error(),
		}, err)
	}

	archiveData, err := ioutil.ReadFile(filepath.Join(localArchiveRoot, archiveName))
	if err != nil {
		return FailureResponse(ctx, http.StatusInternalServerError, BaseError{
			Code:    10012,
			Message: "unable to read archive file",
		}, err)
	}

	if err := sftp.Bucket.Put(
		filepath.Join(remoteArchiveRoot, archiveName), archiveData,
		"content-type", oss.Private, oss.Options{},
	); err != nil {
		return FailureResponse(ctx, http.StatusInternalServerError, BaseError{
			Code:    10012,
			Message: "unable to write archive file",
		}, err)
	}

	return SuccessResponse(ctx, http.StatusOK, &BaseResult{
		Result: sftp.Bucket.SignedURL(
			filepath.Join(remoteArchiveRoot, archiveName), time.Now().Add(time.Duration(120)*time.Minute),
		),
		Success: true,
	})
}
