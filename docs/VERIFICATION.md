# LedgerDrill verification

Executed against Stellar testnet on 2026-10-03. The reference payment intent was 1 XLM per client. All three comparisons finished with ledger evidence verified directly through Horizon, outside the fault proxy.

| Fault | Naive attempts / settlements / XLM | Reconciled attempts / settlements / XLM | Report |
| --- | --- | --- | --- |
| Before submission | 2 / 1 / 1 | 2 / 1 / 1 | [JSON](evidence/before_submit-63db7f847cd56bbf72f5daee208ce058.json) |
| Lost confirmation | 2 / 2 / 2 | 1 / 1 / 1 | [JSON](evidence/lost_confirmation-3bda4f900e20c3726465fbe7660dcd70.json) |
| Delayed lookup | 2 / 2 / 2 | 1 / 1 / 1 | [JSON](evidence/delayed_lookup-d1620fcc97291238ac8b1ddede164a28.json) |

The before-submission run demonstrates why submission attempts and unique settlements differ: resubmitting the original envelope produces one successful hash. The other runs demonstrate that rebuilding a fresh transaction after uncertainty can pay twice. Testnet evidence may disappear after a network reset; these JSON snapshots preserve what was observed at execution time.

Go unit tests and vet passed. Svelte check reported zero errors and warnings; the static dashboard and embedded Go binary built successfully. Controlled upstream tests cover forwarding, hidden replies, hash deduplication, restart behavior, invalid envelopes and access/origin checks.

Deployment configuration is supplied for Docker and Fly.io; the container image has not been built on this disk-constrained host. The public deployment on the existing host is https://ld.kennyy.tech/, with Nginx, HTTPS and a systemd service.

The browser check also launched a fresh before-submission comparison through the dashboard, watched it finish, and downloaded [the matching JSON](evidence/ui-before_submit-ebb1e0ebf1d2cd4d3db0483709db0e76.json). Saved reports remained available after restarting the server. Desktop (1440px) and mobile (390px) checks covered navigation, reports, history, session creation/restoration, keyboard entry, and an injected API error with dismissal. No browser page errors or page-level horizontal overflow remained. The history table scrolls within its own container on mobile. [Browser results](browser-verification.json) and [screenshots](screenshots/) are saved alongside the ledger evidence.

## Public deployment check

Verified https://ld.kennyy.tech/ on 2026-10-03: HTTPS homepage, health and configuration returned 200; HTTP redirects to HTTPS. Desktop and mobile browsers loaded the app without page errors, navigated between tabs, and created proxy sessions whose URLs use the public HTTPS hostname. The systemd service is active and enabled for boot, with SQLite data retained in the existing data directory. Certbot renewal is enabled and its LedgerDrill hook reloads Nginx.

A live lost-confirmation comparison launched through the public domain completed with **2 naive settlements and 1 reconciled settlement**: [public execution report](evidence/public/lost_confirmation-a3936e8de288883ca511d65ec49a2576.json). The acceptance script identifies itself with a LedgerDrill User-Agent because the domain's Cloudflare policy blocks Python's default User-Agent.

## Interactive payout challenge

The homepage now starts one real payout and waits for the initiating browser to choose recovery after an unrevealed randomized failure. Browser tests exercised fresh-payment and original-hash recovery, refresh restoration, history reopening, evidence export, foreign-browser rejection (403) and duplicate-decision rejection (409). Unit tests also cover expired decisions and restart without replay. No browser page errors or desktop/mobile horizontal overflow were observed.

Both initial interactive tests drew the before-broadcast scenario and settled once; their reports are under [challenge evidence](evidence/challenge/). The existing paired comparison passed all three scenarios again after sharing its recovery and verification logic with the challenge; see [regression evidence](evidence/challenge-regression/).

A public browser run at https://ld.kennyy.tech/ drew lost confirmation. Choosing a fresh payment produced **two independently verified settlements totaling 2 XLM**: [public challenge report](evidence/challenge-public/d7f8a7b64e50508665b971a6892c5332.json). [Public browser checks](challenge-public-verification.json) also verified access to the diagnostic lab, history and integration screen. Updated screenshots live in [challenge screenshots](screenshots/challenge/).
