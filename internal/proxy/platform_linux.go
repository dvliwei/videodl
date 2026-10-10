//go:build linux

package proxy

import (
	"fmt"
	"os/exec"
	"strings"
)

func platformSystemProxyURL() string {
	mode := gsettingsValue("org.gnome.system.proxy", "mode")
	if strings.Trim(mode, "'\"") != "manual" {
		return ""
	}

	host := gsettingsValue("org.gnome.system.proxy.http", "host")
	port := gsettingsValue("org.gnome.system.proxy.http", "port")
	if host == "" || port == "" {
		return ""
	}
	return ValidateHTTPProxyURL(fmt.Sprintf("http://%s:%s", host, port))
}

func gsettingsValue(schema, key string) string {
	out, err := exec.Command("gsettings", "get", schema, key).Output()
	if err != nil {
		return ""
	}
	return strings.Trim(strings.TrimSpace(string(out)), "'\"")
}
