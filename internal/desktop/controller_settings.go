package desktop

import (
	"errors"
	"slices"
	"strings"

	"cryptotape/internal/core"
	"github.com/google/uuid"
)

func (c *Controller) SaveSettings(settings core.Settings) error {
	c.settingsMu.Lock()
	defer c.settingsMu.Unlock()
	c.mu.RLock()
	settings.Proxies = slices.Clone(c.settings.Proxies)
	settings.ActiveProxyID = c.settings.ActiveProxyID
	c.mu.RUnlock()
	return c.applySettings(settings)
}

func (c *Controller) applySettings(settings core.Settings) error {
	c.mu.RLock()
	previous, err := c.settings.ProxyURL()
	c.mu.RUnlock()
	if err != nil {
		return err
	}
	route, err := settings.ProxyURL()
	if err != nil {
		return err
	}
	if err := c.store.SaveSettings(settings); err != nil {
		return err
	}
	c.mu.Lock()
	c.settings = settings
	rules := append([]core.Rule(nil), c.rules...)
	c.mu.Unlock()
	c.engine.Configure(rules, settings)
	c.emit("settings:changed", settings)
	if previous != route {
		c.Reconnect()
	}
	return nil
}

func (c *Controller) proxySettings() core.Settings {
	c.mu.RLock()
	defer c.mu.RUnlock()
	settings := c.settings
	settings.Proxies = append([]core.ProxyProfile{}, settings.Proxies...)
	return settings
}

func (c *Controller) SaveProxy(profile core.ProxyProfile, use bool) error {
	c.settingsMu.Lock()
	defer c.settingsMu.Unlock()
	settings := c.proxySettings()
	profile.Name = strings.TrimSpace(profile.Name)
	profile.Host = strings.TrimSpace(profile.Host)
	profile.Protocol = "socks5"
	if profile.Login != "" || profile.Password != "" {
		profile.Protocol = "http"
	}
	isNew := profile.ID == ""
	if isNew {
		profile.ID = uuid.NewString()
	}
	index := slices.IndexFunc(settings.Proxies, func(p core.ProxyProfile) bool { return p.ID == profile.ID })
	if !isNew && index < 0 {
		return errors.New("unknown proxy profile")
	}
	if index < 0 {
		settings.Proxies = append(settings.Proxies, profile)
	} else {
		settings.Proxies[index] = profile
	}
	if use {
		settings.ActiveProxyID = profile.ID
	} else if settings.ActiveProxyID == profile.ID {
		settings.ActiveProxyID = ""
	}
	return c.applySettings(settings)
}

func (c *Controller) SelectProxy(id string) error {
	c.settingsMu.Lock()
	defer c.settingsMu.Unlock()
	settings := c.proxySettings()
	settings.ActiveProxyID = id
	return c.applySettings(settings)
}

func (c *Controller) DeleteProxy(id string) error {
	c.settingsMu.Lock()
	defer c.settingsMu.Unlock()
	settings := c.proxySettings()
	index := slices.IndexFunc(settings.Proxies, func(p core.ProxyProfile) bool { return p.ID == id })
	if index < 0 {
		return errors.New("unknown proxy profile")
	}
	settings.Proxies = slices.Delete(settings.Proxies, index, index+1)
	if settings.ActiveProxyID == id {
		settings.ActiveProxyID = ""
	}
	return c.applySettings(settings)
}
