package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	authall "github.com/alternayte/auth-all"
	"github.com/alternayte/auth-all/plugins/apikeys"
	"github.com/alternayte/drel"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/alternayte/briefing/internal/accounts"
	"github.com/alternayte/briefing/internal/admin"
	"github.com/alternayte/briefing/internal/feed"
	"github.com/alternayte/briefing/internal/platform"
)

type bearer struct{ token string }

func (b bearer) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+b.token)
	return http.DefaultTransport.RoundTrip(r)
}

func connect(ctx context.Context, endpoint, token string) (*mcp.ClientSession, error) {
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	return client.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint:   endpoint,
		HTTPClient: &http.Client{Transport: bearer{token}},
	}, nil)
}

func TestMCPNeedsAValidAccessToken(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	srv := httptest.NewUnstartedServer(nil)
	srv.Start()
	defer srv.Close()
	cfg, err := platform.LoadConfig(func(k string) (string, bool) {
		v, ok := map[string]string{
			"DATABASE_URL": dsn, "BASE_URL": srv.URL, "SESSION_SECRET": strings.Repeat("s", 32),
			"GITHUB_CLIENT_ID": "id", "GITHUB_CLIENT_SECRET": "secret", "OPENROUTER_API_KEY": "key",
		}[k]
		return v, ok
	})
	if err != nil {
		t.Fatal(err)
	}
	pool, err := platform.OpenPool(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	engine, err := drel.NewEngine(dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(engine.Close)
	auth, err := accounts.NewAuth(cfg, pool)
	if err != nil {
		t.Fatal(err)
	}
	web := accounts.NewWeb(cfg, auth, accounts.NewProfileStore(engine, platform.DefaultTuning()))
	jobs, err := platform.NewRiver(pool, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	feedWeb := feed.NewWeb(engine, nil, nil, accounts.NewProfileStore(engine, platform.DefaultTuning()), jobs, platform.DefaultTuning(), time.Now)
	srv.Config.Handler = newHandler(web, feedWeb, admin.NewWeb(engine, cfg, time.Now), pool.Ping)

	email := fmt.Sprintf("mcp-%d@example.com", time.Now().UnixNano())
	user, err := auth.CreateUser(ctx, authall.CreateUserInput{Email: email, DisplayName: "Mcp Tester", EmailVerified: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), "DELETE FROM auth_users WHERE id = $1", user.ID) })
	key, token, err := auth.Keys.Create(ctx, user, apikeys.CreateInput{UserID: user.ID, Name: "test"})
	if err != nil {
		t.Fatal(err)
	}
	endpoint := srv.URL + "/mcp"

	if _, err := connect(ctx, endpoint, "brf_not-a-real-token"); err == nil {
		t.Error("an unknown token connected")
	}

	session, err := connect(ctx, endpoint, token)
	if err != nil {
		t.Fatal(err)
	}
	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, tool := range tools.Tools {
		names[tool.Name] = true
	}
	for _, want := range []string{"accounts_me", "feed_feedback", "feed_explain"} {
		if !names[want] {
			t.Errorf("%s missing from the tools %v", want, names)
		}
	}
	res, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "accounts_me", Arguments: map[string]any{}})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(res.StructuredContent)
	var id accounts.Identity
	if err := json.Unmarshal(raw, &id); err != nil || id.UserID != user.ID || id.DisplayName != "Mcp Tester" {
		t.Errorf("whoami = %s (err %v, error result %v)", raw, err, res.IsError)
	}
	_ = session.Close()

	if err := auth.Keys.Revoke(ctx, key.ID, user, false); err != nil {
		t.Fatal(err)
	}
	if _, err := connect(ctx, endpoint, token); err == nil {
		t.Error("a revoked token connected")
	}
}
