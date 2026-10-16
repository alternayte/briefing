# Briefing

Briefing is a learning-first home tab for tech and science news. It reads feeds, Hacker News and Bluesky accounts, drops the noise, and merges the coverage of one event into one story. Each reader gets a short ranked feed of summaries with links to their sources. Briefing then helps the reader keep what they read: it explains a story at their level, asks spaced review questions, and records the concepts they learned.

Everything a reader can do in the browser, an AI assistant can also do, through MCP. The interface and the content are in English and French.

## What it does

- Polls RSS and Atom feeds, the Hacker News front page and Bluesky author feeds.
- Drops old, empty and promotional items with rules, then asks a decision model which items report something new.
- Clusters the items about one event into one story, and counts the publishers.
- Fetches the articles, extracts their text, and writes a summary with claims that cite their sources, the key concepts, and two review questions.
- Ranks the stories for each reader from their interests and feedback, and delivers a daily budget of stories.
- Explains a story at the reader's level, schedules review questions, and shows the concepts the reader met and mastered.
- Translates the entries into the reader's language.
- Stops writing, explanations and translations when the day's model spending reaches a cap.

## Run it

Briefing is one binary in one container. It needs:

- PostgreSQL 17 with the pgvector extension. The image `pgvector/pgvector:pg17` has both.
- An [OpenRouter](https://openrouter.ai) API key. Briefing sends every model call through it.
- A GitHub OAuth app, for sign-in.

Optional:

- A [Resend](https://resend.com) API key, for sign-in links by email.
- A Cloudflare account ID and API token with the Browser Run permission, to read pages that need JavaScript.

Build the image from this repository and start it with Docker:

```sh
docker build -t briefing .
docker run -p 8080:8080 \
  -e DATABASE_URL='postgres://briefing:secret@db:5432/briefing' \
  -e BASE_URL='https://briefing.example.com' \
  -e SESSION_SECRET="$(openssl rand -hex 32)" \
  -e GITHUB_CLIENT_ID='...' \
  -e GITHUB_CLIENT_SECRET='...' \
  -e OPENROUTER_API_KEY='...' \
  -e ALLOWED_EMAILS='you@example.com' \
  briefing
```

At start, the container creates the database tables, loads the source catalog, serves the app on port 8080, and runs the background jobs. Each step is safe to repeat. A new deployment shows sign-in at `/login`, and the first stories arrive within about half an hour.

The GitHub OAuth app needs this callback URL:

```
https://briefing.example.com/api/auth/oauth/github/callback
```

### Deploy on Coolify

1. Create a PostgreSQL database. Set its image to `pgvector/pgvector:pg17`.
2. Create an application from this repository, with the Dockerfile build pack.
3. Set the port to 8080 and the health check path to `/healthz`.
4. Add the environment variables from the table below. Use the internal URL of the database for `DATABASE_URL`.
5. Set the domain of the application, and use the same address for `BASE_URL`.
6. Deploy. Then sign in with GitHub, or with an emailed link when Resend is set.

The database must allow the extension `vector`. Briefing creates it on the first start.

## Configuration

Briefing reads its configuration from the environment. It stops at start when a required value is missing, and names every missing value.

| Variable | Required | Meaning |
|---|---|---|
| `DATABASE_URL` | yes | PostgreSQL connection URL |
| `BASE_URL` | yes | The public address, such as `https://briefing.example.com` |
| `SESSION_SECRET` | yes | At least 32 random characters |
| `GITHUB_CLIENT_ID`, `GITHUB_CLIENT_SECRET` | yes | The GitHub OAuth app |
| `OPENROUTER_API_KEY` | yes | The OpenRouter key for every model call |
| `ALLOWED_EMAILS` | no | Comma-separated addresses that may sign in. Empty means anyone. |
| `ADMIN_EMAILS` | no | Comma-separated addresses that see `/admin` |
| `RESEND_API_KEY` | no | Turns on sign-in by email link |
| `EMAIL_FROM` | no | The sender of sign-in emails. Default: `Briefing <briefing@your-domain>` |
| `BRIEFING_DAILY_SPEND_USD` | no | The daily cap for model spending. Default: 1 |
| `DECIDE_API` | no | `decisions` (default) or `systemone` |
| `DECIDE_MODEL` | no | The decision model. Default: `cloudflare/clef`, or `~typesafe/jev-latest` with `systemone` |
| `EMBED_MODEL`, `EMBED_DIM` | no | The embedding model and its dimension. Default: `openai/text-embedding-3-small`, 1536 |
| `WRITE_MODEL`, `TRANSLATE_MODEL` | no | The writing and translation models. Default: `anthropic/claude-haiku-5.5` |
| `CF_ACCOUNT_ID`, `CF_API_TOKEN` | no | Turn on page rendering through Cloudflare. Set both or neither. |
| `ADDR` | no | The listen address. Default: `:8080` |

With the default models, one active reader costs a few cents a day. The admin page at `/admin` shows the day's spending by stage.

## Use it from an AI assistant

Briefing serves its tools over MCP (streamable HTTP) at `https://briefing.example.com/mcp`. Each request carries a personal access token as a bearer token.

1. Open Settings and create an access token. Copy it; Briefing shows it only once.
2. Add Briefing to your assistant as a remote MCP server, with the header `Authorization: Bearer <token>`.

   With Claude Code:

   ```sh
   claude mcp add --transport http briefing https://briefing.example.com/mcp \
     --header "Authorization: Bearer brf_..."
   ```

3. Install the Agent Skill, which teaches the assistant the workflows: the morning brief, digging into a story, a review session, and tuning interests. With Claude Code, copy the folder [`skills/briefing`](skills/briefing) to `~/.claude/skills/briefing`.

Revoke a token in Settings when you stop using it. `/llms.txt` describes the server for other assistants.

The tools are `feed_today`, `feed_search`, `feed_story`, `feed_explain`, `feed_feedback`, `learning_review_due`, `learning_review_answer`, `learning_learned`, `feed_interests_list`, `feed_interests_add`, `feed_interests_remove`, `sources_list`, `sources_subscribe`, `sources_add_rss` and `accounts_me`.

## Develop

You need Go, Docker and [just](https://github.com/casey/just).

```sh
cp .env.example .env   # then fill in the keys
just dev               # runs the app on http://127.0.0.1:3333 with live reload
just check             # formats, lints and tests the changed packages
just test              # runs every test with PostgreSQL and the race detector
```

`briefing work` runs the background jobs on their own; `briefing serve` runs only the web server; `briefing all` runs both.

## License

MIT
