# Cloudflare delivery branches

`main` is the unmodified upstream mirror. Update it only with GitHub's **Sync fork** control.

`cloudflare` is the deployable integration branch. Every product change and every upstream update reaches it through a pull request. The pull request from `main` to `cloudflare` preserves a merge commit so Git can retain the relationship to the upstream history.

The Cloudflare Integration workflow checks build-identity handling for source maps on every pull request into `cloudflare`. The stage deployment will follow a successful merge once the D1 compatibility adapter is complete. Production promotes the stage-verified immutable Worker version and container image digest; it does not rebuild from the branch.

## Release sequence

1. Sync `main` in GitHub.
2. Open `main` into `cloudflare`; resolve any conflicts in a `sync/*` branch and merge with a merge commit.
3. Merge after the Cloudflare Integration and affected upstream checks pass.
4. Deploy the resulting commit to stage and validate a test application: ingest, version/build identity, source-map resolution, and notification delivery.
5. Promote that exact Worker version, container digest, and migration level to production.

Stage and production will use separate D1 databases, R2 buckets, Workers, Durable Object namespaces, secrets, and project tokens.
