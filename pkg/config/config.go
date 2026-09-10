package config

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/movetokube/postgres-operator/pkg/utils"
)

type Cfg struct {
	PostgresHost       string
	PostgresUser       string
	PostgresPass       string
	PostgresUriArgs    string
	PostgresPassPolicy utils.PostgresPassPolicy
	PostgresDefaultDb  string
	CloudProvider      CloudProvider
	AnnotationFilter   string
	KeepSecretName     bool
}

var (
	doOnce sync.Once
	config *Cfg
)

type CloudProvider string

const (
	CloudProviderNone  CloudProvider = "None"
	CloudProviderAWS   CloudProvider = "AWS"
	CloudProviderAzure CloudProvider = "Azure"
	CloudProviderGCP   CloudProvider = "GCP"
)

func Get() *Cfg {
	doOnce.Do(func() {
		config = &Cfg{}
		config.PostgresHost = utils.MustGetEnv("POSTGRES_HOST")
		config.PostgresUser = url.PathEscape(utils.MustGetEnv("POSTGRES_USER"))
		config.PostgresPass = url.PathEscape(utils.MustGetEnv("POSTGRES_PASS"))
		config.PostgresUriArgs = utils.MustGetEnv("POSTGRES_URI_ARGS")
		config.PostgresDefaultDb = utils.GetEnv("POSTGRES_DEFAULT_DATABASE")
		config.CloudProvider = ParseCloudProvider(utils.GetEnv("POSTGRES_CLOUD_PROVIDER"))
		config.AnnotationFilter = utils.GetEnv("POSTGRES_INSTANCE")
		if value, err := strconv.ParseBool(utils.GetEnv("KEEP_SECRET_NAME")); err == nil {
			config.KeepSecretName = value
		}

		pp, err := loadPassPolicy()
		if err != nil {
			panic(fmt.Errorf("failed to load password policy config: %w", err))
		}
		config.PostgresPassPolicy = pp
	})
	return config
}

// CloudProvider is an enum for supported cloud providers.

func ParseCloudProvider(s string) CloudProvider {
	switch strings.ToLower(s) {
	case "aws":
		return CloudProviderAWS
	case "azure":
		return CloudProviderAzure
	case "gcp":
		return CloudProviderGCP
	default:
		return CloudProviderNone
	}
}

// loadPassPolicy parses password policy configuration from environment variables.
func loadPassPolicy() (utils.PostgresPassPolicy, error) {
	var pp utils.PostgresPassPolicy
	var err error

	if pp.Length, err = parseIntEnv("POSTGRES_DEFAULT_PASSWORD_LENGTH"); err != nil {
		return pp, err
	}
	if pp.MinLower, err = parseIntEnv("POSTGRES_DEFAULT_PASSWORD_MIN_LOWER"); err != nil {
		return pp, err
	}
	if pp.MinUpper, err = parseIntEnv("POSTGRES_DEFAULT_PASSWORD_MIN_UPPER"); err != nil {
		return pp, err
	}
	if pp.MinNumeric, err = parseIntEnv("POSTGRES_DEFAULT_PASSWORD_MIN_NUMERIC"); err != nil {
		return pp, err
	}
	if pp.MinSpecial, err = parseIntEnv("POSTGRES_DEFAULT_PASSWORD_MIN_SPECIAL"); err != nil {
		return pp, err
	}

	pp.ExcludeChars = utils.GetEnv("POSTGRES_DEFAULT_PASSWORD_EXCLUDE_CHARS")

	if pp.EnsureFirstLetter, err = parseBoolEnv("POSTGRES_DEFAULT_PASSWORD_ENSURE_FIRST_LETTER"); err != nil {
		return pp, err
	}

	return pp, nil
}

func parseIntEnv(key string) (int, error) {
	val := utils.GetEnv(key)
	if val == "" {
		return 0, nil
	}
	i, err := strconv.Atoi(val)
	if err != nil {
		return 0, fmt.Errorf("invalid integer for %s: %v", key, err)
	}
	return i, nil
}

func parseBoolEnv(key string) (bool, error) {
	val := utils.GetEnv(key)
	if val == "" {
		return false, nil
	}
	b, err := strconv.ParseBool(val)
	if err != nil {
		return false, fmt.Errorf("invalid boolean for %s: %v", key, err)
	}
	return b, nil
}
