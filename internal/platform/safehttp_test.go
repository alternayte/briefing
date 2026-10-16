package platform

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestPublicClientRefusesPrivateAddresses(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	client := NewPublicClient(5*time.Second, 3)
	_, err := client.Get(srv.URL)
	if !errors.Is(err, ErrPrivateAddress) {
		t.Fatalf("loopback: got %v", err)
	}
}

func TestIsPublicIP(t *testing.T) {
	cases := map[string]bool{
		"8.8.8.8":          true,
		"2606:4700::1111":  true,
		"127.0.0.1":        false,
		"10.1.2.3":         false,
		"172.16.0.1":       false,
		"192.168.1.1":      false,
		"169.254.169.254":  false,
		"100.64.0.1":       false,
		"0.0.0.0":          false,
		"::1":              false,
		"fe80::1":          false,
		"fc00::1":          false,
		"::ffff:127.0.0.1": false,
	}
	for ip, want := range cases {
		if got := isPublicIP(net.ParseIP(ip)); got != want {
			t.Errorf("%s: got %v", ip, got)
		}
	}
}

func TestPublicClientLimitsRedirects(t *testing.T) {
	// via holds the requests made so far, so its length is the redirect's
	// number: nature.com needs three hops to set its cookie.
	c := NewPublicClient(time.Second, 3)
	if err := c.CheckRedirect(httptestRequest(t), make([]*http.Request, 3)); err != nil {
		t.Errorf("a third redirect must pass: %v", err)
	}
	if err := c.CheckRedirect(httptestRequest(t), make([]*http.Request, 4)); err == nil {
		t.Error("a fourth redirect must fail")
	}
}

func httptestRequest(t *testing.T) *http.Request {
	r, err := http.NewRequestWithContext(context.Background(), "GET", "https://example.com", nil)
	if err != nil {
		t.Fatal(err)
	}
	return r
}
