package utils

import (
	"encoding/json"
	"fmt"
	"strings"
)

type VcapServices struct {
	UserProvided []UserProvided `json:"user_provided"`
}

type UserProvided struct {
	Credentials map[string]interface{} `json:"credentials"`
}

func VCAP(key string) (string, error) {
	v, err := Env("VCAP_SERVICES")
	if err != nil {
		return "", err
	}

	data := &VcapServices{}
	err = json.Unmarshal([]byte(v), data)
	if err != nil {
		return "", err
	}

	for _, up := range data.UserProvided {
		if keyValue, keyExist := up.Credentials[key]; keyExist {
			return keyValue.(string), nil
		}
		if keyValue, keyExist := up.Credentials[strings.ToLower(key)]; keyExist {
			return keyValue.(string), nil
		}
	}

	return "", fmt.Errorf("VCAP can't find `%s` or `%s` variable", key, strings.ToLower(key))
}
