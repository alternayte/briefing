CREATE TYPE "textsource" AS ENUM ('pending', 'none', 'extract', 'render');
CREATE TYPE "itemstatus" AS ENUM ('new', 'filtered', 'passed', 'rejected', 'kept');

CREATE TABLE "items" (
    "id" BIGSERIAL PRIMARY KEY,
    "canonical_url" text NOT NULL,
    "title" text NOT NULL,
    "snippet" text NOT NULL,
    "published_at" timestamptz,
    "text" text NOT NULL DEFAULT '',
    "text_len" integer NOT NULL DEFAULT 0,
    "text_source" "textsource" NOT NULL DEFAULT 'pending',
    "triage" jsonb,
    "embedding" vector,
    "embed_model" text NOT NULL DEFAULT '',
    "status" "itemstatus" NOT NULL DEFAULT 'new',
    "filter_reason" text NOT NULL DEFAULT '',
    "created_at" timestamptz NOT NULL DEFAULT NOW(),
    "updated_at" timestamptz NOT NULL DEFAULT NOW()
);

CREATE TABLE "sightings" (
    "id" BIGSERIAL PRIMARY KEY,
    "source_id" bigint NOT NULL REFERENCES "sources"("id") ON DELETE CASCADE,
    "external_id" text NOT NULL,
    "item_id" bigint NOT NULL REFERENCES "items"("id") ON DELETE CASCADE,
    "engagement" jsonb NOT NULL,
    "seen_at" timestamptz NOT NULL,
    "created_at" timestamptz NOT NULL DEFAULT NOW(),
    "updated_at" timestamptz NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS "idx_items_status" ON "items" ("status");
CREATE UNIQUE INDEX IF NOT EXISTS "uq_items_canonical_url" ON "items" ("canonical_url");
CREATE INDEX IF NOT EXISTS "idx_sightings_item" ON "sightings" ("item_id");
CREATE UNIQUE INDEX IF NOT EXISTS "uq_sightings_source_external" ON "sightings" ("source_id", "external_id");
