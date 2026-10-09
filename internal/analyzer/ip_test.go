package analyzer

import (
	"net"
	"testing"
)

func TestClassifyIPv4(t *testing.T) {
	cases := []struct {
		ip   string
		want ipCategory
	}{
		{"127.0.0.1", catLoopback},
		{"127.255.255.254", catLoopback},
		{"0.0.0.0", catReserved},
		{"0.255.255.255", catReserved},
		{"10.0.0.1", catPrivate},
		{"10.255.255.254", catPrivate},
		{"172.16.0.1", catPrivate},
		{"172.31.255.254", catPrivate},
		{"172.15.0.1", catPublic},
		{"172.32.0.1", catPublic},
		{"192.168.0.1", catPrivate},
		{"192.168.255.254", catPrivate},
		{"169.254.0.1", catLinkLocal},
		{"169.254.255.254", catLinkLocal},
		{"100.64.0.1", catReserved},
		{"100.127.255.254", catReserved},
		{"224.0.0.1", catMulticast},
		{"239.255.255.254", catMulticast},
		{"240.0.0.1", catReserved},
		{"255.255.255.255", catReserved},
		{"8.8.8.8", catPublic},
		{"1.1.1.1", catPublic},
		{"192.0.2.1", catReserved},
		{"198.51.100.1", catReserved},
		{"203.0.113.1", catReserved},
		{"198.18.0.1", catReserved},
	}

	for _, tc := range cases {
		ip := net.ParseIP(tc.ip)
		if ip == nil {
			t.Fatalf("failed to parse %s", tc.ip)
		}
		got := classifyIP(ip)
		if got != tc.want {
			t.Errorf("classifyIP(%s) = %s, want %s", tc.ip, got, tc.want)
		}
	}
}

func TestClassifyIPv6(t *testing.T) {
	cases := []struct {
		ip   string
		want ipCategory
	}{
		{"::1", catLoopback},
		{"::", catUnspecified},
		{"fe80::1", catLinkLocal},
		{"fe80::ffff:ffff:ffff:ffff", catLinkLocal},
		{"ff00::1", catMulticast},
		{"ff02::1", catMulticast},
		{"fc00::1", catUniqueLocal},
		{"fdff:ffff:ffff:ffff:ffff:ffff:ffff:ffff", catUniqueLocal},
		{"2001:db8::1", catPublic},
		{"2606:4700:4700::1111", catPublic},
		{"::ffff:127.0.0.1", catLoopback},
		{"::ffff:10.0.0.1", catPrivate},
		{"::ffff:192.168.1.1", catPrivate},
		{"::ffff:169.254.1.1", catLinkLocal},
	}

	for _, tc := range cases {
		ip := net.ParseIP(tc.ip)
		if ip == nil {
			t.Fatalf("failed to parse %s", tc.ip)
		}
		got := classifyIP(ip)
		if got != tc.want {
			t.Errorf("classifyIP(%s) = %s, want %s", tc.ip, got, tc.want)
		}
	}
}

func TestIsPublicIP(t *testing.T) {
	if !isPublicIP(net.ParseIP("8.8.8.8")) {
		t.Error("8.8.8.8 should be public")
	}
	if isPublicIP(net.ParseIP("127.0.0.1")) {
		t.Error("127.0.0.1 should not be public")
	}
	if isPublicIP(net.ParseIP("10.0.0.1")) {
		t.Error("10.0.0.1 should not be public")
	}
	if isPublicIP(net.ParseIP("::1")) {
		t.Error("::1 should not be public")
	}
}
