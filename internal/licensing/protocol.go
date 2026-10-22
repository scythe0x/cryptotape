package licensing

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"time"
)

const Endpoint = "https://scalping.mythra.pw"
const BotURL = "https://t.me/mythrascalping_bot"
const CheckInterval = time.Hour
const RetryBudget = 70 * time.Second

// only the server public key belongs in the client.
const PublicKeyBase64 = "23+N8dWWWTevRyo1ItXa0W30q0B4xTsG7h0h4bCZ3Wk="

type Request struct {
	Key      string   `json:"key"`
	Hardware []string `json:"hardware"`
	Nonce    string   `json:"nonce"`
}

type Grant struct {
	RequestStarted time.Duration `json:"-"`
	Version        int           `json:"version"`
	Code           string        `json:"code"`
	Nonce          string        `json:"nonce"`
	Hardware       string        `json:"hardware"`
	LicenseID      int64         `json:"licenseId"`
	ServerTime     int64         `json:"serverTime"`
	ExpiresAt      int64         `json:"expiresAt"`
}

type Envelope struct {
	Payload   string `json:"payload"`
	Signature string `json:"signature"`
}

func NewNonce() string { return base64.RawURLEncoding.EncodeToString(randomBytes(32)) }

func randomBytes(size int) []byte {
	b := make([]byte, size)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return b
}

func NormalizeKey(key string) string {
	return strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(key), "-", ""))
}

func ValidRequest(r Request) bool {
	nonce, err := base64.RawURLEncoding.DecodeString(r.Nonce)
	if err != nil || len(nonce) != 32 || len(NormalizeKey(r.Key)) != 32 || len(r.Hardware) < 1 || len(r.Hardware) > 2 {
		return false
	}
	if _, err := hex.DecodeString(NormalizeKey(r.Key)); err != nil {
		return false
	}
	for _, h := range r.Hardware {
		b, err := hex.DecodeString(h)
		if err != nil || len(b) != 32 {
			return false
		}
	}
	return len(r.Hardware) == 1 || r.Hardware[0] != r.Hardware[1]
}

func HardwareDigest(ids []string) string {
	ids = slices.Clone(ids)
	slices.Sort(ids)
	sum := sha256.Sum256([]byte(strings.Join(ids, ":")))
	return hex.EncodeToString(sum[:])
}

func Sign(g Grant, key ed25519.PrivateKey) (Envelope, error) {
	if len(key) != ed25519.PrivateKeySize {
		return Envelope{}, errors.New("invalid signing key")
	}
	b, err := json.Marshal(g)
	if err != nil {
		return Envelope{}, err
	}
	return Envelope{base64.RawURLEncoding.EncodeToString(b), base64.RawURLEncoding.EncodeToString(ed25519.Sign(key, b))}, nil
}

func Verify(e Envelope, key ed25519.PublicKey, r Request) (Grant, error) {
	var g Grant
	b, err := base64.RawURLEncoding.DecodeString(e.Payload)
	if err != nil || len(b) > 4096 {
		return g, errors.New("invalid license signature")
	}
	sig, err := base64.RawURLEncoding.DecodeString(e.Signature)
	if err != nil || len(key) != ed25519.PublicKeySize || !ed25519.Verify(key, b, sig) {
		return g, errors.New("invalid license signature")
	}
	if err := json.Unmarshal(b, &g); err != nil {
		return g, errors.New("invalid license response")
	}
	if g.Version != 1 || g.Nonce != r.Nonce || g.Hardware != HardwareDigest(r.Hardware) || g.ServerTime <= 0 {
		return Grant{}, errors.New("invalid license response")
	}
	if g.Code == "OK" && (g.LicenseID <= 0 || g.ExpiresAt <= g.ServerTime) {
		return Grant{}, errors.New("invalid license expiry")
	}
	return g, nil
}

func Message(code string) string {
	switch code {
	case "":
		return ""
	case "INVALID_KEY":
		return "Неверный ключ"
	case "EXPIRED":
		return "Срок ключа истёк"
	case "REVOKED":
		return "Ключ отозван"
	case "OTHER_PC":
		return "Ключ привязан к другому пк"
	case "PC_USED":
		return "Этот пк уже привязан к другому ключу"
	case "HARDWARE":
		return "Не удалось определить этот пк"
	case "STORAGE":
		return "Не удалось сохранить ключ"
	case "UNTRUSTED":
		return "Не удалось проверить ответ сервера"
	default:
		return "Сервер недоступен"
	}
}
