//go:build !windows

package licensing

import (
	"errors"
	"time"
)

var uptimeStart = time.Now()

func Uptime() time.Duration       { return time.Since(uptimeStart) }
func Hardware() ([]string, error) { return nil, errors.New("licensing requires windows") }

type FileKeys struct{ Path string }

func (FileKeys) Load() (string, error) { return "", errors.New("licensing requires windows") }
func (FileKeys) Save(string) error     { return errors.New("licensing requires windows") }
