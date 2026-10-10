package netguard

import (
	"context"
	"errors"
	"net"
	"testing"
)

type fakeLookup struct {
	ips []net.IPAddr
	err error
}

func (f fakeLookup) LookupIPAddr(context.Context, string) ([]net.IPAddr, error) {
	return f.ips, f.err
}

func TestResolverRejectsNonPublicAddresses(t *testing.T) {
	for _, ip := range []string{
		"127.0.0.1", "10.0.0.1", "169.254.1.1", "224.0.0.1", "192.0.2.1",
		"::1", "fe80::1", "fc00::1", "ff02::1", "::ffff:192.0.2.1",
	} {
		r := NewResolver(fakeLookup{ips: []net.IPAddr{{IP: net.ParseIP(ip)}}})
		if err := r.ValidateHost(context.Background(), "example.test"); err == nil {
			t.Errorf("%s should be rejected", ip)
		}
	}
}

func TestResolverAcceptsPublicAddressAndLiteral(t *testing.T) {
	r := NewResolver(fakeLookup{ips: []net.IPAddr{{IP: net.ParseIP("8.8.8.8")}}})
	if err := r.ValidateHost(context.Background(), "example.test"); err != nil {
		t.Fatal(err)
	}
	if err := r.ValidateHost(context.Background(), "1.1.1.1"); err != nil {
		t.Fatal(err)
	}
}

func TestResolverRejectsDNSFailureAndEmptyResults(t *testing.T) {
	if err := NewResolver(fakeLookup{err: errors.New("lookup failed")}).ValidateHost(context.Background(), "example.test"); err == nil {
		t.Fatal("expected DNS error")
	}
	if err := NewResolver(fakeLookup{}).ValidateHost(context.Background(), "example.test"); err == nil {
		t.Fatal("expected empty DNS result error")
	}
}

func TestResolverRejectsInvalidHost(t *testing.T) {
	r := NewResolver(fakeLookup{ips: []net.IPAddr{{IP: net.ParseIP("8.8.8.8")}}})
	for _, host := range []string{"", "bad host", "[::1]"} {
		if err := r.ValidateHost(context.Background(), host); err == nil {
			t.Errorf("%q should be rejected", host)
		}
	}
}
