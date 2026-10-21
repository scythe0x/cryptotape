package listings

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

const FreeEndpoint = "wss://ws.newlistings.pro/v2/full"

type Config struct {
	Endpoint string `json:"endpoint"`
	Key      string `json:"key"`
}

func ValidEndpoint(endpoint string) bool {
	return endpoint == FreeEndpoint
}

func LoadConfig(path string) (Config, error) {
	config := Config{Endpoint: FreeEndpoint}
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return config, nil
	}
	if err != nil {
		return config, errors.New("cannot read listing source config")
	}
	if json.Unmarshal(raw, &config) != nil || !ValidEndpoint(config.Endpoint) || !validKey(config.Key) {
		return Config{}, errors.New("invalid listing source config")
	}
	return config, nil
}

func SaveConfig(path string, config Config) error {
	config.Key = strings.TrimSpace(config.Key)
	if !ValidEndpoint(config.Endpoint) || !validKey(config.Key) {
		return errors.New("invalid listing source config")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return errors.New("cannot save listing source config")
	}
	raw, err := json.Marshal(config)
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".listings-*")
	if err != nil {
		return errors.New("cannot save listing source config")
	}
	defer os.Remove(file.Name())
	_, writeErr := file.Write(raw)
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil || os.Rename(file.Name(), path) != nil {
		return errors.New("cannot save listing source config")
	}
	return nil
}

func validKey(key string) bool {
	if len(key) > 256 {
		return false
	}
	for _, ch := range key {
		if ch <= ' ' || ch > '~' {
			return false
		}
	}
	return true
}
