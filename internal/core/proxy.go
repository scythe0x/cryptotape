package core

import (
	"errors"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

type ProxyProfile struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Protocol string `json:"protocol"`
	Login    string `json:"login"`
	Password string `json:"password"`
}

var validProxyHost = regexp.MustCompile(`(?i)^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)*\.?$`)

func (p ProxyProfile) URL() (string, error) {
	if p.Host == "" || len(p.Host) > 253 || (net.ParseIP(p.Host) == nil && !validProxyHost.MatchString(p.Host)) ||
		p.Port < 1 || p.Port > 65535 || (p.Protocol != "http" && p.Protocol != "https" && p.Protocol != "socks5") ||
		(p.Login == "" && p.Password != "") {
		return "", errors.New("invalid proxy profile")
	}
	endpoint := url.URL{Scheme: p.Protocol, Host: net.JoinHostPort(p.Host, strconv.Itoa(p.Port))}
	if p.Login != "" {
		endpoint.User = url.UserPassword(p.Login, p.Password)
	}
	return endpoint.String(), nil
}

func (s Settings) ProxyURL() (string, error) {
	if s.ActiveProxyID == "" {
		return "", nil
	}
	for _, profile := range s.Proxies {
		if profile.ID == s.ActiveProxyID {
			return profile.URL()
		}
	}
	return "", errors.New("unknown proxy profile")
}

func ValidateProxies(s Settings) error {
	ids := make(map[string]bool, len(s.Proxies))
	for _, p := range s.Proxies {
		if p.ID == "" || len(p.ID) > 100 || ids[p.ID] || strings.TrimSpace(p.Name) == "" || utf8.RuneCountInString(p.Name) > 60 {
			return errors.New("invalid proxy profile")
		}
		if _, err := p.URL(); err != nil {
			return err
		}
		ids[p.ID] = true
	}
	_, err := s.ProxyURL()
	return err
}
