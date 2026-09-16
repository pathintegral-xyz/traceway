# Cloudflare target architecture

Traceway cannot run multiple writers against copies of its current SQLite files. The Cloudflare deployment is therefore a state-model migration, not a container scale-out.

## Ownership

| Concern | Cloudflare owner | Consistency rule |
| --- | --- | --- |
| Dashboard accounts, projects, configuration and notification intent | D1 | Schema migrations run before a Worker version is promoted. |
| A project's Issue state, occurrence counters and notification decisions | Project Durable Object | One project key is processed serially; independent projects scale independently. |
| Ingest hand-off, symbolication work and notification delivery | Queues | Consumers are idempotent and persist their outcome before acknowledging work. |
| Events, source maps, symbols and attachments | R2 | Keys include the project and immutable build identity. |
| Go-only computation such as native symbolication | Container worker | It owns no durable application state. |

## Transaction replacement

The existing request transaction middleware and outbox depend on an interactive `*sql.Tx`. D1 supports a fixed SQL batch, but not an open transaction that makes decisions between round trips. Each Cloudflare mutation must therefore use one of these forms:

1. A guarded D1 update for a state transition that is independent of other state.
2. One D1 batch when all statements are known before execution.
3. A Project Durable Object operation when an Issue update, counter update and outbox intent must be serialised together.

No service may use a remote D1 HTTP endpoint as a substitute for `database/sql`.

## Required acceptance slices

1. Project authentication and event enqueueing from multiple Worker instances.
2. Duplicate events for one project produce one Issue state transition and correct occurrence count.
3. A new or regressed Issue writes a durable notification intent before Queue acknowledgement.
4. A failed notification retries after a worker restart and a cancel wins against an in-flight completion.
5. A Source Map is found only by its supplied Debug ID/build identity; a miss remains visible and never falls back to a same-named artifact.
6. A stage migration and Worker version are promoted together, then rolled back without serving a mixed schema.
