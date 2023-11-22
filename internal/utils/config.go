package utils

import (
	"net/url"
	"os"
	"strconv"
	"sync"
)

type Cfg struct {
	PostgresHost      string
	PostgresUser      string
	PostgresPass      string
	PostgresUriArgs   string
	PostgresDefaultDb string
	CloudProvider     string
	AnnotationFilter  string
	ObjectPrefix      string
	KeepSecretName    bool
}

var doOnce sync.Once
var config *Cfg

func GetConfig() *Cfg {
	doOnce.Do(func() {
		config = &Cfg{
			PostgresHost:      MustGetEnv("POSTGRES_HOST"),
			PostgresUser:      url.PathEscape(MustGetEnv("POSTGRES_USER")),
			PostgresPass:      url.PathEscape(MustGetEnv("POSTGRES_PASS")),
			PostgresUriArgs:   os.Getenv("POSTGRES_URI_ARGS"),
			PostgresDefaultDb: os.Getenv("POSTGRES_DEFAULT_DATABASE"),
			CloudProvider:     os.Getenv("POSTGRES_CLOUD_PROVIDER"),
			AnnotationFilter:  os.Getenv("POSTGRES_INSTANCE"),
			ObjectPrefix:      "",
		}
	})
	if value, err := strconv.ParseBool(os.Getenv("KEEP_SECRET_NAME")); err == nil {
		config.KeepSecretName = value
	}
	return config
}
