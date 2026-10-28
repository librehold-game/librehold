package httpapi

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
	"time"

	"librehold/internal/app"
)

func TestClientKeyTrustsForwardedForOnlyFromProxies(t *testing.T) {
	proxies := []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}
	for _, tc := range []struct {
		name, remote string
		forwarded    []string
		trusted      []netip.Prefix
		want         string
	}{
		{"an untrusted proxy", "10.0.0.5:4000", []string{"1.1.1.1"}, nil, "10.0.0.5"},
		{"from the proxy", "10.0.0.5:4000", []string{"1.1.1.1"}, proxies, "1.1.1.1"},
		{"a forged entry before the proxy's", "10.0.0.5:4000", []string{"6.6.6.6, 1.1.1.1"}, proxies, "1.1.1.1"},
		{"a forged header line before the proxy's", "10.0.0.5:4000", []string{"6.6.6.6", "1.1.1.1"}, proxies, "1.1.1.1"},
		{"a chain of proxies", "10.0.0.5:4000", []string{"1.1.1.1, 10.0.0.9"}, proxies, "1.1.1.1"},
		{"straight from a client", "8.8.8.8:4000", []string{"1.1.1.1"}, proxies, "8.8.8.8"},
		{"no header from the proxy", "10.0.0.5:4000", nil, proxies, "10.0.0.5"},
		{"garbage", "10.0.0.5:4000", []string{"1.1.1.1, nonsense"}, proxies, "10.0.0.5"},
		{"ipv6 through a proxy", "[fd00::1]:4000", []string{"2001:db8::7"}, []netip.Prefix{netip.MustParsePrefix("fd00::/8")}, "2001:db8::/64"},
		{"an ipv4-mapped client", "[::ffff:8.8.8.8]:4000", nil, nil, "8.8.8.8"},
		{"carrier-grade nat", "100.64.1.1:4000", nil, nil, "100.64.1.1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", "/api/v1/auth/login", nil)
			r.RemoteAddr = tc.remote
			for _, v := range tc.forwarded {
				r.Header.Add("X-Forwarded-For", v)
			}
			a := &api{opts: Options{TrustedProxies: tc.trusted}}
			if got := a.clientKey(r); got != tc.want {
				t.Fatalf("client %q, want %q", got, tc.want)
			}
		})
	}
}

func TestClientKeyGroupsAnIPv6NetworkAsOneClient(t *testing.T) {
	a := &api{}
	keys := map[string]bool{}
	for _, remote := range []string{"[2001:db8:1:2::5]:4000", "[2001:db8:1:2:ffff::9]:4000"} {
		r := httptest.NewRequest("POST", "/api/v1/auth/login", nil)
		r.RemoteAddr = remote
		keys[a.clientKey(r)] = true
	}
	if len(keys) != 1 || !keys["2001:db8:1:2::/64"] {
		t.Fatalf("keys %v, want one for the /64", keys)
	}
}

func TestClientKeyOfTheLocalNetwork(t *testing.T) {
	proxies := []netip.Prefix{netip.MustParsePrefix("172.30.0.10/32")}
	for _, tc := range []struct {
		name, remote, local, forwarded string
		trusted                        []netip.Prefix
	}{
		{"a LAN player", "192.168.1.20:4000", "192.168.1.2:8080", "", nil},
		{"the host itself", "127.0.0.1:4000", "127.0.0.1:8080", "", nil},
		{"through a container gateway", "172.17.0.1:4000", "172.17.0.2:8080", "", nil},
		{"a link-local ipv6 player", "[fe80::5%en0]:4000", "[fe80::1%en0]:8080", "", nil},
		{"an ipv6 player on the server's own /64", "[2001:db8:1:2::5]:4000", "[2001:db8:1:2::1]:8080", "", nil},
		{"a LAN player through the proxy", "172.30.0.10:4000", "172.30.0.2:8080", "192.168.1.20", proxies},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", "/api/v1/auth/login", nil)
			r.RemoteAddr = tc.remote
			local, err := net.ResolveTCPAddr("tcp", tc.local)
			if err != nil {
				t.Fatal(err)
			}
			r = r.WithContext(context.WithValue(r.Context(), http.LocalAddrContextKey, local))
			if tc.forwarded != "" {
				r.Header.Set("X-Forwarded-For", tc.forwarded)
			}
			a := &api{opts: Options{TrustedProxies: tc.trusted}}
			if got := a.clientKey(r); got != app.LocalClient {
				t.Fatalf("client %q, want the local network", got)
			}
		})
	}
}

func TestGuardGivesUpOnASlowBody(t *testing.T) {
	defer func(d time.Duration) { bodyTimeout = d }(bodyTimeout)
	bodyTimeout = 200 * time.Millisecond
	var readErr error
	done := make(chan struct{})
	srv := httptest.NewServer((&api{}).guard(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, readErr = io.ReadAll(r.Body)
		close(done)
	})))
	defer srv.Close()
	conn, err := net.Dial("tcp", srv.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	host := srv.Listener.Addr().String()
	fmt.Fprintf(conn, "POST /api/v1/reports HTTP/1.1\r\nHost: %s\r\nOrigin: http://%s\r\nContent-Type: application/json\r\nContent-Length: 100\r\n\r\n{", host, host)
	select {
	case <-done:
		if readErr == nil {
			t.Fatal("the stalled body read to the end")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("the handler still waits for the body")
	}
}
