# LedgerDrill

**Does your payment app pay twice when confirmation disappears?** LedgerDrill injects repeatable connection failures around real Stellar testnet payments and compares naive retry logic with hash-based reconciliation.

**Live demo: https://ld.kennyy.tech/**

**Source: https://github.com/M3264/ledgerdrill**

The v1 demo uses XLM, funded with Friendbot. It does not move real funds or ask for customer private keys. It is a diagnostic harness, not a wallet, contract debugger, or security certification.

## Run

Requirements: Go 1.27+, Node 22+, npm, network access to Stellar testnet Horizon and Friendbot.

```sh
./scripts/run.sh
```

Open http://127.0.0.1:8098. Select **Send 1 XLM** on the payout challenge. Two fresh test accounts are created, a randomized fault interrupts confirmation, and you choose whether to send a fresh payment or reconcile the original. The result is verified against the ledger. **Failure lab** still compares both reference strategies: pick a scenario and select **Run live comparison**, which creates three fresh test accounts. Account setup and settlement usually take tens of seconds and depend on public testnet availability.

To run an already-built binary:

```sh
./ledgerdrill -addr 127.0.0.1:8098 -data ./data
```

## Payout challenge

The homepage pauses after one actual payment attempt. Its fault is selected randomly from the three scenarios, and revealed after your recovery choice. A fresh-payment choice may settle once if the original never reached Stellar, or twice if it did. Hash reconciliation retains the original signed envelope. The result always comes from independent ledger verification; it is never a preset score.

The initiating browser holds a random decision capability in sessionStorage, separate from transaction signing keys. Refreshing restores it. Only one recovery can be accepted. The decision window is two minutes; on expiry, the server verifies the original attempt but sends no additional payment. Challenge reports contain no capability or signing key. The shared lab permits one active payout or comparison at a time. Completed payouts appear in Run history.

## Scenarios

| Scenario | Injected connection behavior | Expected confirmed payments: naive / reconciled |
| --- | --- | --- |
| Before submission | Reject first attempt before forwarding | 1 / 1 |
| Lost confirmation | Submit successfully, replace response with HTTP 504 | 2 / 1 |
| Delayed lookup | Lose response, hide first two lookups of the confirmed hash | 2 / 1 |

The naive client deliberately demonstrates an incorrect recovery policy: after an error it loads the source's current sequence and creates a **new** transaction. The reconciled client retains its original signed envelope and hash, checks that hash, and only resubmits the same envelope if confirmation remains unavailable. Resubmitting the exact envelope cannot turn into a second unique transaction.

No faults are injected into Stellar consensus or other applications. Each fault belongs to one local proxy session. A reference-client run is illustrative evidence of that scenario, not a universal assessment of your application.

## Test your own integration

Create an isolated fault session:

```sh
curl -sS http://127.0.0.1:8098/api/sessions \
  -H 'Content-Type: application/json' \
  -d '{"scenario":"lost_confirmation"}'
```

Use the returned `proxy_path` as your client's Horizon base URL, for example `http://127.0.0.1:8098/horizon/SESSION_ID/`. Keep signing for the **Test SDF Network ; September 2015** passphrase. Sign in your own client; LedgerDrill never needs its secret key. Use disposable testnet accounts.

Supported endpoints are `GET /`, `GET /fee_stats`, `GET /accounts/:address`, account transactions/payments/operations, `GET /transactions/:hash`, and `POST /transactions` with a form-encoded `tx`. This is a limited classic Horizon proxy. It does not implement SSE, arbitrary Horizon resources, Soroban RPC, or path-payment discovery. Pagination links in upstream responses remain Horizon links; those requests bypass fault injection. The delayed-lookup fault only targets hashes already confirmed through that session.

`GET /api/sessions/:id` returns the proxy trace. `attempts` counts valid signed-envelope submissions, including injected pre-broadcast rejections. Transactions are deduplicated by their testnet hash. For an external session, confirmation reflects observed upstream responses, not a complete independent audit; a timeout from Horizon itself may remain uncertain. Reports do not infer intent from two different transaction hashes.

## API

- `POST /api/challenges`: starts a payout; returns its report and a one-time-issued browser decision capability.
- `GET /api/challenges`: saved public payout reports.
- `GET /api/challenges/:id`: payout state; `.../report` exports JSON.
- `POST /api/challenges/:id/decision {"key":"...","action":"retry|reconcile"}`: accepts one authorized recovery decision.
- `GET /api/config`: scenarios, busy state, authentication requirement.
- `POST /api/runs {"scenario":"..."}`: starts one asynchronous paired reference comparison; HTTP 409 if another run is active.
- `GET /api/runs`: most recent 100 reports.
- `GET /api/runs/:id`: one report.
- `GET /api/runs/:id/report`: JSON attachment.
- `POST /api/sessions {"scenario":"..."}`: new proxy session.
- `GET /api/sessions/:id`: trace and observed transaction evidence.
- `GET /healthz`: process health; not a claim about upstream availability.

Reference comparisons verify every unique submitted hash directly against Horizon, bypassing the fault proxy, decode the ledger's transaction envelope, and count successful XLM payments to the intended recipient. Failed verification produces an **inconclusive** result, not a pass. Received amounts are summed from verified transaction operations; they are not inferred from a changing wallet balance.

## Persistence and restart

SQLite stores run snapshots, proxy fault counters, attempts, and transaction hashes. The first schema migration runs transactionally; unknown newer schema versions are rejected. A submission's hash is written before broadcast. Private fixture keys live only in memory and are discarded after a run.

Interrupted comparisons and payout challenges are marked **interrupted** on startup. They are never resumed with new payments. Inspect their original hashes, client timeline and associated proxy session for settlement evidence. Proxy sessions retain their one-shot fault state across restart. SQLite data must live on a persistent volume for container deployment. Back up the database using SQLite's backup API or stop the service before copying its database/WAL files.

## Access and deployment

The default bind address is loopback. Binding to a non-loopback interface requires `LEDGERDRILL_TOKEN` of at least 24 characters. Supply `Authorization: Bearer ...` for run/session/proxy API requests. The dashboard accepts this token and stores it in sessionStorage for that tab. Do not put the token in a URL. Cross-origin API requests are rejected; use a backend client or the same-origin dashboard.

```sh
export LEDGERDRILL_TOKEN='set-a-long-random-token-outside-source'
./ledgerdrill -addr 0.0.0.0:8098 -data /data
```

For public hosting, use TLS at the reverse proxy, configure the secret through the host's secret manager and mount persistent storage. `Dockerfile` and `fly.toml` are provided; choose a unique Fly app name and provision a `ledgerdrill_data` volume before deploying. The existing host serves `ld.kennyy.tech` through Nginx and a restartable `ledgerdrill.service`; host configurations live in `deploy/`. The public source repository is [M3264/ledgerdrill](https://github.com/M3264/ledgerdrill).

The hosted demo permits public access to disposable testnet reports and reference runs. Nginx applies a shared budget of 10 POST requests per minute with a burst allowance of 10; GET polling does not consume it. For sustained app-integration testing, use a local instance. The origin remains bound to loopback. The HTTPS certificate renews through Certbot, with a deployment hook that reloads Nginx.

## Verification

```sh
go test ./...
go vet ./...
go build ./...
npm --prefix frontend run check
npm --prefix frontend run build
python3 scripts/live-check.py
```

`live-check.py` runs real testnet payments and writes JSON reports under `docs/evidence/`. These reports contain only disposable testnet public addresses, hashes and diagnostic traces. Tests use controlled HTTP upstreams to verify that pre-broadcast failures never relay, confirmed replies can be hidden, retrying one envelope retains one hash, faults survive restart, and rejected transactions never become successful evidence.

## Project layout

- `cmd/ledgerdrill`: server lifecycle, safe bind defaults.
- `internal/lab`: fault proxy, reference clients, independent verification.
- `internal/store`: SQLite migration and journal.
- `internal/server`: API, access checks, embedded dashboard.
- `frontend`: SvelteKit static dashboard; build output is embedded into the Go binary.
- `scripts`: one-command startup and live acceptance proof.
- `RESEARCH.md`: current hackathon landscape and proposed differentiation.

## Sources

[Stellar error handling](https://developers.stellar.org/docs/data/apis/horizon/api-reference/errors/error-handling) explains ambiguous timeouts and unchanged-envelope retries. [MergePay issue #101](https://github.com/mergepay/mergepay-api/issues/101) documents this recovery problem in a Stellar payment integration.

The checked-in [verification record](docs/VERIFICATION.md) includes live testnet evidence and desktop/mobile screenshots from the first working build.
