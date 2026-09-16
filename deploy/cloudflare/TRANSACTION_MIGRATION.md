# Transaction migration inventory

The D1 HTTPS driver preserves ordinary `database/sql` reads and writes. It must not pretend that an interactive `*sql.Tx` can be implemented over D1's HTTPS API.

At commit `6b3228da`, the static inventory contains 127 Go files mentioning `*sql.Tx` and 322 references to `db.DB.Begin`, `db.ExecuteTransaction`, or `middleware.Transactional`. The main SQLite implementation has 29 transactional repository files whose methods take `*sql.Tx`.

This means the Cloudflare migration is a broad but mechanical interface migration for the main database. It is not limited to a few exceptional handlers.

## Preservation-first sequence

1. Keep the SQLite/Postgres repository implementations and their current tests unchanged.
2. Introduce an execution interface accepted by transactional repositories for ordinary reads and single-statement writes. Both `*sql.DB` and `*sql.Tx` satisfy the native build; `d1http` supplies it in the Cloudflare build.
3. Compile the Cloudflare request middleware without creating `*sql.Tx`. Routes with only reads or one guarded write then keep their existing controller and repository logic.
4. Replace each remaining multi-step mutation with one explicit D1 batch. The batch must contain every statement before it is sent.
5. Where a mutation decides from a read that cannot be expressed as a guarded update, move only that operation behind a small project-scoped serial command. Keep the caller's Go domain logic and API shape.
6. Migrate the notification outbox as the first multi-step slice: enqueue intent, claim, cancel, send result, retry, and stale-claim recovery.

## Implemented first slice

The Cloudflare build now uses D1 batches for the outbox drain's stale-claim recovery and every finite state transition. A drain reads due rows, then sends a guarded `pending -> sending` batch for each candidate. Only statements whose `changes` value is non-zero own a delivery; competing instances skip the row. Success, retry, and terminal failure transitions are similarly guarded on `sending`; page notification mirrors share the same batch as their outbox transition.

Enqueue and cancellation still participate in their caller's broader transaction and remain migration work. The first remote stage proof must cover event/issue/outbox enqueue atomically before the outbox drain is enabled for a multi-instance stage.

## Rules

- `d1http.Begin` always fails. A test failure is a migration task, never a reason to silently auto-commit each statement.
- Do not send a database API token to browsers. It is stored only as a stage/prod Container secret.
- Do not add an HTTP SQL gateway. Go Containers use D1's HTTPS API directly.
- Keep every Cloudflare-only implementation behind the `cloudflare` build tag; upstream SQLite and PostgreSQL paths remain untouched.
