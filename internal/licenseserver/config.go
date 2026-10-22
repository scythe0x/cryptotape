package licenseserver

import (
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"os"
)

type Config struct {
	Database           string
	Signing            ed25519.PrivateKey
	Encryption, Lookup []byte
	BotToken           string
}

func LoadConfig() (Config, error) {
	var c Config
	c.Database = os.Getenv("MYTHRA_DATABASE")
	if c.Database == "" {
		return c, errors.New("MYTHRA_DATABASE is required")
	}
	fields := []struct {
		name   string
		target *[]byte
	}{{"MYTHRA_SIGNING_SEED", nil}, {"MYTHRA_ENCRYPTION_KEY", &c.Encryption}, {"MYTHRA_LOOKUP_KEY", &c.Lookup}}
	for _, f := range fields {
		b, err := base64.StdEncoding.DecodeString(os.Getenv(f.name))
		if err != nil || len(b) != 32 {
			return c, errors.New(f.name + " must be a base64 encoded 32 byte key")
		}
		if f.target == nil {
			c.Signing = ed25519.NewKeyFromSeed(b)
		} else {
			*f.target = b
		}
	}
	c.BotToken = os.Getenv("MYTHRA_BOT_TOKEN")
	return c, nil
}
