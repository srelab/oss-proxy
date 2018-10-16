package main

import (
	"fmt"
	"os"
	"time"

	"github.com/urfave/cli"

	"github.com/srelab/ossproxy/pkg/g"
	"github.com/srelab/ossproxy/pkg/http"
	"github.com/srelab/ossproxy/pkg/logger"
	"github.com/srelab/ossproxy/pkg/sftp"
	"github.com/srelab/ossproxy/pkg/util"
)

func main() {
	app := &cli.App{
		Name:     g.NAME,
		Usage:    "Aliyun OSS Service's SFTP / HTTP proxy",
		Version:  g.VERSION,
		Compiled: time.Now(),
		Authors:  []cli.Author{{Name: g.AUTHOR, Email: g.MAIL}},
		Before: func(c *cli.Context) error {
			fmt.Fprintf(c.App.Writer, util.StripIndent(
				`
				 ####   ####   ####     #####  #####   ####  #    # #   #
				#    # #      #         #    # #    # #    #  #  #   # #  
				#    #  ####   ####     #    # #    # #    #   ##     #   
				#    #      #      #    #####  #####  #    #   ##     #   
				#    # #    # #    #    #      #   #  #    #  #  #    #   
				 ####   ####   ####     #      #    #  ####  #    #   #

			`))
			return nil
		},
		Commands: []cli.Command{
			{
				Name:  "start",
				Usage: "start a new oss-proxy",
				Action: func(ctx *cli.Context) error {
					for _, flagName := range ctx.FlagNames() {
						if ctx.String(flagName) != "" {
							continue
						}

						fmt.Println(flagName + " is required")
						os.Exit(127)
					}

					g.ParseConfig(ctx)
					logger.InitLogger()
					sftp.InitFileSystem()

					go sftp.Start()
					go http.Start()

					select {}
				},
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "sftp.keypath", Value: "./id_rsa", Usage: "sftp private key file path"},
					&cli.StringFlag{Name: "sftp.host", Value: "0.0.0.0", Usage: "sftp server host"},
					&cli.StringFlag{Name: "sftp.port", Value: "2022", Usage: "sftp server port"},
					&cli.StringFlag{Name: "http.host", Value: "0.0.0.0", Usage: "http server host"},
					&cli.StringFlag{Name: "http.port", Value: "8088", Usage: "http server port"},
					&cli.StringFlag{Name: "http.debug", Value: "0", Usage: "http server debug"},
					&cli.StringFlag{Name: "ak.id", Value: "0", Usage: "aliyun access key id", EnvVar: "AK_ID"},
					&cli.StringFlag{Name: "ak.secret", Value: "0", Usage: "aliyun access key secret", EnvVar: "AK_SECRET"},
					&cli.StringFlag{Name: "privilege.host", Usage: "privilege server host"},
					&cli.StringFlag{Name: "privilege.port", Usage: "privilege server port"},
					&cli.StringFlag{Name: "log.dir", Value: "./", Usage: "the log file is written to the path"},
					&cli.StringFlag{Name: "log.level", Value: "info", Usage: "valid levels: [debug, info, warn, error, fatal]"},
				},
			},
		},
	}

	app.Run(os.Args)
}
