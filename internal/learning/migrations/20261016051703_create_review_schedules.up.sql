CREATE TABLE "review_schedules" (
    "id" BIGSERIAL PRIMARY KEY,
    "user_id" uuid NOT NULL REFERENCES "auth_users"("id") ON DELETE CASCADE,
    "review_item_id" bigint NOT NULL REFERENCES "review_items"("id") ON DELETE CASCADE,
    "due_at" timestamptz NOT NULL,
    "interval_days" integer NOT NULL,
    "streak" integer NOT NULL DEFAULT 0,
    "last_result" text NOT NULL DEFAULT '',
    "created_at" timestamptz NOT NULL DEFAULT NOW(),
    "updated_at" timestamptz NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS "idx_review_schedules_due" ON "review_schedules" ("user_id", "due_at");
CREATE UNIQUE INDEX IF NOT EXISTS "uq_review_schedules" ON "review_schedules" ("user_id", "review_item_id");