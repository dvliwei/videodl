//go:build darwin

package proxy

import (
	"bufio"
	"fmt"
	"os/exec"
	"strings"
)

func platformSystemProxyURL() string {
	services, err := listNetworkServices()
	if err != nil {
		return ""
	}
	for _, svc := range services {
		if proxy := readNetworkServiceProxy(svc, "securewebproxy"); proxy != "" {
			return proxy
		}
	}
	for _, svc := range services {
		if proxy := readNetworkServiceProxy(svc, "webproxy"); proxy != "" {
			return proxy
		}
	}
	return ""
}

func listNetworkServices() ([]string, error) {
	cmd := exec.Command("networksetup", "-listallnetworkservices")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var services []string
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	first := true
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if first {
			first = false
			continue
		}
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "*") {
			continue
		}
		services = append(services, line)
	}
	return services, nil
}

func readNetworkServiceProxy(service, proxyType string) string {
	cmd := exec.Command("networksetup", "-get"+proxyType, service)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}

	var enabled bool
	var server string
	var port string

	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := scanner.Text()
		idx := strings.Index(line, ":")
		if idx < 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		switch strings.ToLower(key) {
		case "enabled":
			enabled = strings.EqualFold(val, "yes") || val == "1"
		case "server":
			server = val
		case "port":
			port = val
		}
	}

	if !enabled || server == "" {
		return ""
	}
	if port == "" {
		port = "80"
	}
	return ValidateHTTPProxyURL(fmt.Sprintf("http://%s:%s", server, port))
}
