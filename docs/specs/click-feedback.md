# Click feedback

## What it does
Every click on a button or a link shows that the app received it. While the answer is in flight, the clicked button or link turns muted and pulses, and a thin bar runs at the top of the page during a page change. A click that the server answers within about 100 ms shows nothing extra. A failed click shows an error toast.

## Decisions
- The pending state lives in gx, not in Briefing — every gx app has the gap, and AGENTS.md sends gx defects to its repo.
- While a gx form submits, the runtime sets `aria-busy="true"` on the form and on its submitter, and ignores a second submit of the same form until the answer — the reader sees the click land, and a double click does not send twice.
- During a client navigation, the runtime sets `aria-busy="true"` on the clicked link and on `<html>`; a new navigation replaces the pending one — one rule covers links and forms.
- The runtime sets the pending state only after about 100 ms, and clears it when the answer arrives or fails — fast clicks show no flash.
- Background requests, such as the one-minute check for new stories, set no pending state — only a reader's click shows feedback.
- Briefing styles `[aria-busy="true"]` on buttons and links as muted with a slow pulse, and `html[aria-busy="true"]` as a thin accent bar at the top — the theme owns the look; the page needs no JS of its own.
- The pulse and the bar stop under `prefers-reduced-motion`; the muted state stays — the signal does not depend on motion.
- Briefing takes the gx release with this change and adds only the theme rules — the work in Briefing waits for the gx release.

## Out
- A row in place of a card that leaves Today.
- Optimistic changes to the pressed state before the server answers.
- Changes to how fast the server answers.

## How I know it works
- An issue on alternayte/gx describes the pending state for forms and navigations, and a gx release closes it.
- With the network throttled to "Slow 4G", a click on "Useful" mutes and pulses that button until the card updates.
- With the same throttle, a click on "Settings" shows the bar at the top and mutes the link until the page changes.
- Without throttling, a click on "Useful" shows no pulse.
- A double click on "Done" sends one request.
- The one-minute check for new stories shows no bar.
- With reduced motion on, a pending button is muted and does not pulse.
