//go:build windows

package proxy

import (
	"os/exec"
	"strings"
)

const windowsInternetSettingsKey = `HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings`

func platformSystemProxyURL() string {
	enabled := registryValue("ProxyEnable")
	server := registryValue("ProxyServer")
	if enabled != "0x1" || server == "" {
		return ""
	}

	for _, entry := range strings.Split(server, ";") {
		entry = strings.TrimSpace(entry)
		if strings.HasPrefix(strings.ToLower(entry), "http=") {
			entry = strings.TrimSpace(entry[len("http="):])
		}
		if strings.Contains(entry, "=") {
			continue
		}
		candidate := entry
		if !strings.Contains(candidate, "://") {
			candidate = "http://" + candidate
		}
		if validated := ValidateHTTPProxyURL(candidate); validated != "" {
			return validated
		}
	}
	return ""
}

func registryValue(name string) string {
	out, err := exec.Command("reg", "query", windowsInternetSettingsKey, "/v", name).Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 3 && strings.EqualFold(fields[0], name) {
			return strings.TrimSpace(strings.Join(fields[2:], " "))
		}
	}
	return ""
}
