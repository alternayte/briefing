package ingest

import "testing"

func TestCanonicalURL(t *testing.T) {
	cases := map[string]string{
		"https://Example.COM/a/b?utm_source=x&utm_medium=y&id=3#top": "https://example.com/a/b?id=3",
		"http://example.com:80/":                                     "http://example.com/",
		"https://example.com:443/path/":                              "https://example.com/path/",
		"https://www.example.com/a?b=2&a=1":                          "https://www.example.com/a?a=1&b=2",
		"https://example.com/a?fbclid=1&gclid=2&ref=hn&mc_cid=3":     "https://example.com/a",
		"https://example.com/a?ref_src=twsrc":                        "https://example.com/a",
		"https://example.com":                                        "https://example.com/",
		"https://example.com/%7Euser":                                "https://example.com/~user",
	}
	for in, want := range cases {
		got, err := CanonicalURL(in)
		if err != nil {
			t.Errorf("%s: %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("%s:\n got %s\nwant %s", in, got, want)
		}
	}
}

func TestCanonicalURLRejectsNonWeb(t *testing.T) {
	for _, in := range []string{"", "mailto:a@b.c", "javascript:alert(1)", "/relative", "ftp://example.com/x"} {
		if _, err := CanonicalURL(in); err == nil {
			t.Errorf("%q accepted", in)
		}
	}
}

func TestRegistrableDomain(t *testing.T) {
	cases := map[string]string{
		"https://www.bbc.co.uk/news/x":   "bbc.co.uk",
		"https://blog.cloudflare.com/a":  "cloudflare.com",
		"https://example.github.io/post": "example.github.io",
		"https://localhost:8080/":        "localhost",
	}
	for in, want := range cases {
		if got := RegistrableDomain(in); got != want {
			t.Errorf("%s: %q", in, got)
		}
	}
}
