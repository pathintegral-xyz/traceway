# Cloudflare target architecture

The Cloudflare branch preserves Traceway's Go API, repositories, controllers, grouping, notifications, symbolication, and dashboard. It replaces only storage drivers and process-bound scheduling.

```text
Cloudflare Router Worker
  -> stateless Traceway Go Containers
  -> d1http database/sql driver
  -> MAIN D1 and TELEMETRY D1

Traceway's existing S3 storage implementation -> R2
```

## Database ownership

| Existing data | Cloudflare owner | Compatibility approach |
| --- | --- | --- |
| Main SQLite database | `traceway-main-{stage,prod}` D1 | Existing SQL reaches D1 through `d1http`. |
| Telemetry SQLite database | `traceway-telemetry-{stage,prod}` D1 | Existing SQL reaches D1 through `d1http`; capacity is measured before wider rollout. |
| Source maps, symbols, attachments | R2 | Existing S3 storage configuration uses the R2 S3 endpoint. |

There is no runtime SQLite file, WAL file, or R2 database-copy loop. Every Container connects to the same stage or production D1 databases through a least-privilege API token held as a Cloudflare secret.

## Compatibility boundary

`backend/app/db/d1http` implements ordinary `database/sql` queries and writes over D1's HTTPS `/raw` endpoint. The raw endpoint returns ordered columns and rows, preserving the `database/sql.Rows` contract without parsing SQL in the driver.

The driver deliberately rejects `Begin` and `BeginTx`. D1 has atomic batches, but does not expose the open interactive transaction required by the current `*sql.Tx` contract. This makes every remaining transaction dependency visible instead of silently weakening consistency.

## Migration order

1. Run the upstream `sqlite` and `sqlite_telemetry` migrations against separate stage D1 databases.
2. Use `d1http` for non-transactional repository reads and writes, preserving their SQL and call sites.
3. Mechanically migrate transactional repository signatures to a shared execution interface, then convert remaining multi-step mutations to explicit D1 batches or guarded single-statement updates. The detailed inventory is in [TRANSACTION_MIGRATION.md](TRANSACTION_MIGRATION.md).
4. Trigger existing Go background functions from Cloudflare Cron or Queues instead of allowing every Container to poll.
5. Run the same commit against stage, then promote its immutable Container image and migration level to production.

## Required acceptance slices

1. Standard Traceway repository queries scan the same row values and write metadata through D1.
2. Duplicate event ingestion from several Containers produces correct grouping and occurrence counts.
3. A new or regressed Issue persists its notification intent before any send attempt.
4. A failed notification retries after a Container restart and a cancel wins against an in-flight completion.
5. A Source Map is found only by its supplied Debug ID/build identity; a miss remains visible and never falls back to a same-named artifact.
6. A stage migration and Container version are promoted together, then rolled back without serving a mixed schema.
