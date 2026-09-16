# Cloudflare delivery branches

`main` is the unmodified upstream mirror. Update it only with GitHub's **Sync fork** control.

`cloudflare` is the deployable integration branch. Every product change and every upstream update reaches it through a pull request. The pull request from `main` to `cloudflare` preserves a merge commit so Git can retain the relationship to the upstream history.

The Cloudflare Integration workflow checks build-identity handling for source maps on every pull request into `cloudflare`. `npm test` in this directory is the local workerd/D1 compatibility baseline: it applies both upstream SQLite migration streams and checks the D1 batch boundary. It is not evidence that the current Go database layer can use D1.

The stage deployment will follow a successful merge once the Cloudflare persistence and queue implementation is complete. Production promotes the stage-verified immutable Worker version and container image digest; it does not rebuild from the branch.

## Release sequence

1. Sync `main` in GitHub.
2. Open `main` into `cloudflare`; resolve any conflicts in a `sync/*` branch and merge with a merge commit.
3. Merge after the Cloudflare Integration and affected upstream checks pass.
4. Deploy the resulting commit to stage and validate a test application: ingest, version/build identity, source-map resolution, and notification delivery.
5. Promote that exact Worker version, container digest, and migration level to production.

Stage and production will use separate D1 databases, R2 buckets, Workers, Durable Object namespaces, secrets, and project tokens.

## D1 compatibility baseline

Run this from `deploy/cloudflare`:

```sh
npm ci
npm test
```

The probe intentionally verifies only facts that are safe to carry forward:

- all current `sqlite` and `sqlite_telemetry` migration files parse and execute in local workerd D1;
- a failed D1 `batch()` rolls back its whole batch;
- later statements in the same batch see earlier writes;
- interactive `BEGIN` transactions are unavailable.

Traceway currently relies on `database/sql`, request-scoped `*sql.Tx`, and a separate telemetry database. Those contracts must be replaced with Worker/DO operations, D1 batches or guarded updates, and Queues before any multi-instance deployment is enabled.
