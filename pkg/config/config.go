package config

import (
	"net/url"
	"strconv"
	"sync"

	"github.com/movetokube/postgres-operator/pkg/utils"
)

type cfg struct {
	PostgresHost      string
	PostgresUser      string
	PostgresPass      string
	PostgresUriArgs   string
	PostgresDefaultDb string
	CloudProvider     string
	AnnotationFilter  string
	KeepSecretName    bool
}

var doOnce sync.Once
var config *cfg

func Get() *cfg {
	doOnce.Do(func() {
		config = &cfg{}
		config.PostgresHost = utils.MustGetEnv("POSTGRES_HOST")
		config.PostgresUser = url.PathEscape(utils.MustGetEnv("POSTGRES_USER"))
		config.PostgresPass = url.PathEscape(utils.MustGetEnv("POSTGRES_PASS"))
		config.PostgresUriArgs = utils.MustGetEnv("POSTGRES_URI_ARGS")
		config.PostgresDefaultDb = utils.GetEnv("POSTGRES_DEFAULT_DATABASE")
		config.CloudProvider = utils.GetEnv("POSTGRES_CLOUD_PROVIDER")
		config.AnnotationFilter = utils.GetEnv("POSTGRES_INSTANCE")
		if value, err := strconv.ParseBool(utils.GetEnv("KEEP_SECRET_NAME")); err == nil {
			config.KeepSecretName = value
		}
	})
	return config
}
