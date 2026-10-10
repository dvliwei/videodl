package netguard

import (
	"context"
	"fmt"
	"net"
	"strings"
)

type IPResolver interface {
	LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error)
}

type Resolver struct {
	lookup IPResolver
}

func NewResolver(lookup IPResolver) *Resolver {
	if lookup == nil {
		lookup = net.DefaultResolver
	}
	return &Resolver{lookup: lookup}
}

func (r *Resolver) ValidateHost(ctx context.Context, host string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	host = strings.TrimSpace(host)
	if host == "" || strings.HasPrefix(host, "[") || strings.HasSuffix(host, "]") || strings.ContainsAny(host, " \t\r\n/\\") {
		return fmt.Errorf("netguard: invalid host")
	}
	if ip := net.ParseIP(host); ip != nil {
		if !IsPublicIP(ip) {
			return fmt.Errorf("netguard: host address is not public (%s)", ClassifyIP(ip))
		}
		return nil
	}
	ips, err := r.lookup.LookupIPAddr(ctx, host)
	if err != nil {
		return fmt.Errorf("netguard: DNS resolution failed: %w", err)
	}
	if len(ips) == 0 {
		return fmt.Errorf("netguard: host resolved to no addresses")
	}
	for _, addr := range ips {
		if !IsPublicIP(addr.IP) {
			return fmt.Errorf("netguard: host resolves to a non-public address (%s)", ClassifyIP(addr.IP))
		}
	}
	return nil
}
