package platform

import (
	"strings"
	"testing"
)

func validEnv() map[string]string {
	return map[string]string{
		"DATABASE_URL":         "postgres://briefing@localhost/briefing",
		"BASE_URL":             "https://briefing.example.com",
		"SESSION_SECRET":       strings.Repeat("s", 32),
		"GITHUB_CLIENT_ID":     "id",
		"GITHUB_CLIENT_SECRET": "secret",
		"OPENROUTER_API_KEY":   "key",
	}
}

func lookup(env map[string]string) func(string) (string, bool) {
	return func(k string) (string, bool) {
		v, ok := env[k]
		return v, ok
	}
}

func TestLoadConfigNamesEveryMissingRequiredValue(t *testing.T) {
	_, err := LoadConfig(lookup(map[string]string{"BASE_URL": "https://x.example"}))
	if err == nil {
		t.Fatal("want an error")
	}
	for _, k := range []string{"DATABASE_URL", "SESSION_SECRET", "GITHUB_CLIENT_ID", "GITHUB_CLIENT_SECRET", "OPENROUTER_API_KEY"} {
		if !strings.Contains(err.Error(), k) {
			t.Errorf("error %q does not name %s", err, k)
		}
	}
}

func TestLoadConfigAppliesDefaults(t *testing.T) {
	c, err := LoadConfig(lookup(validEnv()))
	if err != nil {
		t.Fatal(err)
	}
	if c.DecideAPI != DecideAPIDecisions || c.DecideModel != "cloudflare/clef" {
		t.Errorf("decide = %s %s", c.DecideAPI, c.DecideModel)
	}
	if c.EmbedModel != "openai/text-embedding-3-small" || c.EmbedDim != 1536 {
		t.Errorf("embed = %s %d", c.EmbedModel, c.EmbedDim)
	}
	if c.DailySpendUSD != 1 {
		t.Errorf("spend cap = %v", c.DailySpendUSD)
	}
	if c.MagicLinkEnabled() || c.RenderingEnabled() || c.SignUpRestricted() {
		t.Error("optional features must be off without their env")
	}
}

func TestLoadConfigParsesEmailListsCaseInsensitively(t *testing.T) {
	env := validEnv()
	env["ALLOWED_EMAILS"] = " Ann@Example.com, bob@example.com ,"
	env["ADMIN_EMAILS"] = "ANN@example.com"
	c, err := LoadConfig(lookup(env))
	if err != nil {
		t.Fatal(err)
	}
	if !c.SignUpRestricted() || !c.EmailAllowed("ann@example.COM") || c.EmailAllowed("eve@example.com") {
		t.Errorf("allowlist = %v", c.AllowedEmails)
	}
	if !c.IsAdmin("Ann@example.com") || c.IsAdmin("bob@example.com") {
		t.Errorf("admins = %v", c.AdminEmails)
	}
}

func TestLoadConfigOpenSignUpAllowsAnyEmail(t *testing.T) {
	c, err := LoadConfig(lookup(validEnv()))
	if err != nil {
		t.Fatal(err)
	}
	if !c.EmailAllowed("anyone@example.com") {
		t.Error("an empty allowlist means open sign-up")
	}
}

func TestLoadConfigRejectsInvalidValues(t *testing.T) {
	cases := map[string]map[string]string{
		"relative base url":     {"BASE_URL": "/briefing"},
		"short secret":          {"SESSION_SECRET": "short"},
		"unknown decide api":    {"DECIDE_API": "magic"},
		"non-numeric dim":       {"EMBED_DIM": "big"},
		"zero dim":              {"EMBED_DIM": "0"},
		"negative spend cap":    {"BRIEFING_DAILY_SPEND_USD": "-1"},
		"cloudflare half set":   {"CF_ACCOUNT_ID": "acct"},
		"cloudflare other half": {"CF_API_TOKEN": "tok"},
	}
	for name, override := range cases {
		t.Run(name, func(t *testing.T) {
			env := validEnv()
			for k, v := range override {
				env[k] = v
			}
			if _, err := LoadConfig(lookup(env)); err == nil {
				t.Error("want an error")
			}
		})
	}
}

func TestLoadConfigEnablesOptionalFeatures(t *testing.T) {
	env := validEnv()
	env["RESEND_API_KEY"] = "re_x"
	env["CF_ACCOUNT_ID"] = "acct"
	env["CF_API_TOKEN"] = "tok"
	env["DECIDE_API"] = "systemone"
	c, err := LoadConfig(lookup(env))
	if err != nil {
		t.Fatal(err)
	}
	if !c.MagicLinkEnabled() || !c.RenderingEnabled() {
		t.Error("features must turn on with their env")
	}
	if c.DecideAPI != DecideAPISystemOne || c.DecideModel != "jev-latest" {
		t.Errorf("systemone default model = %s", c.DecideModel)
	}
}
