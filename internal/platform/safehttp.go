package platform

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/netip"
	"syscall"
	"time"

	"golang.org/x/net/publicsuffix"
)

// ErrPrivateAddress reports a connection to an address that is not on the
// public internet.
var ErrPrivateAddress = errors.New("refusing a private, loopback or link-local address")

// carrierNAT is 100.64.0.0/10, which net.IP has no predicate for.
var carrierNAT = netip.MustParsePrefix("100.64.0.0/10")

// NewPublicClient returns an HTTP client for URLs that users or feeds supply.
// It checks every resolved address at dial time, so a DNS answer cannot point
// it at the host network, and it follows at most maxRedirects redirects.
func NewPublicClient(timeout time.Duration, maxRedirects int) *http.Client {
	dialer := &net.Dialer{
		Timeout: timeout,
		Control: func(_, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			if ip := net.ParseIP(host); ip == nil || !isPublicIP(ip) {
				return fmt.Errorf("%w: %s", ErrPrivateAddress, host)
			}
			return nil
		},
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = dialer.DialContext
	// Some publishers set a cookie and redirect back to the same URL; without
	// a jar that loops until the redirect limit.
	jar, _ := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
		Jar:       jar,
		CheckRedirect: func(_ *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return fmt.Errorf("stopped after %d redirects", maxRedirects)
			}
			return nil
		},
	}
}

func isPublicIP(ip net.IP) bool {
	if v4 := ip.To4(); v4 != nil {
		ip = v4
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsMulticast() || ip.IsUnspecified() || ip.IsInterfaceLocalMulticast() {
		return false
	}
	if a, ok := netip.AddrFromSlice(ip); ok && carrierNAT.Contains(a.Unmap()) {
		return false
	}
	return true
}
