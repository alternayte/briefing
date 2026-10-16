CREATE TYPE "confidence" AS ENUM ('high', 'medium', 'low');

CREATE TABLE "concepts" (
    "id" BIGSERIAL PRIMARY KEY,
    "slug" text NOT NULL,
    "name" text NOT NULL,
    "definition" text NOT NULL,
    "embedding" vector,
    "embed_model" text NOT NULL DEFAULT '',
    "created_at" timestamptz NOT NULL DEFAULT NOW(),
    "updated_at" timestamptz NOT NULL DEFAULT NOW()
);

CREATE TABLE "review_items" (
    "id" BIGSERIAL PRIMARY KEY,
    "story_id" bigint NOT NULL REFERENCES "stories"("id") ON DELETE CASCADE,
    "revision" integer NOT NULL,
    "question" text NOT NULL,
    "choices" jsonb NOT NULL,
    "answer_index" integer NOT NULL,
    "explanation" text NOT NULL,
    "created_at" timestamptz NOT NULL DEFAULT NOW(),
    "updated_at" timestamptz NOT NULL DEFAULT NOW()
);

CREATE TABLE "story_concepts" (
    "id" BIGSERIAL PRIMARY KEY,
    "story_id" bigint NOT NULL REFERENCES "stories"("id") ON DELETE CASCADE,
    "concept_id" bigint NOT NULL REFERENCES "concepts"("id") ON DELETE CASCADE,
    "created_at" timestamptz NOT NULL DEFAULT NOW(),
    "updated_at" timestamptz NOT NULL DEFAULT NOW()
);

CREATE TABLE "summaries" (
    "id" BIGSERIAL PRIMARY KEY,
    "story_id" bigint NOT NULL REFERENCES "stories"("id") ON DELETE CASCADE,
    "revision" integer NOT NULL,
    "headline" text NOT NULL,
    "summary" text NOT NULL,
    "why_it_matters" text NOT NULL,
    "what_is_new" text NOT NULL,
    "claims" jsonb NOT NULL,
    "confidence" "confidence" NOT NULL,
    "model" text NOT NULL,
    "prompt_version" text NOT NULL,
    "publisher_count" integer NOT NULL,
    "text_items" integer NOT NULL,
    "created_at" timestamptz NOT NULL DEFAULT NOW(),
    "updated_at" timestamptz NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS "uq_concepts_slug" ON "concepts" ("slug");
CREATE UNIQUE INDEX IF NOT EXISTS "uq_review_items" ON "review_items" ("story_id", "revision", "question");
CREATE INDEX IF NOT EXISTS "idx_story_concepts_concept" ON "story_concepts" ("concept_id");
CREATE UNIQUE INDEX IF NOT EXISTS "uq_story_concepts" ON "story_concepts" ("story_id", "concept_id");
CREATE UNIQUE INDEX IF NOT EXISTS "uq_summaries_story_revision" ON "summaries" ("story_id", "revision");
