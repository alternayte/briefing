CREATE TYPE "entrystate" AS ENUM ('candidate', 'delivered', 'seen');

CREATE TABLE "feed_entries" (
    "id" BIGSERIAL PRIMARY KEY,
    "user_id" uuid NOT NULL REFERENCES "auth_users"("id") ON DELETE CASCADE,
    "story_id" bigint NOT NULL REFERENCES "stories"("id") ON DELETE CASCADE,
    "relevance" double precision NOT NULL,
    "excluded" double precision NOT NULL,
    "score" double precision NOT NULL,
    "reason" text NOT NULL DEFAULT '',
    "shown_revision" integer NOT NULL DEFAULT 0,
    "delivered_on" date,
    "state" "entrystate" NOT NULL DEFAULT 'candidate',
    "created_at" timestamptz NOT NULL DEFAULT NOW(),
    "updated_at" timestamptz NOT NULL DEFAULT NOW()
);

CREATE TABLE "interests" (
    "id" BIGSERIAL PRIMARY KEY,
    "user_id" uuid NOT NULL REFERENCES "auth_users"("id") ON DELETE CASCADE,
    "text" text NOT NULL,
    "negative" boolean NOT NULL DEFAULT false,
    "weight" double precision NOT NULL DEFAULT 1,
    "embedding" vector,
    "embed_model" text NOT NULL DEFAULT '',
    "created_at" timestamptz NOT NULL DEFAULT NOW(),
    "updated_at" timestamptz NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS "idx_feed_entries_user_state" ON "feed_entries" ("user_id", "state");
CREATE UNIQUE INDEX IF NOT EXISTS "uq_feed_entries" ON "feed_entries" ("user_id", "story_id");
CREATE INDEX IF NOT EXISTS "idx_interests_user" ON "interests" ("user_id");
