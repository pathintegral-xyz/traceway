import { readFile, writeFile } from 'node:fs/promises';

const required = [
	'CLOUDFLARE_ACCOUNT_ID',
	'TRACEWAY_WORKER_NAME',
	'TRACEWAY_MAIN_D1_DATABASE_ID',
	'TRACEWAY_TELEMETRY_D1_DATABASE_ID',
	'TRACEWAY_R2_BUCKET',
	'TRACEWAY_APP_BASE_URL',
	'TRACEWAY_DOMAIN'
];

const missing = required.filter((name) => !process.env[name]);
if (missing.length > 0) {
	throw new Error(`Missing deployment variables: ${missing.join(', ')}`);
}

const config = JSON.parse(await readFile('runtime/wrangler.jsonc.example', 'utf8'));
const accountId = process.env.CLOUDFLARE_ACCOUNT_ID;

config.name = process.env.TRACEWAY_WORKER_NAME;
config.account_id = accountId;
config.routes = [{ pattern: process.env.TRACEWAY_DOMAIN, custom_domain: true }];
config.containers[0].image_vars = {
	TRACEWAY_REVISION: process.env.GITHUB_SHA || 'local'
};
config.vars = {
	CLOUDFLARE_ACCOUNT_ID: accountId,
	CLOUDFLARE_D1_MAIN_DATABASE_ID: process.env.TRACEWAY_MAIN_D1_DATABASE_ID,
	CLOUDFLARE_D1_TELEMETRY_DATABASE_ID: process.env.TRACEWAY_TELEMETRY_D1_DATABASE_ID,
	S3_BUCKET: process.env.TRACEWAY_R2_BUCKET,
	S3_ENDPOINT: `https://${accountId}.r2.cloudflarestorage.com`,
	APP_BASE_URL: process.env.TRACEWAY_APP_BASE_URL
};

await writeFile('runtime/wrangler.generated.jsonc', `${JSON.stringify(config, null, 2)}\n`);
