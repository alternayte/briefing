# Briefing

## What this is
Briefing is a learning-first home tab. It ingests tech and science sources, merges duplicate coverage into stories, and shows each user a ranked feed of source-backed summaries with spaced review.

## Run
No code exists yet. Add the run command here when the first entry point lands.

## Test
No code and no test commands exist yet.

## Stack rules
- UI is Gx with Datastar. Every user capability is a Gx action; actions marked as tools also serve MCP and WebMCP.
- HTTP uses the standard library ServeMux. No router library.
- Database access is drel on Postgres 17 with pgvector. Auth is auth-all. Background jobs are River.
- All model calls go through OpenRouter.
- drel, Gx and auth-all belong to the maintainer. Report a defect in one of them as an issue on its repo. Never fork, vendor or replace them.
- Every user-visible string goes through the i18n catalog, in English and French.

## Domain words
- source: a feed the app polls, such as an RSS feed, Hacker News or a Bluesky author.
- item: one article, identified by its canonical URL.
- sighting: one appearance of an item in one source, with engagement data.
- story: a cluster of items that report the same event.
- revision: a new version of a story's summary after its coverage changes.
- triage: the decision-model pass that keeps or drops an item.
- concept: a reusable idea a reader needs to understand a story.
- review item: a multiple-choice question for spaced review of a story.
- feed entry: one story in one user's feed, with its score and state.
- link card: a feed entry shown without a summary, because the story has too little text.
