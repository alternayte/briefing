CREATE TABLE "explanations" (
    "id" BIGSERIAL PRIMARY KEY,
    "user_id" uuid NOT NULL REFERENCES "auth_users"("id") ON DELETE CASCADE,
    "story_id" bigint NOT NULL REFERENCES "stories"("id") ON DELETE CASCADE,
    "revision" integer NOT NULL,
    "locale" text NOT NULL,
    "text" text NOT NULL,
    "model" text NOT NULL,
    "created_at" timestamptz NOT NULL DEFAULT NOW(),
    "updated_at" timestamptz NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX IF NOT EXISTS "uq_explanations" ON "explanations" ("user_id", "story_id", "revision", "locale");