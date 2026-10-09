package analyzer

import (
	"net"
)

type ipCategory string

const (
	catLoopback    ipCategory = "loopback"
	catPrivate     ipCategory = "private"
	catLinkLocal   ipCategory = "link-local"
	catMulticast   ipCategory = "multicast"
	catUnspecified ipCategory = "unspecified"
	catReserved    ipCategory = "reserved"
	catUniqueLocal ipCategory = "unique-local"
	catPublic      ipCategory = "public"
)

func classifyIP(ip net.IP) ipCategory {
	if ip == nil {
		return catReserved
	}

	if ip4 := ip.To4(); ip4 != nil {
		return classifyIPv4(ip4)
	}
	return classifyIPv6(ip)
}

func classifyIPv4(ip net.IP) ipCategory {
	switch {
	case ip[0] == 127:
		return catLoopback
	case ip[0] == 0:
		return catReserved
	case ip[0] == 10:
		return catPrivate
	case ip[0] == 169 && ip[1] == 254:
		return catLinkLocal
	case ip[0] == 172 && ip[1] >= 16 && ip[1] <= 31:
		return catPrivate
	case ip[0] == 192 && ip[1] == 168:
		return catPrivate
	case ip[0] == 100 && ip[1] >= 64 && ip[1] <= 127:
		return catReserved
	case ip[0] == 192 && ip[1] == 0 && ip[2] == 2:
		return catReserved
	case ip[0] == 198 && (ip[1] == 18 || ip[1] == 19):
		return catReserved
	case ip[0] == 198 && ip[1] == 51 && ip[2] == 100:
		return catReserved
	case ip[0] == 203 && ip[1] == 0 && ip[2] == 113:
		return catReserved
	case ip[0] >= 224 && ip[0] <= 239:
		return catMulticast
	case ip[0] >= 240:
		return catReserved
	}
	return catPublic
}

func classifyIPv6(ip net.IP) ipCategory {
	if ip.IsLoopback() {
		return catLoopback
	}
	if ip.IsLinkLocalUnicast() {
		return catLinkLocal
	}
	if ip.IsLinkLocalMulticast() {
		return catMulticast
	}
	if ip.IsMulticast() {
		return catMulticast
	}
	if ip.IsUnspecified() {
		return catUnspecified
	}

	if len(ip) == net.IPv6len {
		if ip[0]&0xfe == 0xfc {
			return catUniqueLocal
		}
	}

	return catPublic
}

func isPublicIP(ip net.IP) bool {
	return classifyIP(ip) == catPublic
}

func hasOnlyPublicIPs(ips []net.IP) (bool, net.IP, ipCategory) {
	for _, ip := range ips {
		cat := classifyIP(ip)
		if cat != catPublic {
			return false, ip, cat
		}
	}
	return true, nil, catPublic
}
