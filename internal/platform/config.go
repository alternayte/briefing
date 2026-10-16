package platform

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
)

type DecideAPI string

const (
	DecideAPIDecisions DecideAPI = "decisions"
	DecideAPISystemOne DecideAPI = "systemone"
)

const minSessionSecretLen = 32

// Config is every value the process reads from its environment.
type Config struct {
	Addr               string
	DatabaseURL        string
	BaseURL            *url.URL
	SessionSecret      string
	GitHubClientID     string
	GitHubClientSecret string
	OpenRouterAPIKey   string

	AllowedEmails []string
	AdminEmails   []string
	ResendAPIKey  string
	EmailFrom     string

	DecideAPI      DecideAPI
	DecideModel    string
	EmbedModel     string
	EmbedDim       int
	WriteModel     string
	TranslateModel string

	DailySpendUSD float64

	CFAccountID string
	CFAPIToken  string
}

// LoadConfigFromEnv reads the config from the process environment.
func LoadConfigFromEnv() (Config, error) {
	return LoadConfig(os.LookupEnv)
}

// LoadConfig reads the config through lookup and reports every invalid or
// missing value at once.
func LoadConfig(lookup func(string) (string, bool)) (Config, error) {
	r := envReader{lookup: lookup}
	c := Config{
		Addr:               r.optional("ADDR", ":8080"),
		DatabaseURL:        r.required("DATABASE_URL"),
		SessionSecret:      r.required("SESSION_SECRET"),
		GitHubClientID:     r.required("GITHUB_CLIENT_ID"),
		GitHubClientSecret: r.required("GITHUB_CLIENT_SECRET"),
		OpenRouterAPIKey:   r.required("OPENROUTER_API_KEY"),
		AllowedEmails:      emailList(r.optional("ALLOWED_EMAILS", "")),
		AdminEmails:        emailList(r.optional("ADMIN_EMAILS", "")),
		ResendAPIKey:       r.optional("RESEND_API_KEY", ""),
		DecideAPI:          DecideAPI(r.optional("DECIDE_API", string(DecideAPIDecisions))),
		EmbedModel:         r.optional("EMBED_MODEL", "openai/text-embedding-3-small"),
		EmbedDim:           r.positiveInt("EMBED_DIM", 1536),
		WriteModel:         r.optional("WRITE_MODEL", "anthropic/claude-haiku-5.5"),
		TranslateModel:     r.optional("TRANSLATE_MODEL", "anthropic/claude-haiku-5.5"),
		DailySpendUSD:      r.nonNegativeFloat("BRIEFING_DAILY_SPEND_USD", 1),
		CFAccountID:        r.optional("CF_ACCOUNT_ID", ""),
		CFAPIToken:         r.optional("CF_API_TOKEN", ""),
	}

	if raw := r.required("BASE_URL"); raw != "" {
		u, err := url.Parse(raw)
		if err != nil || u.Scheme == "" || u.Host == "" {
			r.fail("BASE_URL must be an absolute URL, got %q", raw)
		}
		c.BaseURL = u
		c.EmailFrom = r.optional("EMAIL_FROM", "Briefing <briefing@"+u.Hostname()+">")
	}
	if c.SessionSecret != "" && len(c.SessionSecret) < minSessionSecretLen {
		r.fail("SESSION_SECRET must be at least %d characters", minSessionSecretLen)
	}
	switch c.DecideAPI {
	case DecideAPIDecisions:
		c.DecideModel = r.optional("DECIDE_MODEL", "cloudflare/clef")
	case DecideAPISystemOne:
		c.DecideModel = r.optional("DECIDE_MODEL", "~typesafe/jev-latest")
	default:
		r.fail("DECIDE_API must be %q or %q, got %q", DecideAPIDecisions, DecideAPISystemOne, c.DecideAPI)
	}
	if (c.CFAccountID == "") != (c.CFAPIToken == "") {
		r.fail("CF_ACCOUNT_ID and CF_API_TOKEN must be set together")
	}

	if len(r.errs) > 0 {
		return Config{}, fmt.Errorf("config: %w", errors.Join(r.errs...))
	}
	return c, nil
}

func (c Config) MagicLinkEnabled() bool { return c.ResendAPIKey != "" }

func (c Config) RenderingEnabled() bool { return c.CFAccountID != "" }

// SignUpRestricted reports whether only allowlisted emails may sign in.
func (c Config) SignUpRestricted() bool { return len(c.AllowedEmails) > 0 }

func (c Config) EmailAllowed(email string) bool {
	return !c.SignUpRestricted() || slices.Contains(c.AllowedEmails, normalizeEmail(email))
}

func (c Config) IsAdmin(email string) bool {
	return slices.Contains(c.AdminEmails, normalizeEmail(email))
}

type envReader struct {
	lookup func(string) (string, bool)
	errs   []error
}

func (r *envReader) fail(format string, args ...any) {
	r.errs = append(r.errs, fmt.Errorf(format, args...))
}

func (r *envReader) required(key string) string {
	v, _ := r.lookup(key)
	v = strings.TrimSpace(v)
	if v == "" {
		r.fail("%s is required", key)
	}
	return v
}

func (r *envReader) optional(key, fallback string) string {
	v, _ := r.lookup(key)
	if v = strings.TrimSpace(v); v != "" {
		return v
	}
	return fallback
}

func (r *envReader) positiveInt(key string, fallback int) int {
	raw := r.optional(key, "")
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		r.fail("%s must be a positive integer, got %q", key, raw)
	}
	return n
}

func (r *envReader) nonNegativeFloat(key string, fallback float64) float64 {
	raw := r.optional(key, "")
	if raw == "" {
		return fallback
	}
	f, err := strconv.ParseFloat(raw, 64)
	if err != nil || f < 0 {
		r.fail("%s must be a number of at least 0, got %q", key, raw)
	}
	return f
}

func emailList(raw string) []string {
	var out []string
	for e := range strings.SplitSeq(raw, ",") {
		if e = normalizeEmail(e); e != "" {
			out = append(out, e)
		}
	}
	return out
}

func normalizeEmail(e string) string { return strings.ToLower(strings.TrimSpace(e)) }
