CREATE TABLE "subscriptions" (
    "id" BIGSERIAL PRIMARY KEY,
    "user_id" uuid NOT NULL REFERENCES "auth_users"("id") ON DELETE CASCADE,
    "source_id" bigint NOT NULL REFERENCES "sources"("id") ON DELETE CASCADE,
    "created_at" timestamptz NOT NULL DEFAULT NOW(),
    "updated_at" timestamptz NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX IF NOT EXISTS "uq_subscriptions" ON "subscriptions" ("user_id", "source_id");