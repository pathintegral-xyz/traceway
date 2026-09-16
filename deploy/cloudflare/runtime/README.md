# Cloudflare Containers stage runtime

This directory is a deployable Stage template, not an instruction to deploy production or route production traffic. Copy `wrangler.jsonc.example` to `wrangler.jsonc`, replace every `replace-with-*` value, then run `npm run check:runtime` before using Wrangler.

The Worker passes its secrets into each stateless Go Container only when it starts. Browser clients never receive them. The Container image is built from the repository root through `image_build_context`, embeds the dashboard, and runs Traceway with `-tags cloudflare`.

## Operator actions required before stage deployment

1. Create two empty D1 databases named `traceway-main-stage` and `traceway-telemetry-stage-0`; copy both database UUIDs into the stage config. The telemetry database is one compatibility shard for the Stage proof, not the production-wide telemetry topology.
2. Create an R2 bucket such as `traceway-stage`; create an S3-compatible R2 API token restricted to that bucket, and retain its access key ID and secret access key.
3. Create a Cloudflare API token restricted to **D1 Read** and **D1 Write** for only the two stage databases. This is the token that the Go Containers use for the direct D1 HTTPS API.
4. Set these four Worker secrets on the stage Worker: `JWT_SECRET` (at least 32 random bytes), `CLOUDFLARE_D1_API_TOKEN`, `S3_ACCESS_KEY`, and `S3_SECRET_KEY`.
5. Replace the public placeholders in `wrangler.jsonc`: Worker name, account ID, both D1 IDs, R2 bucket, R2 S3 endpoint, and the stage HTTPS origin. The origin must already be routed through Cloudflare before it becomes `APP_BASE_URL`.
6. Ensure a local Docker-compatible daemon is running before `wrangler deploy`; Cloudflare builds and pushes the image from the Dockerfile during deployment.

Do not create production resources from this template. Production receives a separately copied configuration, separate D1 databases, separate R2 bucket and credentials, and a promotion only after the remote stage acceptance slice passes.
