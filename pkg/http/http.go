package http

import (
	"fmt"
	"log"

	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
	"github.com/srelab/ossproxy/pkg/g"
	"github.com/srelab/ossproxy/pkg/logger"
)

func Start() {
	e := echo.New()
	e.Use(middleware.CORS())
	e.Use(middleware.Recover())

	e.Use(middleware.LoggerWithConfig(middleware.LoggerConfig{
		Skipper: middleware.DefaultSkipper,
		Format:  middleware.DefaultLoggerConfig.Format,
		Output:  logger.GetLogWriter("access.log"),
	}))

	e.HideBanner = true
	e.Debug = g.Config().Http.Debug

	PublicHandler{}.Init(e.Group("/api/v1"))
	SftpHandler{}.Init(e.Group("/api/v1/sftp"))
	ShareHandler{}.Init(e.Group("/api/v1/share"))

	address := fmt.Sprintf("%s:%s", g.Config().Http.Host, g.Config().Http.Port)
	if err := e.Start(address); err != nil {
		log.Println(err)
	}
}
