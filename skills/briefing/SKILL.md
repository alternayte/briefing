---
name: briefing
description: Use the reader's Briefing through its MCP tools. Briefing reads tech and science sources, keeps what matters to the reader, and helps them remember it. Use this skill when the reader asks for their news, a morning brief, what is new about a topic, an explanation of a story, a review session, or a change to what they follow.
---

# Briefing

Briefing is the reader's news brief. It ranks tech and science stories for this reader, writes a short entry for each story, and asks review questions so the reader remembers what they read. Every tool acts for one reader, with the reader's personal access token.

Answer in the reader's language. `feed_today` and `accounts_me` return it. The tools return text in that language when a translation exists.

Show the source links. Every story has them. Do not add facts that the tools did not return.

## Morning brief

1. Call `feed_today`.
2. Show the stories under `today` in order. Give each headline, one or two sentences of the summary, and the publishers.
3. Mention the stories under `more` only by headline, and only when the reader asks for more.
4. When `reviews_due` is more than 0, offer a review session at the end.

Keep the brief short. The reader reads it at the start of the day.

## Digging into a story

1. Call `feed_story` with the story id. This counts as the reader opening the story.
2. Give what is new and why it matters. Cite each claim with its source URLs.
3. When the reader asks what the story means, call `feed_explain`. It writes an explanation at the reader's level and in their language. Use its text as it is.
4. When the reader reacts, record it with `feed_feedback`:
   - `useful`: the story was worth reading. This also schedules its review questions.
   - `knew`: the reader already knew it. Similar stories then rank lower.
   - `irrelevant`: the story is not for this reader.
   - `misleading`: the entry misstates its sources.

To find stories on a topic, call `feed_search` with a short query. Add `from` and `to` (YYYY-MM-DD) when the reader names a period.

## Review session

1. Call `learning_review_due`.
2. Ask one question at a time. Show the choices with letters, and do not show the answer.
3. Send the reader's choice to `learning_review_answer` as an index from 0.
4. Tell the reader if the answer is correct, then give the explanation from the result.
5. Continue until no question is due, or the reader stops.

At most five questions are due each day. Do not invent questions.

To show what the reader has learned, call `learning_learned`. Group the concepts by state: mastered, familiar, met.

## Tuning interests

1. Call `feed_interests_list` to see the topics the reader follows and the topics they leave out.
2. To follow a topic, call `feed_interests_add` with the reader's words. To leave a topic out, set `negative` to true.
3. To remove a topic, call `feed_interests_remove` with its id. Confirm with the reader first.
4. To change sources, call `sources_list`, then `sources_subscribe` with the source id and `on`.
5. To add a blog or site, call `sources_add_rss` with its RSS or Atom address.

The feed ranks again after each change. New stories arrive within the hour.
