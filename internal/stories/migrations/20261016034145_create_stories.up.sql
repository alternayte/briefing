CREATE TYPE "storystatus" AS ENUM ('open', 'closed');

CREATE TABLE "stories" (
    "id" BIGSERIAL PRIMARY KEY,
    "revision" integer NOT NULL DEFAULT 0,
    "centroid" vector,
    "opened_at" timestamptz NOT NULL,
    "last_item_at" timestamptz NOT NULL,
    "publisher_count" integer NOT NULL DEFAULT 1,
    "significance" double precision NOT NULL DEFAULT 0,
    "learning" double precision NOT NULL DEFAULT 0,
    "sensational" double precision NOT NULL DEFAULT 0,
    "category" text NOT NULL,
    "status" "storystatus" NOT NULL DEFAULT 'open',
    "created_at" timestamptz NOT NULL DEFAULT NOW(),
    "updated_at" timestamptz NOT NULL DEFAULT NOW()
);

CREATE TABLE "story_items" (
    "id" BIGSERIAL PRIMARY KEY,
    "story_id" bigint NOT NULL REFERENCES "stories"("id") ON DELETE CASCADE,
    "item_id" bigint NOT NULL REFERENCES "items"("id") ON DELETE CASCADE,
    "created_at" timestamptz NOT NULL DEFAULT NOW(),
    "updated_at" timestamptz NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS "idx_stories_open_recent" ON "stories" ("last_item_at", "status");
CREATE UNIQUE INDEX IF NOT EXISTS "uq_story_items" ON "story_items" ("story_id", "item_id");
CREATE UNIQUE INDEX IF NOT EXISTS "uq_story_items_item" ON "story_items" ("item_id");
