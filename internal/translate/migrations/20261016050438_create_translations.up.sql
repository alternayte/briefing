CREATE TYPE "translationkind" AS ENUM ('story', 'article');

CREATE TABLE "translations" (
    "id" BIGSERIAL PRIMARY KEY,
    "kind" "translationkind" NOT NULL,
    "object_id" bigint NOT NULL,
    "revision" integer NOT NULL,
    "locale" text NOT NULL,
    "fields" jsonb NOT NULL,
    "model" text NOT NULL,
    "requested_by" uuid REFERENCES "auth_users"("id") ON DELETE SET NULL,
    "created_at" timestamptz NOT NULL DEFAULT NOW(),
    "updated_at" timestamptz NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS "uq_translations" ON "translations" ("kind", "object_id", "revision", "locale");
