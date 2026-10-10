//go:build !darwin

package proxy

func platformSystemProxyURL() string {
	return ""
}
