package utils

import (
	"fmt"
	"os"
	"strings"
)

func EnvWithDefault(key string, defValue string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return defValue
	}
	return value
}

func Env(key string) (string, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return "", fmt.Errorf("can't find `%s` environment variable", key)
	}
	return value, nil
}
