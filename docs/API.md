# Azhar Finance — API Documentation

REST API for a Syariah (Islamic) financing service. It provides user
authentication with email verification, QRIS payments via Xendit, and Murabahah
(cost-plus sale) financing with an upfront installment schedule.

- **Base URL (local):** `http://localhost:8080`
- **Module:** `github.com/ilhamazhar/golang-gpt`
- **Format:** JSON request and response bodies (`Content-Type: application/json`)
- **Money:** all amounts are integer **minor units** (e.g. rupiah), never floats.

---

## Table of contents

- [Conventions](#conventions)
  - [Response envelope](#response-envelope)
  - [Pagination](#pagination)
  - [Authentication](#authentication)
  - [Rate limiting](#rate-limiting)
  - [Errors & validation](#errors--validation)
- [Health](#health)
  - [`GET /health`](#get-health)
- [Auth endpoints](#auth-endpoints)
  - [`POST /auth/register`](#post-authregister)
  - [`POST /auth/login`](#post-authlogin)
  - [`GET /auth/verify`](#get-authverify)
  - [`POST /auth/resend-verification`](#post-authresend-verification)
  - [`POST /auth/forgot-password`](#post-authforgot-password)
  - [`POST /auth/reset-password`](#post-authreset-password)
  - [`POST /auth/refresh`](#post-authrefresh)
  - [`POST /auth/logout`](#post-authlogout)
- [Current user (`/api/me`)](#current-user-apime)
  - [`GET /api/me/`](#get-apime)
  - [`PUT /api/me/password`](#put-apimepassword)
- [Payments](#payments)
  - [`POST /api/payments/qris`](#post-apipaymentsqris)
  - [`GET /api/payments/:order_ref`](#get-apipaymentsorder_ref)
- [Financings (Murabahah)](#financings-murabahah)
  - [`POST /api/financings`](#post-apifinancings)
  - [`GET /api/financings`](#get-apifinancings)
  - [`GET /api/financings/:id`](#get-apifinancingsid)
  - [`POST /api/financings/:id/sign`](#post-apifinancingsidsign)
  - [`POST /api/financings/:id/installments/:no/pay`](#post-apifinancingsidinstallmentsnopay)
- [Users](#users)
  - [`GET /api/users`](#get-apiusers)
  - [`GET /api/users/:id`](#get-apiusersid)
  - [`PUT /api/users/:id`](#put-apiusersid)
  - [`DELETE /api/users/:id`](#delete-apiusersid)
- [Webhooks](#webhooks)
  - [`POST /webhooks/xendit`](#post-webhooksxendit)
- [Enumerations](#enumerations)
- [Configuration](#configuration)

---

## Conventions

### Response envelope

Every endpoint (except the email-verify redirect and the Xendit webhook) returns
the same JSON envelope.

**Success:**

```json
{
  "success": true,
  "message": "Human-readable message",
  "data": { }
}
```

**Failure:**

```json
{
  "success": false,
  "message": "What went wrong",
  "errors": null
}
```

`data` and `errors` are omitted when empty.

### Pagination

List endpoints accept `page` and `limit` query parameters and return a
`pagination` block alongside the envelope.

| Query param | Default | Bounds                  |
|-------------|---------|-------------------------|
| `page`      | `1`     | `>= 1`                  |
| `limit`     | `10`    | `1`–`100` (clamped)     |

```json
{
  "success": true,
  "message": "Users retrieved",
  "data": [ ],
  "pagination": {
    "page": 1,
    "limit": 10,
    "total_items": 42,
    "total_pages": 5
  }
}
```

### Authentication

Protected routes live under `/api/*` and require a JWT **access token** in the
`Authorization` header:

```
Authorization: Bearer <access_token>
```

Obtain tokens via [`POST /auth/login`](#post-authlogin). Access tokens are
short-lived (default 24h); use [`POST /auth/refresh`](#post-authrefresh) with the
refresh token (default 7 days) to obtain a new pair.

The `curl` examples for protected routes below assume the access token is in a
shell variable:

```bash
export TOKEN="eyJ..."   # access_token from the login response
```

Missing/invalid token → `401 Unauthorized`:

```json
{ "success": false, "message": "Missing or invalid token" }
```

### Rate limiting

| Scope        | Key   | Default limit         |
|--------------|-------|-----------------------|
| `/auth/*`    | IP    | 10 requests / minute  |
| `/api/*`     | User  | 100 requests / minute |

Limits are configurable (see [Configuration](#configuration)). Responses include:

- `X-RateLimit-Remaining` — requests left in the window
- `X-RateLimit-Reset` — Unix timestamp when the window resets
- `Retry-After` — seconds to wait (only on `429`)

Exceeding the limit → `429 Too Many Requests`:

```json
{ "success": false, "message": "too many requests, please try again in 12 second(s)" }
```

### Errors & validation

| Status | Meaning                                                          |
|--------|-----------------------------------------------------------------|
| `400`  | Malformed JSON / invalid path parameter                         |
| `401`  | Missing, invalid, or wrong-type token; bad credentials          |
| `403`  | Email not verified (login)                                      |
| `404`  | Resource not found                                              |
| `409`  | Conflict (duplicate email, invalid state transition)            |
| `422`  | Validation failed / password change rejected                   |
| `429`  | Rate limit exceeded                                             |
| `500`  | Internal error                                                  |
| `502`  | Upstream payment provider (Xendit) failure                     |

Validation failures (`422`) include a per-field `errors` array:

```json
{
  "success": false,
  "message": "validation failed",
  "errors": [
    { "field": "email", "message": "email must be a valid email" },
    { "field": "password", "message": "password must be at least 6 characters" }
  ]
}
```

---

## Health

### GET /health

Liveness probe. **Public** — no authentication, no rate limiting. Intended for
load balancers and uptime monitors.

```bash
curl http://localhost:8080/health
```

**`200 OK`:**

```json
{
  "success": true,
  "message": "ok",
  "data": { "status": "healthy" }
}
```

---

## Auth endpoints

Base path: `/auth` — IP rate-limited, no authentication required.

### POST /auth/register

Create a new account. Sends a verification email; the account cannot log in until
verified.

**Request:**

```json
{
  "name": "Ilham Azhar",
  "email": "ilham@example.com",
  "password": "secret123",
  "password_confirm": "secret123"
}
```

| Field              | Rules                                |
|--------------------|--------------------------------------|
| `name`             | required, ≤ 255 chars                |
| `email`            | required, valid email                |
| `password`         | required, ≥ 6 chars                  |
| `password_confirm` | required, must equal `password`      |

```bash
curl -X POST http://localhost:8080/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Ilham Azhar",
    "email": "ilham@example.com",
    "password": "secret123",
    "password_confirm": "secret123"
  }'
```

**`201 Created`:**

```json
{
  "success": true,
  "message": "Registered successfully. Please verify your email.",
  "data": {
    "user": {
      "id": "8f3c...uuid",
      "name": "Ilham Azhar",
      "email": "ilham@example.com",
      "created_at": "2026-06-18T10:00:00Z",
      "updated_at": "2026-06-18T10:00:00Z"
    },
    "verification_token": "abc123"
  }
}
```

> `verification_token` is returned **only in non-production** environments, so the
> verify flow can be exercised without a real email provider. It is omitted in
> production.

**Errors:** `409` (email already in use), `422` (validation).

### POST /auth/login

Exchange credentials for an access/refresh token pair.

**Request:**

```json
{ "email": "ilham@example.com", "password": "secret123" }
```

```bash
curl -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{ "email": "ilham@example.com", "password": "secret123" }'
```

**`200 OK`:**

```json
{
  "success": true,
  "message": "Logged in successfully",
  "data": {
    "access_token": "eyJ...",
    "refresh_token": "eyJ...",
    "token_type": "Bearer",
    "expires_in": 86400,
    "user": {
      "id": "8f3c...uuid",
      "name": "Ilham Azhar",
      "email": "ilham@example.com",
      "email_verified_at": "2026-06-18T10:05:00Z",
      "created_at": "2026-06-18T10:00:00Z",
      "updated_at": "2026-06-18T10:05:00Z"
    }
  }
}
```

`expires_in` is the access token lifetime in **seconds**.

**Errors:** `401` (invalid credentials), `403` (email not verified).

### GET /auth/verify

Consumes the token from the email link. Designed to be opened directly in a
browser — it does **not** return JSON; it issues a `302 Found` redirect to the
frontend login page with a status flag.

**Query parameter:** `token` — the verification token from the email.

| Outcome  | Redirect                                        |
|----------|-------------------------------------------------|
| Success  | `<FRONTEND_URL>/login?verified=true`            |
| Failure  | `<FRONTEND_URL>/login?verified=false`           |

```bash
# -i shows the redirect headers (Location); -L would follow it
curl -i "http://localhost:8080/auth/verify?token=abc123"
```

### POST /auth/resend-verification

Re-sends the verification email. Always returns `200` regardless of whether the
email exists, to avoid leaking account existence.

**Request:**

```json
{ "email": "ilham@example.com" }
```

```bash
curl -X POST http://localhost:8080/auth/resend-verification \
  -H "Content-Type: application/json" \
  -d '{ "email": "ilham@example.com" }'
```

**`200 OK`:**

```json
{
  "success": true,
  "message": "If the email exists and is unverified, a verification link has been sent",
  "data": { "verification_token": "abc123" }
}
```

> `data.verification_token` appears only in non-production environments.

### POST /auth/forgot-password

Request a password reset. Sends an email containing a link to the frontend reset
page (`<FRONTEND_URL>/reset-password?token=...`). Always returns `200` regardless
of whether the email exists, to avoid leaking account existence.

**Request:**

```json
{ "email": "ilham@example.com" }
```

```bash
curl -X POST http://localhost:8080/auth/forgot-password \
  -H "Content-Type: application/json" \
  -d '{ "email": "ilham@example.com" }'
```

**`200 OK`:**

```json
{
  "success": true,
  "message": "If the email exists, a password reset link has been sent",
  "data": { "reset_token": "abc123" }
}
```

> `data.reset_token` appears only in non-production environments, so the reset
> flow can be exercised without a real email provider. The token is single-use and
> expires after `PASSWORD_RESET_EXPIRY_HOURS` (default 1 hour).

### POST /auth/reset-password

Consume a reset token and set a new password. The token is invalidated on
success (one-time use).

**Request:**

```json
{
  "token": "abc123",
  "new_password": "newsecret456",
  "confirm_password": "newsecret456"
}
```

| Field              | Rules                                  |
|--------------------|----------------------------------------|
| `token`            | required (from the reset email)        |
| `new_password`     | required, ≥ 6 chars                    |
| `confirm_password` | required, must equal `new_password`    |

```bash
curl -X POST http://localhost:8080/auth/reset-password \
  -H "Content-Type: application/json" \
  -d '{
    "token": "abc123",
    "new_password": "newsecret456",
    "confirm_password": "newsecret456"
  }'
```

**`200 OK`:** `{ "success": true, "message": "Password reset successfully" }`

**Errors:** `400` (invalid, expired, or already-used token), `422` (validation).

### POST /auth/refresh

Rotate tokens using a valid refresh token.

**Request:**

```json
{ "refresh_token": "eyJ..." }
```

```bash
curl -X POST http://localhost:8080/auth/refresh \
  -H "Content-Type: application/json" \
  -d '{ "refresh_token": "eyJ..." }'
```

**`200 OK`:** same `TokenResponse` shape as [login](#post-authlogin), message
`"Token refreshed"`.

**Errors:** `401` (invalid/expired/revoked refresh token).

### POST /auth/logout

Revoke a refresh token.

**Request:**

```json
{ "refresh_token": "eyJ..." }
```

```bash
curl -X POST http://localhost:8080/auth/logout \
  -H "Content-Type: application/json" \
  -d '{ "refresh_token": "eyJ..." }'
```

**`200 OK`:**

```json
{ "success": true, "message": "Logged out successfully" }
```

**Errors:** `401` (invalid refresh token).

---

## Current user (`/api/me`)

Requires `Authorization: Bearer <access_token>`.

### GET /api/me/

Return the authenticated user's profile.

> Note the trailing slash: the route is `/api/me/`.

```bash
curl http://localhost:8080/api/me/ \
  -H "Authorization: Bearer $TOKEN"
```

**`200 OK`:**

```json
{
  "success": true,
  "message": "User info retrieved",
  "data": {
    "id": "8f3c...uuid",
    "name": "Ilham Azhar",
    "email": "ilham@example.com",
    "email_verified_at": "2026-06-18T10:05:00Z",
    "created_at": "2026-06-18T10:00:00Z",
    "updated_at": "2026-06-18T10:05:00Z"
  }
}
```

### PUT /api/me/password

Change the authenticated user's password.

**Request:**

```json
{
  "current_password": "secret123",
  "new_password": "newsecret456",
  "confirm_password": "newsecret456"
}
```

| Field              | Rules                                  |
|--------------------|----------------------------------------|
| `current_password` | required                               |
| `new_password`     | required, ≥ 6 chars                    |
| `confirm_password` | required, must equal `new_password`    |

```bash
curl -X PUT http://localhost:8080/api/me/password \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "current_password": "secret123",
    "new_password": "newsecret456",
    "confirm_password": "newsecret456"
  }'
```

**`200 OK`:** `{ "success": true, "message": "Password changed successfully" }`

**Errors:** `422` (wrong current password / validation).

---

## Payments

Requires authentication. Backed by Xendit QRIS.

### POST /api/payments/qris

Create a standalone QRIS payment for the authenticated user.

**Request:**

```json
{ "amount": 50000, "description": "Top up" }
```

| Field         | Rules                          |
|---------------|--------------------------------|
| `amount`      | required, `> 0` (minor units)  |
| `description` | required, ≤ 255 chars          |

```bash
curl -X POST http://localhost:8080/api/payments/qris \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{ "amount": 50000, "description": "Top up" }'
```

**`201 Created`:**

```json
{
  "success": true,
  "message": "QRIS created",
  "data": {
    "order_ref": "ord_a1b2c3",
    "qr_string": "00020101021126...",
    "amount": 50000,
    "currency": "IDR",
    "status": "PENDING",
    "expires_at": "2026-06-18T11:00:00Z",
    "description": "Top up"
  }
}
```

Render `qr_string` as a QRIS code for the payer to scan.

**Errors:** `502` (Xendit failure).

### GET /api/payments/:order_ref

Look up a payment's current status by its `order_ref`.

```bash
curl http://localhost:8080/api/payments/ord_a1b2c3 \
  -H "Authorization: Bearer $TOKEN"
```

**`200 OK`:**

```json
{
  "success": true,
  "message": "Order status retrieved",
  "data": {
    "order_ref": "ord_a1b2c3",
    "amount": 50000,
    "status": "PAID",
    "paid_at": "2026-06-18T10:30:00Z",
    "expires_at": "2026-06-18T11:00:00Z",
    "description": "Top up"
  }
}
```

`status` is one of [`PaymentStatus`](#paymentstatus). It transitions to `PAID`
asynchronously when Xendit calls the [webhook](#post-webhooksxendit).

**Errors:** `404` (unknown `order_ref`).

---

## Financings (Murabahah)

Requires authentication. A **Murabahah** is a cost-plus sale: the financier buys
an asset at `cost_price` and sells it to the customer at `total_price =
cost_price + margin_amount`, repaid over `tenor` monthly installments.

> **Syariah invariant:** the margin is fixed once the akad is signed. It is never
> recalculated as a function of time (that would be *riba*). All money fields are
> integer minor units.

**Lifecycle:** `DRAFT` → (sign akad) → `ACTIVE` → (all installments paid) →
`SETTLED`.

### POST /api/financings

Create a Murabahah financing in `DRAFT` status. The full installment schedule is
generated upfront.

**Request:**

```json
{
  "asset_name": "Honda Vario 160",
  "cost_price": 25000000,
  "margin_amount": 3000000,
  "down_payment": 5000000,
  "tenor": 12,
  "first_due_date": "2026-07-18T00:00:00Z"
}
```

| Field            | Rules                                             |
|------------------|---------------------------------------------------|
| `asset_name`     | required, ≤ 255 chars                             |
| `cost_price`     | required, `> 0`                                  |
| `margin_amount`  | `>= 0`                                            |
| `down_payment`   | `>= 0` and `< cost_price` (it reduces principal)  |
| `tenor`          | required, `1`–`360` (monthly installments)       |
| `first_due_date` | optional; defaults to ~one month out when omitted |

```bash
curl -X POST http://localhost:8080/api/financings \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "asset_name": "Honda Vario 160",
    "cost_price": 25000000,
    "margin_amount": 3000000,
    "down_payment": 5000000,
    "tenor": 12,
    "first_due_date": "2026-07-18T00:00:00Z"
  }'
```

**`201 Created`:**

```json
{
  "success": true,
  "message": "Financing created",
  "data": {
    "id": 1,
    "akad_type": "MURABAHAH",
    "asset_name": "Honda Vario 160",
    "cost_price": 25000000,
    "margin_amount": 3000000,
    "total_price": 28000000,
    "down_payment": 5000000,
    "tenor": 12,
    "currency": "IDR",
    "status": "DRAFT",
    "installments": [
      {
        "installment_no": 1,
        "due_date": "2026-07-18T00:00:00Z",
        "principal_part": 1666667,
        "margin_part": 250000,
        "amount": 1916667,
        "status": "UNPAID"
      }
    ],
    "created_at": "2026-06-18T10:00:00Z"
  }
}
```

**Errors:** `400` (invalid input), `422` (validation).

### GET /api/financings

List the authenticated user's financings (paginated). See
[Pagination](#pagination).

```bash
curl "http://localhost:8080/api/financings?page=1&limit=10" \
  -H "Authorization: Bearer $TOKEN"
```

**`200 OK`:** paginated list of `FinancingResponse` objects (installments may be
omitted in the list view), message `"Financings retrieved"`.

### GET /api/financings/:id

Retrieve a single financing (with its full installment schedule) owned by the
caller.

```bash
curl http://localhost:8080/api/financings/1 \
  -H "Authorization: Bearer $TOKEN"
```

**`200 OK`:** message `"Financing retrieved"`, `data` is a `FinancingResponse`.

**Errors:** `400` (invalid id), `404` (not found / not owned).

### POST /api/financings/:id/sign

Sign the akad: transitions a `DRAFT` financing to `ACTIVE` and stamps
`akad_signed_at`. No request body.

```bash
curl -X POST http://localhost:8080/api/financings/1/sign \
  -H "Authorization: Bearer $TOKEN"
```

**`200 OK`:** message `"Akad signed"`, `data` is the updated `FinancingResponse`
with `status: "ACTIVE"` and `akad_signed_at` set.

**Errors:**

| Status | Cause                                                       |
|--------|-------------------------------------------------------------|
| `400`  | Invalid id                                                  |
| `404`  | Financing not found                                         |
| `409`  | Financing is not in `DRAFT` (cannot sign)                   |

### POST /api/financings/:id/installments/:no/pay

Create a QRIS payment for a single installment of an `ACTIVE` financing. `:no` is
the 1-based installment number. No request body.

```bash
# pay installment #1 of financing #1
curl -X POST http://localhost:8080/api/financings/1/installments/1/pay \
  -H "Authorization: Bearer $TOKEN"
```

**`201 Created`:** message `"Installment payment created"`, `data` is a
[`QRISResponse`](#post-apipaymentsqris). When the payer completes the QRIS, the
[webhook](#post-webhooksxendit) settles the installment and marks it `PAID`.

**Errors:**

| Status | Cause                                                       |
|--------|-------------------------------------------------------------|
| `400`  | Invalid id or installment number                            |
| `404`  | Financing not found                                         |
| `409`  | Financing not `ACTIVE`, or installment already paid         |
| `502`  | Upstream payment provider (Xendit) failure                  |

---

## Users

Requires authentication. User administration.

### GET /api/users

List all users (paginated). See [Pagination](#pagination).

```bash
curl "http://localhost:8080/api/users?page=1&limit=10" \
  -H "Authorization: Bearer $TOKEN"
```

**`200 OK`:** paginated list of `UserResponse`, message `"Users retrieved"`.

### GET /api/users/:id

Retrieve a user by UUID.

```bash
curl http://localhost:8080/api/users/8f3c0000-0000-0000-0000-000000000000 \
  -H "Authorization: Bearer $TOKEN"
```

**`200 OK`:** message `"User retrieved"`, `data` is a `UserResponse`.

**Errors:** `400` (invalid UUID), `404` (not found).

### PUT /api/users/:id

Update a user's name and/or email.

**Request:**

```json
{ "name": "New Name", "email": "new@example.com" }
```

| Field   | Rules                          |
|---------|--------------------------------|
| `name`  | optional, ≤ 255 chars          |
| `email` | optional, valid email          |

```bash
curl -X PUT http://localhost:8080/api/users/8f3c0000-0000-0000-0000-000000000000 \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{ "name": "New Name", "email": "new@example.com" }'
```

**`200 OK`:** message `"User updated"`, `data` is the updated `UserResponse`.

**Errors:** `400` (invalid UUID), `409` (email already in use).

### DELETE /api/users/:id

Soft-delete a user by UUID.

```bash
curl -X DELETE http://localhost:8080/api/users/8f3c0000-0000-0000-0000-000000000000 \
  -H "Authorization: Bearer $TOKEN"
```

**`200 OK`:** `{ "success": true, "message": "User deleted" }`

**Errors:** `400` (invalid UUID), `404` (not found).

---

## Webhooks

### POST /webhooks/xendit

Xendit payment callback. **Public** endpoint (not under `/api`, not JWT-protected)
authenticated by a shared secret in the header.

- **Header:** `X-Callback-Token: <XENDIT_CALLBACK_TOKEN>`
- **Body limit:** 1 MB
- **Behavior:** updates the matching payment (and any linked financing installment)
  to `PAID`. Always responds `200 OK` with an empty body, even on internal
  processing errors (errors are logged, not surfaced, so Xendit does not retry
  indefinitely). Bodies over the limit get `413 Request Entity Too Large`.

```bash
# illustrative only — normally sent by Xendit, not by hand
curl -X POST http://localhost:8080/webhooks/xendit \
  -H "X-Callback-Token: $XENDIT_CALLBACK_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{ "event": "qr.payment", "data": { "reference_id": "ord_a1b2c3", "status": "PAID" } }'
```

This endpoint is called by Xendit, not by API clients.

---

## Enumerations

### PaymentStatus

| Value     | Meaning                          |
|-----------|----------------------------------|
| `PENDING` | Awaiting payment                 |
| `PAID`    | Settled                          |
| `EXPIRED` | QR expired before payment        |
| `FAILED`  | Payment failed                   |

### FinancingStatus

| Value         | Meaning                                       |
|---------------|-----------------------------------------------|
| `DRAFT`       | Created, akad not yet signed                  |
| `ACTIVE`      | Akad signed, disbursed, installments running  |
| `SETTLED`     | All installments paid                         |
| `WRITTEN_OFF` | Written off                                   |

### InstallmentStatus

| Value    | Meaning              |
|----------|----------------------|
| `UNPAID` | Not yet paid         |
| `PAID`   | Paid                 |
| `LATE`   | Past due, unpaid     |

### AkadType

| Value       | Meaning                              |
|-------------|--------------------------------------|
| `MURABAHAH` | Cost-plus sale (only supported type) |

---

## Configuration

Server behavior is driven by environment variables (see `config/config.go`).
Relevant to the API surface:

| Variable                  | Default                  | Description                              |
|---------------------------|--------------------------|------------------------------------------|
| `SERVER_PORT`             | `8080`                   | HTTP listen port                         |
| `APP_ENV`                 | `development`            | `development` \| `production`            |
| `FRONTEND_URL`            | `http://localhost:3000`  | Email-verify redirect target             |
| `JWT_EXPIRY_HOURS`        | `24`                     | Access token lifetime                    |
| `JWT_REFRESH_EXPIRY_HOURS`| `168`                    | Refresh token lifetime (7 days)          |
| `EMAIL_VERIFY_EXPIRY_HOURS`| `24`                    | Verification token lifetime              |
| `PASSWORD_RESET_EXPIRY_HOURS`| `1`                  | Password-reset token lifetime            |
| `RATE_LIMIT_AUTH_MAX`     | `10`                     | Max `/auth/*` requests per period (IP)   |
| `RATE_LIMIT_AUTH_PERIOD`  | `minute`                 | `second` \| `minute` \| `hour` \| `day`  |
| `RATE_LIMIT_API_MAX`      | `100`                    | Max `/api/*` requests per period (user)  |
| `RATE_LIMIT_API_PERIOD`   | `minute`                 | `second` \| `minute` \| `hour` \| `day`  |
| `CORS_ALLOWED_ORIGINS`    | `*`                      | Comma-separated allowed origins          |

CORS allows methods `GET, POST, PUT, PATCH, DELETE` and headers
`Origin, Content-Type, Authorization`.
