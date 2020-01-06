package config

import (
	"github.com/movetokube/postgres-operator/pkg/utils"
	"sync"
)

type cfg struct {
	PostgresHost      string
	PostgresUser      string
	PostgresPass      string
	PostgresUriArgs   string
	PostgresDefaultDb string
	CloudProvider     string
}

var doOnce sync.Once
var config *cfg

func Get() *cfg {
	doOnce.Do(func() {
		config = &cfg{}
		config.PostgresHost = utils.MustGetEnv("POSTGRES_HOST")
		config.PostgresUser = utils.MustGetEnv("POSTGRES_USER")
		config.PostgresPass = utils.MustGetEnv("POSTGRES_PASS")
		config.PostgresUriArgs = utils.MustGetEnv("POSTGRES_URI_ARGS")
		config.PostgresDefaultDb = utils.GetEnv("POSTGRES_DEFAULT_DATABASE")
		config.CloudProvider = utils.GetEnv("POSTGRES_CLOUD_PROVIDER")
	})
	return config
}
