# Done, help and dates in the feed

## What it does
A reader takes a feed entry off Today with Done, Clear read, "Knew it" or "Not relevant". The entry moves to a collapsed "Done today" section, and the reader can undo it. A pressed feedback button undoes its feedback on a second click. "Misleading" hides the summary for that reader and lists the flag on /admin. Each card shows how fresh its coverage is, and the day of the feed follows the reader's timezone. A "?" button explains the four options, Done and the daily budget.

## Decisions
- A new entry state `done` holds every entry off Today — one state covers Done, Clear read, "Knew it" and "Not relevant"; `seen` keeps its meaning.
- "Knew it" and "Not relevant" set `done`; "Useful" and "Misleading" leave the entry on Today — the reader may still read those stories.
- Done is a button on each Today card; Clear read sets every `seen` entry of today to `done` — the reader clears one card or all read cards.
- `done` entries show in a collapsed "Done today" section at the bottom of Today — the reader can undo a mistake.
- A `done` entry still counts toward the daily budget; Today does not refill — the budget limits daily reading.
- A `done` entry with a new revision returns to Today marked "Updated", unless it has a knew or irrelevant event — Done means "read this version"; "Knew it" and "Not relevant" judge the topic.
- "Misleading" shows the entry as a link card with "You flagged this summary" for that reader — the reader does not keep reading a summary they judged wrong.
- /admin lists every misleading flag with the story and the reader — the maintainer can review the summaries.
- A pressed feedback button deletes its event on a second click and reverses its effect — the pressed state is the truth.
- Undo of "Useful" removes the story's unanswered review items; undo of an interest weight step applies the opposite step when no other opinion event remains — `rate` applies the step only once.
- Undo of "Knew it" or "Not relevant" returns the entry to Today; undo of "Misleading" shows the summary again — undo restores the state before the click.
- A "Done today" entry set by Done or Clear read has a Restore button — Restore returns it to `seen`.
- "Knew it" or "Not relevant" on a "More" card sets `done`; the entry gets no delivered_on and never reaches Today — a story marked "Knew it" or "Not relevant" does not come back the next day.
- "More" cards have no Done button; Done and Clear read act on Today only — "More" is not the day's reading.
- A card shows the relative time of last_item_at first in its meta line: "3 h ago", "yesterday", then a short date — this tells the reader how fresh the information is.
- The story page shows "First reported <opened_at> · updated <last_item_at>" in full — the page has room for both dates.
- The profile stores a timezone; the browser sends it when the profile has none; Settings changes it; the default is UTC — every local date needs a zone.
- Today, the daily budget reset, the daily review cap and all shown dates use the reader's local date — Today rolls over at the reader's midnight, not UTC midnight.
- One "?" button at the top of Today, next to Clear read, and one next to the feedback buttons on the story page open a native `<dialog>` — one explanation stays correct and works without JS.
- The dialog explains each option and its effect, undo by second click, Done, Clear read, "Done today", the daily budget and the reset time — these are every rule that changes what Today shows.
- Every new string goes through the i18n catalog in English and French — AGENTS.md requires it.
- `feed_feedback` gains `undo: true`; the UI toggle sends it when the button is pressed — an agent states the result it wants, so a retry is safe.
- New tools `feed_done` (story id, or `all_seen: true`) and `feed_restore` (story id) are Gx actions marked as tools — every capability is a Gx action.
- `feed_today` returns `done` entries with a done flag — an assistant sees the same feed as the page.

## Out
- Hacker News and engagement attribution on cards.
- A rewrite of a summary flagged misleading.
- Refill of Today after Done.
- A Done button on "More" cards.

## How I know it works
- Click "Not relevant" on a Today card: the card moves to "Done today" without a page reload, and the scroll position stays.
- Click the pressed "Not relevant" again: the card returns to Today, and the interest weight in the database is back to its old value.
- Click Clear read: every greyed card moves to "Done today"; unread cards stay.
- A story marked Done gets a new revision: it returns to Today marked "Updated". A story marked "Knew it" stays in "Done today".
- Click "Misleading": the card shows the link card with "You flagged this summary", and /admin lists the flag.
- Click "Not relevant" on a "More" card, then run the ranker the next day: the story is not delivered.
- With timezone Europe/Paris, an entry delivered at 23:30 UTC belongs to the next local day.
- Cards show "3 h ago" in English and "il y a 3 h" in French; the story page shows both full dates.
- Click "?": the dialog opens with the explanation in the reader's locale; it opens with JS turned off.
- Call `feed_feedback` with `undo: true` twice: the second call changes nothing.
