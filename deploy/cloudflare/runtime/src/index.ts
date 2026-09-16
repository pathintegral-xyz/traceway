import { Container, getRandom } from '@cloudflare/containers';

export interface Env {
	TRACEWAY: DurableObjectNamespace<TracewayContainer>;
	JWT_SECRET: string;
	CLOUDFLARE_D1_API_TOKEN: string;
	S3_ACCESS_KEY: string;
	S3_SECRET_KEY: string;
	CLOUDFLARE_ACCOUNT_ID: string;
	CLOUDFLARE_D1_MAIN_DATABASE_ID: string;
	CLOUDFLARE_D1_TELEMETRY_DATABASE_ID: string;
	S3_BUCKET: string;
	S3_ENDPOINT: string;
	APP_BASE_URL: string;
}

export class TracewayContainer extends Container {
	defaultPort = 8082;
	requiredPorts = [8082];
	sleepAfter = '30m';
	enableInternet = true;
	pingEndpoint = '/health';

	override onStop({ exitCode, reason }: { exitCode?: number; reason: string }) {
		console.error('Traceway container stopped', { exitCode, reason });
	}

	override async onError(error: unknown) {
		try {
			if (!this.ctx.container) {
				console.error('Traceway container error', { error });
				return;
			}
			const process = await this.ctx.container.exec(['cat', '/tmp/traceway-startup.log']);
			const output = await process.output();
			console.error('Traceway container error', {
				error,
				startupLog: new TextDecoder().decode(output?.stdout)
			});
		} catch (diagnosticError) {
			console.error('Traceway container error', { error, diagnosticError });
		}
	}
}

function runtimeEnv(env: Env): Record<string, string> {
	return {
		JWT_SECRET: env.JWT_SECRET,
		CLOUDFLARE_ACCOUNT_ID: env.CLOUDFLARE_ACCOUNT_ID,
		CLOUDFLARE_D1_MAIN_DATABASE_ID: env.CLOUDFLARE_D1_MAIN_DATABASE_ID,
		CLOUDFLARE_D1_TELEMETRY_DATABASE_ID: env.CLOUDFLARE_D1_TELEMETRY_DATABASE_ID,
		CLOUDFLARE_D1_API_TOKEN: env.CLOUDFLARE_D1_API_TOKEN,
		STORAGE_TYPE: 's3',
		S3_BUCKET: env.S3_BUCKET,
		S3_REGION: 'auto',
		S3_ENDPOINT: env.S3_ENDPOINT,
		S3_ACCESS_KEY: env.S3_ACCESS_KEY,
		S3_SECRET_KEY: env.S3_SECRET_KEY,
		APP_BASE_URL: env.APP_BASE_URL,
		CLOUD_MODE: 'true',
		TRUSTED_PROXY_HEADER: 'CF-Connecting-IP',
		PORTS: '8082'
	};
}

export default {
	async fetch(request: Request, env: Env): Promise<Response> {
		const container = await getRandom(env.TRACEWAY);
		const envVars = runtimeEnv(env);
		console.log('Traceway runtime credentials present', {
			jwt: Boolean(envVars.JWT_SECRET),
			d1: Boolean(envVars.CLOUDFLARE_D1_API_TOKEN),
			r2AccessKey: Boolean(envVars.S3_ACCESS_KEY),
			r2SecretKey: Boolean(envVars.S3_SECRET_KEY)
		});
		try {
			await container.start({ envVars });
			return await container.fetch(request);
		} catch (error) {
			console.error('Traceway container failed to start', {
				error: error instanceof Error ? error.message : String(error),
				state: await container.getState()
			});
			return new Response('Traceway is starting', { status: 503 });
		}
	}
};
