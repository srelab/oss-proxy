package g

import (
	"sync"

	"github.com/urfave/cli"
)

type LogConfig struct {
	Dir   string
	Level string
}

type SftpConfig struct {
	Keypath string
	Port    string
	Host    string
}

type HttpConfig struct {
	Port  string
	Host  string
	Debug bool
}

type PrivilegeConfig struct {
	Host string
	Port string
}

type AkConfig struct {
	ID     string
	Secret string
}

type GlobalConfig struct {
	Name    string
	Keypath string

	Http      *HttpConfig
	Sftp      *SftpConfig
	Log       *LogConfig
	Privilege *PrivilegeConfig
	Ak        *AkConfig
}

var (
	config *GlobalConfig
	lock   = new(sync.RWMutex)
)

func Config() *GlobalConfig {
	lock.RLock()
	defer lock.RUnlock()
	return config
}

func ParseConfig(ctx *cli.Context) {
	config = &GlobalConfig{
		Name: NAME,
		Sftp: &SftpConfig{
			Keypath: ctx.String("sftp.keypath"),
			Host:    ctx.String("sftp.host"),
			Port:    ctx.String("sftp.port"),
		},
		Http: &HttpConfig{
			Debug: ctx.Bool("http.debug"),
			Host:  ctx.String("http.host"),
			Port:  ctx.String("http.port"),
		},
		Log: &LogConfig{
			Dir:   ctx.String("log.dir"),
			Level: ctx.String("log.level"),
		},
		Privilege: &PrivilegeConfig{
			Host: ctx.String("privilege.host"),
			Port: ctx.String("privilege.port"),
		},
		Ak: &AkConfig{
			ID:     ctx.String("ak.id"),
			Secret: ctx.String("ak.secret"),
		},
	}
}
