package netguard

import "net"

// IPCategory is the shared address classification used by analyzer and the
// yt-dlp proxy. Only public unicast addresses are allowed upstream.
type IPCategory string

const (
	CategoryLoopback    IPCategory = "loopback"
	CategoryPrivate     IPCategory = "private"
	CategoryLinkLocal   IPCategory = "link-local"
	CategoryMulticast   IPCategory = "multicast"
	CategoryUnspecified IPCategory = "unspecified"
	CategoryReserved    IPCategory = "reserved"
	CategoryUniqueLocal IPCategory = "unique-local"
	CategoryPublic      IPCategory = "public"
)

func ClassifyIP(ip net.IP) IPCategory {
	if ip == nil {
		return CategoryReserved
	}
	if ip4 := ip.To4(); ip4 != nil {
		return classifyIPv4(ip4)
	}
	return classifyIPv6(ip)
}

func IsPublicIP(ip net.IP) bool { return ClassifyIP(ip) == CategoryPublic }

func classifyIPv4(ip net.IP) IPCategory {
	switch {
	case ip[0] == 127:
		return CategoryLoopback
	case ip[0] == 0:
		return CategoryReserved
	case ip[0] == 10:
		return CategoryPrivate
	case ip[0] == 169 && ip[1] == 254:
		return CategoryLinkLocal
	case ip[0] == 172 && ip[1] >= 16 && ip[1] <= 31:
		return CategoryPrivate
	case ip[0] == 192 && ip[1] == 168:
		return CategoryPrivate
	case ip[0] == 100 && ip[1] >= 64 && ip[1] <= 127:
		return CategoryReserved
	case ip[0] == 192 && ip[1] == 0 && ip[2] == 2:
		return CategoryReserved
	case ip[0] == 198 && (ip[1] == 18 || ip[1] == 19):
		return CategoryReserved
	case ip[0] == 198 && ip[1] == 51 && ip[2] == 100:
		return CategoryReserved
	case ip[0] == 203 && ip[1] == 0 && ip[2] == 113:
		return CategoryReserved
	case ip[0] >= 224 && ip[0] <= 239:
		return CategoryMulticast
	case ip[0] >= 240:
		return CategoryReserved
	default:
		return CategoryPublic
	}
}

func classifyIPv6(ip net.IP) IPCategory {
	if ip.IsLoopback() {
		return CategoryLoopback
	}
	if ip.IsLinkLocalUnicast() {
		return CategoryLinkLocal
	}
	if ip.IsLinkLocalMulticast() || ip.IsMulticast() {
		return CategoryMulticast
	}
	if ip.IsUnspecified() {
		return CategoryUnspecified
	}
	if len(ip) == net.IPv6len && ip[0]&0xfe == 0xfc {
		return CategoryUniqueLocal
	}
	return CategoryPublic
}
