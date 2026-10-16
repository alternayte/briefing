CREATE TABLE "write_attempts" (
    "id" BIGSERIAL PRIMARY KEY,
    "story_id" bigint NOT NULL REFERENCES "stories"("id") ON DELETE CASCADE,
    "day" date NOT NULL,
    "attempts" integer NOT NULL,
    "created_at" timestamptz NOT NULL DEFAULT NOW(),
    "updated_at" timestamptz NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX IF NOT EXISTS "uq_write_attempts" ON "write_attempts" ("story_id", "day");