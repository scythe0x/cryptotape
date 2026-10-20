package core

import (
	"net/url"
	"testing"
)

func TestProxyURLsSupportDomainsAuthenticationAndSOCKS(t *testing.T) {
	for _, protocol := range []string{"http", "https", "socks5"} {
		profile := ProxyProfile{Host: "proxy.example", Port: 443, Protocol: protocol, Login: "test@user", Password: "test:/@secret"}
		address, err := profile.URL()
		if err != nil {
			t.Fatal(err)
		}
		parsed, err := url.Parse(address)
		if err != nil {
			t.Fatal(err)
		}
		password, ok := parsed.User.Password()
		if parsed.Scheme != protocol || parsed.Host != "proxy.example:443" || parsed.User.Username() != profile.Login || !ok || password != profile.Password {
			t.Fatal("proxy credentials or scheme were not encoded correctly")
		}
	}
	profile := ProxyProfile{Host: "::1", Port: 1080, Protocol: "socks5"}
	if address, err := profile.URL(); err != nil || address != "socks5://[::1]:1080" {
		t.Fatalf("IPv6 proxy: %s %v", address, err)
	}
}

func TestProxyValidationAndDirectDefault(t *testing.T) {
	if address, err := DefaultSettings().ProxyURL(); err != nil || address != "" {
		t.Fatal("default route must be direct")
	}
	valid := ProxyProfile{ID: "one", Name: "proxy-1", Host: "localhost", Port: 1080, Protocol: "socks5"}
	for _, change := range []func(*ProxyProfile){
		func(p *ProxyProfile) { p.Port = 0 }, func(p *ProxyProfile) { p.Port = 65536 },
		func(p *ProxyProfile) { p.Host = "proxy.example/path" }, func(p *ProxyProfile) { p.Host = "bad host" },
		func(p *ProxyProfile) { p.Protocol = "ss" }, func(p *ProxyProfile) { p.Password = "password" },
	} {
		profile := valid
		change(&profile)
		if _, err := profile.URL(); err == nil {
			t.Fatal("invalid profile accepted")
		}
	}
	for _, settings := range []Settings{
		{Proxies: []ProxyProfile{valid, valid}},
		{Proxies: []ProxyProfile{valid}, ActiveProxyID: "missing"},
	} {
		if err := ValidateProxies(settings); err == nil {
			t.Fatal("invalid selection accepted")
		}
	}
}
