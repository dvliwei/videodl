//go:build !darwin && !linux && !windows

package proxy

func platformSystemProxyURL() string {
	return ""
}
