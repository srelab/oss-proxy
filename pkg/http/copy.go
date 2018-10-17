package http

import (
	"net/http"
	"path/filepath"
	"strings"

	"github.com/denverdino/aliyungo/oss"
	"github.com/srelab/ossproxy/pkg/g"

	"github.com/labstack/echo"
)

type CopyHandler struct{}
type CopyPayLoad struct {
	Bucket string `json:"bucket"`
	Paths  []struct {
		Src string `json:"src"`
		Dst string `json:"dst"`
	} `json:"paths"`
}

type CopyResult struct {
	Successes []map[string]string `json:"successes"`
	Errors    []map[string]string `json:"errors"`
}

func (handler CopyHandler) Init(g *echo.Group) {
	g.PUT("/*", handler.Put)
}

func (CopyHandler) Put(ctx echo.Context) error {
	result := CopyResult{}
	payload := CopyPayLoad{}

	if err := ctx.Bind(&payload); err != nil {
		return FailureResponse(ctx, http.StatusBadRequest, ApiErrorParameter, err)
	}

	prefix := ctx.Param("*")
	if prefix == "" {
		prefix = "/"
	}

	client := oss.NewOSSClient(
		"oss-cn-shenzhen",
		false,
		g.Config().Ak.ID,
		g.Config().Ak.Secret,
		false,
	)

	bucket := client.Bucket("welab-ftp")

	for _, path := range payload.Paths {
		if _, err := bucket.PutCopy(
			filepath.Join("contract", prefix, path.Dst),
			oss.Private, oss.CopyOptions{},
			filepath.Join("/", payload.Bucket, path.Src),
		); err != nil {
			errmsg := strings.Replace(err.Error(), "Aliyun API Error:", "", 1)
			result.Errors = append(result.Errors, map[string]string{"path": path.Src, "msg": errmsg})

			continue
		}

		result.Successes = append(result.Successes, map[string]string{"path": path.Src})
	}

	return SuccessResponse(ctx, http.StatusOK, &BaseResult{
		Result:  result,
		Success: true,
	})
}
