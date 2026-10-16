// Package ingest polls sources and turns what they link to into items and
// sightings.
package ingest

import (
	"errors"
	"net/url"
	"strings"
)

// trackingParams are query keys that never change the page a URL names.
var trackingParams = map[string]bool{
	"fbclid": true, "gclid": true, "dclid": true, "msclkid": true, "yclid": true,
	"mc_cid": true, "mc_eid": true, "ref": true, "ref_src": true, "ref_url": true,
	"igshid": true, "spm": true, "_hsenc": true, "_hsmi": true, "mkt_tok": true,
	"oly_anon_id": true, "oly_enc_id": true, "vero_id": true, "cmpid": true, "taid": true,
}

// CanonicalURL normalizes a link so that two links to one article compare
// equal: lower-case scheme and host, no default port, no fragment, no
// tracking parameters, and sorted query keys.
func CanonicalURL(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", err
	}
	u.Scheme = strings.ToLower(u.Scheme)
	if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", errors.New("canonical url: not an absolute http(s) URL")
	}
	host := strings.ToLower(u.Hostname())
	if port := u.Port(); port != "" && !(u.Scheme == "http" && port == "80") && !(u.Scheme == "https" && port == "443") {
		host += ":" + port
	}
	u.Host = host
	u.Fragment, u.RawFragment, u.User = "", "", nil
	if u.Path == "" {
		u.Path = "/"
	}
	u.RawPath = ""
	q := u.Query()
	for k := range q {
		if strings.HasPrefix(strings.ToLower(k), "utm_") || trackingParams[strings.ToLower(k)] {
			q.Del(k)
		}
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}
