import { Container, getContainer } from '@cloudflare/containers';

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

	override onError(error: unknown) {
		console.error('Traceway container error', error);
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

function canRetryContainerRequest(request: Request): boolean {
	if (request.method === 'GET' || request.method === 'HEAD' || request.method === 'OPTIONS') {
		return true;
	}
	return new URL(request.url).pathname === '/api/synthetics/overview';
}

async function startWhenReady(
	container: Pick<TracewayContainer, 'startAndWaitForPorts'>,
	envVars: Record<string, string>
): Promise<void> {
	await container.startAndWaitForPorts({
		startOptions: { envVars },
		cancellationOptions: {
			instanceGetTimeoutMS: 15000,
			portReadyTimeoutMS: 30000
		}
	});
}

export default {
	async fetch(request: Request, env: Env): Promise<Response> {
		const container = getContainer(env.TRACEWAY, 'primary');
		const envVars = runtimeEnv(env);
		console.log('Traceway runtime credentials present', {
			jwt: Boolean(envVars.JWT_SECRET),
			d1: Boolean(envVars.CLOUDFLARE_D1_API_TOKEN),
			r2AccessKey: Boolean(envVars.S3_ACCESS_KEY),
			r2SecretKey: Boolean(envVars.S3_SECRET_KEY)
		});
		try {
			await startWhenReady(container, envVars);
			const response = await container.fetch(request.clone() as unknown as Request);
			if (response.status < 500 || !canRetryContainerRequest(request)) {
				return response;
			}

			await startWhenReady(container, envVars);
			return await container.fetch(request as unknown as Request);
		} catch (error) {
			console.error('Traceway container failed to start', {
				error: error instanceof Error ? error.message : String(error),
				state: await container.getState()
			});
			return new Response('Traceway is starting', { status: 503 });
		}
	}
};
