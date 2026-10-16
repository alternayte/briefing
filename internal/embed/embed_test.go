package embed

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/alternayte/briefing/internal/platform"
)

func TestEmbedKeepsInputOrderAndChecksDimension(t *testing.T) {
	data, err := os.ReadFile("testdata/embeddings.json")
	if err != nil {
		t.Fatal(err)
	}
	// Answer with the items reversed, as the API may.
	var resp map[string]any
	_ = json.Unmarshal(data, &resp)
	items := resp["data"].([]any)
	items[0], items[1] = items[1], items[0]
	reordered, _ := json.Marshal(resp)

	var sent struct {
		Model string   `json:"model"`
		Input []string `json:"input"`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &sent)
		_, _ = w.Write(reordered)
	}))
	defer srv.Close()
	or := platform.NewOpenRouter(srv.URL, "k", srv.Client(), nil, time.Now)

	e := New(or, platform.Config{EmbedModel: "openai/text-embedding-3-small", EmbedDim: 1536})
	vs, err := e.Embed(context.Background(), "embed", []string{"first", "second"})
	if err != nil {
		t.Fatal(err)
	}
	if sent.Model != "openai/text-embedding-3-small" || len(sent.Input) != 2 {
		t.Errorf("request = %+v", sent)
	}
	if len(vs) != 2 || vs[0].Dim() != 1536 {
		t.Fatalf("vectors = %d", len(vs))
	}
	// The fixture's index 0 embedding is first after reordering by index.
	var orig map[string]any
	_ = json.Unmarshal(data, &orig)
	first := orig["data"].([]any)[0].(map[string]any)["embedding"].([]any)[0].(float64)
	if float64(vs[0].Floats()[0]) != float64(float32(first)) {
		t.Error("vectors are not in input order")
	}

	wrong := New(or, platform.Config{EmbedModel: "m", EmbedDim: 768})
	if _, err := wrong.Embed(context.Background(), "embed", []string{"a", "b"}); err == nil {
		t.Error("a dimension mismatch was accepted")
	}
}
