package utils

import (
	"encoding/json"
	"fmt"
	"strings"
)

type VcapServices struct {
	UserProvided UserProvided `json:"user_provided"`
}

type UserProvided struct {
	Credentials map[string]string `json:"credentials"`
}

func VCAP(key string) (string, error) {
	v, err := Env("VCAP_SERVICES")
	if err != nil {
		return "", err
	}

	data := VcapServices{}
	err = json.Unmarshal([]byte(v), &data)
	if err != nil {
		return "", err
	}

	if keyValue, keyExist := data.UserProvided.Credentials[key]; keyExist {
		return strings.TrimSpace(keyValue), nil
	}
	if keyValue, keyExist := data.UserProvided.Credentials[strings.ToLower(key)]; keyExist {
		return strings.TrimSpace(keyValue), nil
	}

	return "", fmt.Errorf("can't find `%s` environment variable", key)
}
