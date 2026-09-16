# Selection review: D1, serverless, and upstream preservation

This review supersedes any conclusion based only on feature scores. The deciding constraint is now: operate multiple Cloudflare instances against shared D1/R2 state while preserving as much upstream code as possible and continuing to merge upstream.

## Result

Traceway remains the selected base. It is not a low-effort D1 port. It is the least risky option **when active upstream and product completeness are both required**, because ordinary Go `database/sql` reads and writes can run through the small `d1http` adapter. The Cloudflare-specific work is therefore concentrated at its explicit transaction boundaries instead of replacing every repository query.

The selection must be revisited if the team decides that minimizing transaction migration matters more than receiving active upstream changes. Under that different priority, Urgentry is the first candidate to prototype, but it is not an equivalent long-lived upstream dependency.

## Static comparison

The counts below exclude tests and were captured from the checked source snapshots on 2026-09-16. They are indicators of migration shape, not estimates of person-days.

| Project | Ordinary SQL call sites | Explicit transaction surface | Server-local durable state | Consequence for D1 multi-instance work |
| --- | ---: | ---: | --- | --- |
| Traceway | 109 Go files / 1,881 references | 118 files / 807 references | No database-file synchronization proposed | `d1http` preserves the ordinary `database/sql` surface; transaction paths need batches or guarded commands. |
| Urgentry Tiny | 122 Go files / 819 references | 24 files / 56 references | Blob storage already has an S3 implementation | A Go HTTP driver can preserve ordinary SQL and its transaction scope is materially smaller; the source snapshot has low upstream activity and only a small commit history. |
| Stackpit | 58 Rust files / 691 references | 8 files / 34 references | No comparable ingest spool found in the server path | Its small transaction count is misleading: SQLx has no drop-in D1 HTTPS driver, so the whole direct-query surface needs an abstraction or rewrite. Open-source HTTP notification delivery also remains a product gap. |
| Rustrak | 40 Rust files / 307 references | 15 files / 26 references | Ingest relies on filesystem hard-link/rename quarantine and spool operations | SQLx query migration is required and the file-backed ingress protocol must become Queues/R2-aware before replicas are safe. |

## Why this still favours Traceway

1. Traceway's direct-query surface is preserved by a concrete compatibility adapter instead of being recreated: `backend/app/db/d1http` implements `database/sql` query and execute operations over D1's HTTPS raw API.
2. Its remaining work is visible and bounded by the transaction inventory. The driver rejects `Begin` and `BeginTx`, so no path can silently degrade an atomic operation into partial writes.
3. It has the strongest upstream signal and the fullest verified product path: Debug-ID source maps, native symbols, archive/reopen, and durable notification outbox/retry. These are already present upstream, rather than becoming a Cloudflare-only product fork.

## Non-negotiable implementation rule

Do not claim the current branch is deployable merely because migrations run on D1 or a single SQL query works. Multi-instance stage deployment begins only after the following vertical slice is proven against remote D1:

1. event ingest creates or reopens an issue and records its occurrence atomically;
2. the corresponding notification intent is durable in the same success boundary;
3. a second instance cannot duplicate the group counter or notification;
4. a retry after an injected transport failure is idempotent.

Each converted operation remains behind the `cloudflare` build tag. `main` stays the exact GitHub-Sync mirror of upstream, and `cloudflare` is the only long-lived integration branch.
