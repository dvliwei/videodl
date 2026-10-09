package analyzer

import (
	"testing"
)

func TestValidateURL_ValidHTTP(t *testing.T) {
	u, err := ValidateURL("http://example.com/path")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if u.Hostname() != "example.com" {
		t.Errorf("hostname = %q, want %q", u.Hostname(), "example.com")
	}
}

func TestValidateURL_ValidHTTPS(t *testing.T) {
	_, err := ValidateURL("https://example.com:8080/path?q=1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestValidateURL_RejectsFTP(t *testing.T) {
	_, err := ValidateURL("ftp://example.com/file")
	if err == nil {
		t.Fatal("expected error for ftp scheme")
	}
}

func TestValidateURL_RejectsFileScheme(t *testing.T) {
	_, err := ValidateURL("file:///etc/passwd")
	if err == nil {
		t.Fatal("expected error for file scheme")
	}
}

func TestValidateURL_RejectsDataScheme(t *testing.T) {
	_, err := ValidateURL("data:text/html,<h1>hi</h1>")
	if err == nil {
		t.Fatal("expected error for data scheme")
	}
}

func TestValidateURL_RejectsInvalidScheme(t *testing.T) {
	_, err := ValidateURL("javascript:alert(1)")
	if err == nil {
		t.Fatal("expected error for javascript scheme")
	}
}

func TestValidateURL_RejectsUserInfo(t *testing.T) {
	_, err := ValidateURL("http://user:pass@example.com/page")
	if err == nil {
		t.Fatal("expected error for URL with user info")
	}
}

func TestValidateURL_AcceptsLoopbackHostname(t *testing.T) {
	_, err := ValidateURL("http://127.0.0.1/page")
	if err != nil {
		t.Fatalf("loopback IP syntax is valid, got %v", err)
	}
}

func TestValidateURL_AcceptsLocalhost(t *testing.T) {
	_, err := ValidateURL("http://localhost/page")
	if err != nil {
		t.Fatalf("localhost hostname should pass URL validation (DNS resolution happens later), got %v", err)
	}
}

func TestValidateURL_AcceptsPrivateIP(t *testing.T) {
	cases := []string{
		"http://10.0.0.1/page",
		"http://172.16.0.1/page",
		"http://192.168.0.1/page",
	}
	for _, raw := range cases {
		_, err := ValidateURL(raw)
		if err != nil {
			t.Errorf("syntax-valid private IP should be accepted, got error: %v for %s", err, raw)
		}
	}
}

func TestValidateURL_AcceptsLinkLocal(t *testing.T) {
	_, err := ValidateURL("http://169.254.1.1/page")
	if err != nil {
		t.Fatalf("link-local IP syntax is valid, got %v", err)
	}
}

func TestValidateURL_AcceptsIPv6Loopback(t *testing.T) {
	_, err := ValidateURL("http://[::1]/page")
	if err != nil {
		t.Fatalf("IPv6 loopback syntax is valid, got %v", err)
	}
}

func TestValidateURL_AcceptsIPv6LinkLocal(t *testing.T) {
	_, err := ValidateURL("http://[fe80::1]/page")
	if err != nil {
		t.Fatalf("IPv6 link-local syntax is valid, got %v", err)
	}
}

func TestValidateURL_AcceptsIPv6Public(t *testing.T) {
	_, err := ValidateURL("http://[2001:db8::1]/page")
	if err != nil {
		t.Fatalf("expected no error for public IPv6, got %v", err)
	}
}

func TestValidateURL_RejectsEmptyHost(t *testing.T) {
	_, err := ValidateURL("http:///path")
	if err == nil {
		t.Fatal("expected error for empty host")
	}
}

func TestValidateURL_RejectsMissingScheme(t *testing.T) {
	_, err := ValidateURL("example.com/path")
	if err == nil {
		t.Fatal("expected error for missing scheme")
	}
}

func TestValidateURL_AcceptsBogonAddresses(t *testing.T) {
	cases := []string{
		"http://0.0.0.0/page",
		"http://224.0.0.1/page",
		"http://255.255.255.255/page",
		"http://192.0.2.1/page",
		"http://203.0.113.1/page",
		"http://100.64.0.1/page",
	}
	for _, raw := range cases {
		_, err := ValidateURL(raw)
		if err != nil {
			t.Errorf("syntax-valid bogon IP should be accepted by ValidateURL (rejected later by SafeClient), got error: %v for %s", err, raw)
		}
	}
}

func TestSanitizeDisplayURL_StripsQueryAndFragment(t *testing.T) {
	u, _ := ValidateURL("https://example.com/video.mp4?token=abc123#section")
	got := SanitizeDisplayURL(u)
	want := "https://example.com/video.mp4"
	if got != want {
		t.Errorf("SanitizeDisplayURL() = %q, want %q", got, want)
	}
}

func TestSanitizeDisplayURL_NoQueryNoFragment(t *testing.T) {
	u, _ := ValidateURL("https://example.com/video.mp4")
	got := SanitizeDisplayURL(u)
	if got != "https://example.com/video.mp4" {
		t.Errorf("SanitizeDisplayURL() = %q", got)
	}
}
