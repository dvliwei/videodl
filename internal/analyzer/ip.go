package analyzer

import (
	"net"

	"videodl/internal/netguard"
)

type ipCategory = netguard.IPCategory

const (
	catLoopback    = netguard.CategoryLoopback
	catPrivate     = netguard.CategoryPrivate
	catLinkLocal   = netguard.CategoryLinkLocal
	catMulticast   = netguard.CategoryMulticast
	catUnspecified = netguard.CategoryUnspecified
	catReserved    = netguard.CategoryReserved
	catUniqueLocal = netguard.CategoryUniqueLocal
	catPublic      = netguard.CategoryPublic
)

func classifyIP(ip net.IP) ipCategory { return netguard.ClassifyIP(ip) }
func isPublicIP(ip net.IP) bool       { return netguard.IsPublicIP(ip) }

func hasOnlyPublicIPs(ips []net.IP) (bool, net.IP, ipCategory) {
	for _, ip := range ips {
		cat := classifyIP(ip)
		if cat != catPublic {
			return false, ip, cat
		}
	}
	return true, nil, catPublic
}
