# Cloudflare delivery branches

`main` is the unmodified upstream mirror. Update it only with GitHub's **Sync fork** control.

`cloudflare` is the deployable integration branch. Every product change and every upstream update reaches it through a pull request. The pull request from `main` to `cloudflare` preserves a merge commit so Git can retain the relationship to the upstream history.

The Cloudflare Integration workflow checks build-identity handling for source maps on every pull request into `cloudflare`. `npm test` in this directory is the local workerd/D1 compatibility baseline: it applies both upstream SQLite migration streams and checks the D1 batch boundary. It is not evidence that the current Go database layer can use D1.

The upstream V2 telemetry schema is applied to D1 at backend startup. `V2_MOVE_OVER` is unavailable in Cloudflare mode, so pre-V2 trace history remains in the legacy tables and is not shown by the V2 dashboard. New telemetry uses the V2 tables.

`.github/workflows/cloudflare-deploy.yml` deploys Stage automatically from `cloudflare`. Production is manual-only: select `prod` and provide a full commit SHA already reachable from `cloudflare`. Protect the GitHub `production` Environment with required reviewers; the workflow rebuilds that exact commit against the distinct production bindings.

Both GitHub Environments (`stage` and `production`) require these Variables: `CLOUDFLARE_ACCOUNT_ID`, `TRACEWAY_WORKER_NAME`, `TRACEWAY_MAIN_D1_DATABASE_ID`, `TRACEWAY_TELEMETRY_D1_DATABASE_ID`, `TRACEWAY_R2_BUCKET`, `TRACEWAY_APP_BASE_URL`, and `TRACEWAY_DOMAIN`. `TRACEWAY_DOMAIN` is declared as a Workers Custom Domain, so Cloudflare creates the DNS record and certificate; it must be an unused hostname in an active Cloudflare zone. They require these Secrets: `CLOUDFLARE_API_TOKEN`, `TRACEWAY_JWT_SECRET`, `TRACEWAY_D1_API_TOKEN`, `TRACEWAY_R2_ACCESS_KEY`, and `TRACEWAY_R2_SECRET_KEY`. The deployment job generates an untracked Wrangler configuration from those values and updates only its target Worker's four runtime secrets.

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
