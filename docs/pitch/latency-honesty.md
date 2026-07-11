# Pitch latency honesty (for D)

The engine build surfaced a claim in the pitch that the code cannot back up.
Judges are alumni engineers who will do the arithmetic, and our own judging
criteria reward honest benchmarks and punish unverifiable claims. This note
gives the real numbers and proposes wording that keeps the punch.

## What the engine actually does (measured)

| Alert | How it is detected | Real latency |
|-------|--------------------|--------------|
| ZONE_VIOLATION | inline, per message | single-digit ms (p99 ~3ms, live on the histogram) |
| SPOOF_TELEPORT | inline, per message | single-digit ms (same path) |
| DARK_EVENT | 1s sweep detecting absence | the silence threshold + up to 1s |

The dark threshold is 6x the vessel's expected AIS reporting interval:

- 0 to 14 kn: 10s interval, so ~60s of silence before we are certain
- 14 to 23 kn: 6s interval, so ~36s
- over 23 kn: 2s interval, so ~12s

There is no honest path to "1.2 seconds" for a dark event. That would require an
expected reporting interval near 0.2s, which no vessel class has. The whole
point of the alert is that we wait just long enough to be *statistically certain*
the vessel went dark, rather than firing on a single missed ping.

## The line to change

Current (PRD section 8, Act 2):

> "Skylight's satellite fallback sees this gap in hours. We saw it in 1.2
> seconds, and we already know Patrol B catches them in 41 minutes."

The "1.2 seconds" is the problem. The comparison and the intercept are the real
weapons; keep those.

## Proposed honest wording (pick one)

Option A (tight):

> "Skylight's satellite fallback sees this gap in hours. We flag the silence the
> instant it becomes certain, seconds after the vessel misses its reports, and
> the intercept is already solved on screen."

Option B (leans on the contrast):

> "A dark vessel does not announce itself; it just stops. Satellite fallback
> catches that in hours. We catch it in the seconds it takes to be sure the
> silence is real, and we have already computed who can reach it and when."

Option C (names the mechanism, most defensible under questioning):

> "We alert on absence: the moment a moving vessel goes past six times its
> expected report interval near a protected zone, the cone blooms and the
> intercept solves. Seconds, not the hours a satellite pass takes."

## Two more things to keep the demo verifiable

1. Quote the ETA the screen actually shows, not a fixed "41 minutes." The
   intercept ETA is computed live from patrol position and vessel drift; in the
   current scenario the nearer patrol is about 12 minutes out and the farther one
   about 51. If you want a specific number in the script, tune the patrol config
   or the scenario so the live figure lands there, then read the live figure.
2. The "milliseconds" claim is honest for zone and spoof only, and it is shown
   live on the Act 3 histogram. Keep the millisecond language attached to those
   two, never to the dark event. The histogram already displays inline and sweep
   latency separately, so the distinction is visible, not hidden.
