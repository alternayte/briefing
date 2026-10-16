CREATE TABLE "profiles" (
    "id" uuid PRIMARY KEY REFERENCES auth_users(id) ON DELETE CASCADE,
    "display_name" text NOT NULL,
    "locale" text NOT NULL,
    "daily_budget" integer NOT NULL,
    "onboarded_at" timestamptz,
    "created_at" timestamptz NOT NULL DEFAULT NOW(),
    "updated_at" timestamptz NOT NULL DEFAULT NOW()
);
