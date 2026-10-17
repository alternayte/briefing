set dotenv-load

test_db := "postgres://briefing:briefing@localhost:55432/briefing_test?sslmode=disable"
# Placeholder secrets: migrate validates the whole config before it runs.
test_env := "BASE_URL=http://localhost:8080 SESSION_SECRET=test-secret-test-secret-test-secret GITHUB_CLIENT_ID=test GITHUB_CLIENT_SECRET=test OPENROUTER_API_KEY=test"

# Fast gate: format, vet and tests of the packages changed since upstream.
check:
    #!/usr/bin/env bash
    set -euo pipefail
    base=$(git merge-base HEAD @{upstream} 2>/dev/null || git rev-parse HEAD)
    changed=$( { git diff --name-only "$base"; git ls-files --others --exclude-standard; } | sort -u)
    # Deleted files stay in the index until the commit; gofmt skips them.
    gofiles=$(git ls-files --cached --others --exclude-standard '*.go' | while read -r f; do if [ -e "$f" ]; then echo "$f"; fi; done)
    unformatted=$(gofmt -l $gofiles)
    if [ -n "$unformatted" ]; then echo "gofmt: $unformatted"; exit 1; fi
    # Generated code depends on Go types anywhere in the module, so both
    # generators always run; each takes about a second.
    if [ -f drel.yaml ]; then go tool drel generate; fi
    if [ -f gx.toml ]; then go run ./cmd/gx generate && go run ./cmd/gx check; fi
    # gx build writes the stylesheet from the classes in .gx files and the
    # theme; gx generate leaves it alone, so a new class needs gx build.
    if echo "$changed" | grep -qE '\.(gx|css)$'; then
        before=$(shasum gxstyles/styles_gx.go)
        go run ./cmd/gx build -main ./cmd/briefing -o "$(mktemp -d)/briefing" >/dev/null
        if [ "$before" != "$(shasum gxstyles/styles_gx.go)" ]; then echo "gxstyles: the stylesheet was stale; gx build rewrote it, commit gxstyles/styles_gx.go"; exit 1; fi
    fi
    if echo "$changed" | grep -qE '^go\.(mod|sum)$'; then
        pkgs="./..."
    else
        pkgs=$( { echo "$changed" | grep -E '\.(go|gx)$' || true; } | while read -r f; do if [ -e "$f" ]; then echo "./$(dirname "$f")"; fi; done | sort -u | tr '\n' ' ')
    fi
    # Drop directories with no file for the default build, such as the gxdev ones.
    pkgs=$(go list -e -f '{{"{{"}}if or .GoFiles .TestGoFiles{{"}}"}}{{"{{"}}.ImportPath{{"}}"}}{{"{{"}}end{{"}}"}}' $pkgs | tr '\n' ' ')
    if [ -z "${pkgs// }" ]; then echo "check: no Go changes"; exit 0; fi
    go run ./cmd/gx lint
    go test $pkgs

# Full suite with Postgres and the race detector.
test: db
    DATABASE_URL="{{test_db}}" {{test_env}} go run ./cmd/briefing migrate
    TEST_DATABASE_URL="{{test_db}}" go test -race -count=1 ./...

# Start the local Postgres with pgvector.
db:
    docker compose up -d --wait postgres

# Run the app with live reload on http://127.0.0.1:3333, env from .env.
dev: db
    go run ./cmd/gx dev

# Run the server with the env from .env.
run:
    go run ./cmd/briefing serve
