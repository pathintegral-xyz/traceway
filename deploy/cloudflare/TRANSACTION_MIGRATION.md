# Transaction migration inventory

The D1 HTTPS driver preserves ordinary `database/sql` reads and writes. It must not pretend that an interactive `*sql.Tx` can be implemented over D1's HTTPS API.

At commit `6b3228da`, the static inventory contains 127 Go files mentioning `*sql.Tx` and 322 references to `db.DB.Begin`, `db.ExecuteTransaction`, or `middleware.Transactional`. The main SQLite implementation has 29 transactional repository files whose methods take `*sql.Tx`.

This means the Cloudflare migration is a broad but mechanical interface migration for the main database. It is not limited to a few exceptional handlers.

## Preservation-first sequence

1. Keep the SQLite/Postgres repository implementations and their current tests unchanged.
2. Introduce an execution interface accepted by transactional repositories for ordinary reads and single-statement writes. Both `*sql.DB` and `*sql.Tx` satisfy the native build; `d1http` supplies it in the Cloudflare build.
3. Compile the Cloudflare request middleware without creating `*sql.Tx`. Routes with only reads or one guarded write then keep their existing controller and repository logic.
4. Replace each remaining multi-step mutation with one explicit D1 batch. The batch must contain every statement before it is sent and must target one D1 database.
5. Where a mutation decides from a read that cannot be expressed as a guarded update, move only that operation behind a small project-scoped serial command. Keep the caller's Go domain logic and API shape.
6. Migrate the notification outbox as the first multi-step slice: enqueue intent, claim, cancel, send result, retry, and stale-claim recovery.

## Database boundary

Traceway already separates append-only telemetry from relational application
state. The Cloudflare target preserves that split with a telemetry D1 and a
main D1. D1 cannot make one atomic transaction across two databases, so the
Cloudflare target must not claim a stronger guarantee than the upstream
application has: report ingestion finishes in telemetry first, then the
in-process event hook evaluates notification rules and commits delivery intent
to the main outbox. A crash between those steps may defer that evaluation; it
must never produce a partially-written telemetry batch or a partially-written
outbox state transition.

The atomic units are therefore:

- one telemetry D1 batch for each repository write batch;
- one main D1 batch for each outbox/page state transition or enqueue sequence;
- an idempotent, at-least-once bridge between telemetry ingestion and rule
  evaluation, matching the existing asynchronous hook model.

## Implemented first slice

The Cloudflare build now uses D1 batches for the outbox drain's stale-claim recovery and every finite state transition. A drain reads due rows, then sends a guarded `pending -> sending` batch for each candidate. Only statements whose `changes` value is non-zero own a delivery; competing instances skip the row. Success, retry, and terminal failure transitions are similarly guarded on `sending`; page notification mirrors share the same batch as their outbox transition.

Enqueue and cancellation still participate in their caller's broader transaction and remain migration work. The first remote stage proof must cover telemetry batch writes, concurrent outbox claim/finalization, and idempotent event-rule evaluation before the outbox drain is enabled for a multi-instance stage.

## Rules

- `d1http.Begin` always fails. A test failure is a migration task, never a reason to silently auto-commit each statement.
- Do not send a database API token to browsers. It is stored only as a stage/prod Container secret.
- Do not add an HTTP SQL gateway. Go Containers use D1's HTTPS API directly.
- Keep every Cloudflare-only implementation behind the `cloudflare` build tag; upstream SQLite and PostgreSQL paths remain untouched.

## Central adaptation boundary

The migration must not add `if db.IsCloudflare()` branches to controllers, and
must not make `d1http` manufacture a fake `*sql.Tx`. A `*sql.Tx` permits
interactive reads whose results decide later writes; D1 batch requests require
the complete statement set before execution. Pretending otherwise would turn
atomic flows into silent autocommit sequences.

The permanent boundary is therefore a command layer:

1. Controllers call a domain command such as `Register`, `CreateProject`, or
   `AcceptInvitation`.
2. The native command implementation runs the existing repository calls in a
   SQLite/Postgres transaction.
3. The Cloudflare command implementation validates the request, builds one
   `db.BatchMain` statement set, and sends it atomically to one D1 database.
4. Shared read repositories accept `lit.Executor`, so ordinary reads are
   reused by both implementations without a transaction wrapper.

This keeps the deployment distinction in one package per domain command rather
than spreading it through routes and controllers. Every remaining
`middleware.Transactional` route is migrated to a command before Cloudflare
traffic is enabled for it; the Cloudflare middleware must never call `Begin`.

The implementation order is authentication and setup first (register, login,
OAuth setup, invitations), then organization/project mutations, dashboards,
notifications, and synthetics. Background work uses the same command boundary
and guarded batches.
