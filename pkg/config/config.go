package config

import (
	"log"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/caarlos0/env/v11"
)

type cfg struct {
	PostgresHost      string `env:"POSTGRES_HOST,required"`
	PostgresUser      string `env:"POSTGRES_USER,required"`
	PostgresPass      string `env:"POSTGRES_PASS,required"`
	PostgresPort      uint32 `env:"POSTGRES_PORT" envDefault:"5432"`
	PostgresUriArgs   string `env:"POSTGRES_URI_ARGS,required"`
	PostgresDefaultDb string `env:"POSTGRES_DEFAULT_DATABASE"`
	CloudProvider     string `env:"POSTGRES_CLOUD_PROVIDER"`
	AnnotationFilter  string `env:"POSTGRES_INSTANCE"`
	KeepSecretName    bool   `env:"KEEP_SECRET_NAME"`
}

var doOnce sync.Once
var config *cfg

func Get() *cfg {
	doOnce.Do(func() {
		config = &cfg{
			PostgresPort: 5432,
		}
		if err := env.Parse(config); err != nil {
			log.Fatal(err)
		}
		config.PostgresUser = url.PathEscape(config.PostgresUser)
		config.PostgresPass = url.PathEscape(config.PostgresPass)
		if strings.Contains(config.PostgresHost, ":") {
			parts := strings.Split(config.PostgresHost, ":")
			if len(parts) > 1 {
				port, err := strconv.ParseInt(parts[1], 10, 32)
				if err != nil {
					log.Fatal(err)
				}
				config.PostgresPort = uint32(port)
				config.PostgresHost = parts[0]
			}
		}
	})
	return config
}
