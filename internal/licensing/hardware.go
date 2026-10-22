package licensing

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
)

func fingerprint(kind, value string) string {
	sum := sha256.Sum256([]byte("mythra-hardware-v1:" + kind + ":" + value))
	return hex.EncodeToString(sum[:])
}
func usableSerial(s string) bool {
	s = strings.ToUpper(strings.TrimSpace(s))
	if len(s) < 4 || len(s) > 128 {
		return false
	}
	for _, bad := range []string{"TO BE FILLED", "DEFAULT", "UNKNOWN", "NOT SPECIFIED", "NONE", "SYSTEM SERIAL", "BASE BOARD SERIAL", "INVALID", "OEM", "N/A"} {
		if strings.Contains(s, bad) {
			return false
		}
	}
	compact := strings.ReplaceAll(strings.ReplaceAll(s, "-", ""), " ", "")
	if compact == "" {
		return false
	}
	for _, sequence := range []string{"01234567890123456789", "98765432109876543210", "ABCDEFGHIJKLMNOPQRSTUVWXYZ"} {
		if strings.Contains(sequence, compact) {
			return false
		}
	}
	return strings.Trim(compact, string(compact[0])) != ""
}

// hardwareFromSMBIOS accepts raw structure data, without the windows firmware header.
func hardwareFromSMBIOS(data []byte) ([]string, error) {
	var uuid, serial, manufacturer string
	for offset := 0; offset+4 <= len(data); {
		typ, length := data[offset], int(data[offset+1])
		if length < 4 || offset+length > len(data) {
			return nil, errors.New("invalid smbios")
		}
		formatted := data[offset : offset+length]
		end := offset + length
		for end+1 < len(data) && (data[end] != 0 || data[end+1] != 0) {
			end++
		}
		if end+1 >= len(data) {
			return nil, errors.New("invalid smbios strings")
		}
		stringsTable := strings.Split(string(data[offset+length:end]), "\x00")
		value := func(index byte) string {
			if index == 0 || int(index) > len(stringsTable) {
				return ""
			}
			return strings.ToUpper(strings.TrimSpace(stringsTable[index-1]))
		}
		if typ == 1 && length >= 24 {
			raw := formatted[8:24]
			allZero, allFF := true, true
			for _, v := range raw {
				allZero = allZero && v == 0
				allFF = allFF && v == 255
			}
			if !allZero && !allFF {
				uuid = hex.EncodeToString(raw)
			}
		}
		if typ == 2 && length >= 8 {
			manufacturer = value(formatted[4])
			serial = value(formatted[7])
		}
		offset = end + 2
		if typ == 127 {
			break
		}
	}
	var ids []string
	if usableSerial(uuid) {
		ids = append(ids, fingerprint("uuid", uuid))
	}
	if usableSerial(serial) {
		ids = append(ids, fingerprint("board", manufacturer+":"+serial))
	}
	if len(ids) == 0 {
		return nil, errors.New("hardware unavailable")
	}
	return ids, nil
}
