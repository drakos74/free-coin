package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
)

const path = "infra/config"

// MustLoad loads the config for the given key
func MustLoad(key string, v interface{}) []byte {

	b, err := ioutil.ReadFile(fmt.Sprintf("%s/%s.json", path, key))
	if err != nil {
		panic(fmt.Sprintf("could not load config for %s: %s", key, err.Error()))
	}

	err = json.Unmarshal(b, v)
	if err != nil {
		panic(fmt.Sprintf("could not unmarshal the config for %s: %s", key, err.Error()))
	}

	return b

}
