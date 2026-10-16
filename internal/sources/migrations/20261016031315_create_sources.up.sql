CREATE TYPE "sourcekind" AS ENUM ('rss', 'hn', 'bluesky');

CREATE TABLE "sources" (
    "id" BIGSERIAL PRIMARY KEY,
    "kind" "sourcekind" NOT NULL,
    "url" text NOT NULL,
    "title" text NOT NULL,
    "tier" integer NOT NULL,
    "language" text NOT NULL,
    "enabled" boolean NOT NULL DEFAULT true,
    "poll_interval_seconds" integer NOT NULL,
    "etag" text NOT NULL DEFAULT '',
    "last_modified" text NOT NULL DEFAULT '',
    "cursor" text NOT NULL DEFAULT '',
    "last_ok_at" timestamptz,
    "last_error" text NOT NULL DEFAULT '',
    "created_at" timestamptz NOT NULL DEFAULT NOW(),
    "updated_at" timestamptz NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS "uq_sources_kind_url" ON "sources" ("kind", "url");
