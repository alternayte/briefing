CREATE TYPE "eventkind" AS ENUM ('opened', 'useful', 'knew', 'irrelevant', 'misleading', 'explained', 'reviewed_correct', 'reviewed_wrong');

CREATE TABLE "learning_events" (
    "id" BIGSERIAL PRIMARY KEY,
    "user_id" uuid NOT NULL REFERENCES "auth_users"("id") ON DELETE CASCADE,
    "story_id" bigint REFERENCES "stories"("id") ON DELETE CASCADE,
    "concept_id" bigint REFERENCES "concepts"("id") ON DELETE CASCADE,
    "kind" "eventkind" NOT NULL,
    "payload" jsonb NOT NULL,
    "created_at" timestamptz NOT NULL DEFAULT NOW(),
    "updated_at" timestamptz NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS "idx_learning_events_user_kind" ON "learning_events" ("user_id", "kind", "created_at");
