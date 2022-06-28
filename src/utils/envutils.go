package utils

import (
	"fmt"
	"os"
	"strings"
)

func VcapOrEnvWithDefault(key string, defValue string) string {
	value, err := VCAP(key)
	if err != nil {
		return EnvWithDefault(key, defValue)
	}
	return value
}

func EnvWithDefault(key string, defValue string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return defValue
	}
	return value
}

func VcapOrEnv(key string) (string, error) {
	value, _ := VCAP(key)
	if value == "" {
		return Env(key)
	}
	return value, nil
}
func Env(key string) (string, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return "", fmt.Errorf("can't find `%s` environment variable", key)
	}
	return value, nil
}
