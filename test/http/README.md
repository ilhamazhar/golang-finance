# `.http` e2e collection

Plain-text HTTP requests covering **every endpoint** as one ordered scenario,
with register→login→JWT chaining, captured `order_ref`, and JS assertions. No
Node, no npm, no `inso` — runs in the IDE or via a standalone CLI.

```
test/http/
├── azhar.http                              # the scenario (run top to bottom)
├── http-client.env.json                    # non-secret vars (local / staging)
├── http-client.private.env.json.example    # template for the secret callback token
└── README.md
```

## Setup

1. Copy the secret template (the real file should be gitignored):
   ```
   cp http-client.private.env.json.example http-client.private.env.json
   ```
   Put your real `callback_token` (server's `XENDIT_CALLBACK_TOKEN`) in it.
2. Edit `http-client.env.json` → set `base_url` (and the staging host if used).
3. No DB seeding required — the scenario calls `/auth/register` first with a
   unique email per run (`test-{{$timestamp}}@azhar.test`), so each run starts
   from a clean user and ends by deleting that user. Re-runnable as-is.

## Run

**GoLand / IntelliJ (built-in HTTP Client):**
open `azhar.http`, pick the env (top-right dropdown → `local`), then either
run each request with the gutter arrow top-to-bottom, or use **Run all requests
in file**. `register` must run first so `{{user_id}}` / `{{registered_email}}`
are captured, then `login` for `{{access_token}}`.

**VS Code:** install the **REST Client** extension (`humao.rest-client`). It
runs the requests and chaining, but the `> {% client.test(...) %}` assertion
blocks are JetBrains-only — in VS Code they're ignored (requests still work).

**Headless / CI — `ijhttp` (standalone, no Node):**
```
ijhttp test/http/azhar.http \
  --env local \
  --env-file test/http/http-client.env.json \
  --private-env-file test/http/http-client.private.env.json \
  --report
```
`--report` writes JUnit XML for the pipeline. Download `ijhttp` from JetBrains
(IntelliJ HTTP Client CLI) or use its Docker image — both are self-contained.

## Notes / limits

- **Payments need Xendit test keys.** The `Create QRIS` request calls Xendit;
  without a key it returns 502 and the PAID transition can't run. To exercise
  the webhook state machine offline, pre-seed a `PENDING` payment row and set
  `order_ref` manually in `http-client.env.json` instead of capturing it.
- **No terminal-state guard on the webhook.** Unlike DAMRI, this service's
  `HandleWebhook` does *not* protect terminal statuses — a late `payment.expired`
  after `payment.succeeded` will currently flip PAID → EXPIRED. The scenario
  therefore stops at the PAID assertion and does not send a late-expired event.
  If you add a guard, add a late-expired step here too.
- **Bad callback token returns 200, not 401.** The webhook handler logs the
  verify error and still 200s (Xendit best-practice — don't trigger retries).
  The negative test asserts 200 by design; flip it if you change that contract.
- **Rate limits** (`/auth/*` IP-based, `/api/*` user-based) can bite during
  rapid re-runs locally. Raise `RATE_LIMIT_AUTH_MAX` / `RATE_LIMIT_API_MAX`
  in `.env`, or wait out the period.

## Why this over Insomnia/`inso`

Versioned plain text in the repo, diff-able in PRs, runs in the IDE you already
use for Go, and the CI runner is a single binary — no npm dependency tree and
no Node-version deprecation warnings.
