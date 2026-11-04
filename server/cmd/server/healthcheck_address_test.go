package main

import "testing"

func TestHealthcheckAddress(t *testing.T) {
	tests := []struct {
		name string
		addr string
		want string
	}{
		{"empty host", ":8080", "127.0.0.1:8080"},
		{"wildcard IPv4", "0.0.0.0:9000", "127.0.0.1:9000"},
		{"wildcard IPv6", "[::]:8080", "127.0.0.1:8080"},
		{"loopback IPv4", "127.0.0.1:8081", "127.0.0.1:8081"},
		{"named host", "localhost:8082", "localhost:8082"},
		{"loopback IPv6", "[::1]:8083", "[::1]:8083"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := healthcheckAddress(tt.addr)
			if err != nil {
				t.Fatalf("healthcheckAddress(%q): %v", tt.addr, err)
			}
			if got != tt.want {
				t.Fatalf("healthcheckAddress(%q) = %q, want %q", tt.addr, got, tt.want)
			}
		})
	}
}

func TestHealthcheckAddressRejectsMalformedAddress(t *testing.T) {
	for _, addr := range []string{"localhost", "::1:8080", "127.0.0.1:"} {
		t.Run(addr, func(t *testing.T) {
			if _, err := healthcheckAddress(addr); err == nil {
				t.Fatalf("healthcheckAddress(%q) did not return an error", addr)
			}
		})
	}
}
