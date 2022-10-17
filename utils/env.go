package utils

import (
	"log"
	"os"
)

func MustGetEnv(name string) string {
	value, found := os.LookupEnv(name)
	if !found {
		log.Fatalf("environment variable %s is missing", name)
	}
	return value
}

func GetEnv(name string) string {
	return os.Getenv(name)
}
