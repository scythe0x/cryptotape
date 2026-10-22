//go:build windows

package licensing

import (
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var kernel = windows.NewLazySystemDLL("kernel32.dll")
var firmware = kernel.NewProc("GetSystemFirmwareTable")
var ticks = kernel.NewProc("GetTickCount64")

func Uptime() time.Duration { n, _, _ := ticks.Call(); return time.Duration(n) * time.Millisecond }
func Hardware() ([]string, error) {
	const provider = 0x52534d42
	size, _, _ := firmware.Call(provider, 0, 0, 0)
	if size < 8 || size > 1024*1024 {
		return nil, errors.New("hardware unavailable")
	}
	data := make([]byte, int(size))
	n, _, _ := firmware.Call(provider, 0, uintptr(unsafe.Pointer(&data[0])), size)
	if n != size || int(binary.LittleEndian.Uint32(data[4:8])) != len(data)-8 {
		return nil, errors.New("hardware unavailable")
	}
	return hardwareFromSMBIOS(data[8:])
}

type FileKeys struct{ Path string }

func (f FileKeys) Load() (string, error) {
	data, err := os.ReadFile(f.Path)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if len(data) == 0 || len(data) > 16384 {
		return "", errors.New("invalid key storage")
	}
	input := windows.DataBlob{Size: uint32(len(data)), Data: &data[0]}
	var output windows.DataBlob
	if err = windows.CryptUnprotectData(&input, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &output); err != nil {
		return "", errors.New("cannot read key storage")
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(output.Data)))
	return string(unsafe.Slice(output.Data, output.Size)), nil
}
func (f FileKeys) Save(key string) error {
	data := []byte(NormalizeKey(key))
	if len(data) != 32 {
		return errors.New("invalid key")
	}
	input := windows.DataBlob{Size: uint32(len(data)), Data: &data[0]}
	var output windows.DataBlob
	if err := windows.CryptProtectData(&input, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &output); err != nil {
		return errors.New("cannot protect key")
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(output.Data)))
	if err := os.MkdirAll(filepath.Dir(f.Path), 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(f.Path), ".license-*")
	if err != nil {
		return err
	}
	name := file.Name()
	defer os.Remove(name)
	if _, err = file.Write(unsafe.Slice(output.Data, output.Size)); err != nil {
		file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	from, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return err
	}
	to, err := windows.UTF16PtrFromString(f.Path)
	if err != nil {
		return err
	}
	return windows.MoveFileEx(from, to, windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH)
}
