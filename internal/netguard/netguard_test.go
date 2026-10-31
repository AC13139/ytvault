package netguard

import (
	"errors"
	"net"
	"testing"
	"time"
)

const defaultTestTimeout = 5 * time.Second

func TestIsPublicIP(t *testing.T) {
	tests := []struct {
		ip     string
		public bool
	}{
		{"8.8.8.8", true},
		{"1.1.1.1", true},
		{"2606:4700:4700::1111", true},
		{"127.0.0.1", false},
		{"127.10.20.30", false},
		{"10.0.0.1", false},
		{"172.16.0.1", false},
		{"192.168.1.1", false},
		{"169.254.169.254", false}, // cloud metadata
		{"0.0.0.0", false},
		{"100.64.0.1", false}, // CGNAT
		{"::1", false},
		{"::", false},
		{"fc00::1", false}, // unique local
		{"fe80::1", false}, // link local
		{"::ffff:127.0.0.1", false},
		{"::ffff:10.0.0.1", false},
	}
	for _, tt := range tests {
		ip := net.ParseIP(tt.ip)
		if ip == nil {
			t.Fatalf("bad test address %q", tt.ip)
		}
		if got := IsPublicIP(ip); got != tt.public {
			t.Errorf("IsPublicIP(%s) = %v, want %v", tt.ip, got, tt.public)
		}
	}
}

func TestValidateURLRejectsInternalAddresses(t *testing.T) {
	tests := []string{
		"http://127.0.0.1/",
		"https://127.0.0.1/",
		"http://169.254.169.254/latest/meta-data/",
		"http://0.0.0.0/latest/meta-data/",
		"http://[::]/",
		"http://[::1]/",
		"http://10.0.0.5/",
		"http://192.168.0.1/",
		"http://localhost:8080/",
	}
	for _, raw := range tests {
		err := ValidateURL(raw)
		if err == nil {
			t.Errorf("ValidateURL(%q) = nil, want an error", raw)
			continue
		}
		if !errors.Is(err, ErrBlocked) {
			t.Errorf("ValidateURL(%q) = %v, want ErrBlocked", raw, err)
		}
	}
}

func TestValidateURLRejectsNonHTTPSchemes(t *testing.T) {
	for _, raw := range []string{"ftp://example.com/x", "file:///etc/passwd", "gopher://example.com"} {
		if err := ValidateURL(raw); err == nil {
			t.Errorf("ValidateURL(%q) = nil, want an error", raw)
		}
	}
}

func TestValidateURLAcceptsPublicAddress(t *testing.T) {
	if err := ValidateURL("https://8.8.8.8/"); err != nil {
		t.Errorf("ValidateURL(public) = %v, want nil", err)
	}
}

func TestClientRefusesToDialLoopback(t *testing.T) {
	client := Client(defaultTestTimeout, 3)

	// A server on loopback must be unreachable through the guard.
	resp, err := client.Get("http://127.0.0.1:1/")
	if err == nil {
		resp.Body.Close()
		t.Fatal("request to loopback succeeded, want it blocked")
	}
	if !errors.Is(err, ErrBlocked) {
		t.Errorf("error = %v, want ErrBlocked", err)
	}
}
