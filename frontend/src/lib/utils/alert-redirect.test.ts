import { describe, expect, it, vi } from 'vitest';
import type { Load } from '@sveltejs/kit';
import { authenticatedLandingPath } from './landing';

vi.mock('$lib/state/auth.svelte', () => ({ authState: { isAuthenticated: false } }));
vi.mock('$lib/status-domain', () => ({ statusDomainSlug: null }));

const routes = [
	['/', () => import('../../routes/+page')],
	['/issues', () => import('../../routes/issues/+page')],
	['/issues/abc', () => import('../../routes/issues/[exceptionHash]/+page')],
	['/endpoints', () => import('../../routes/endpoints/+page')],
	['/tasks', () => import('../../routes/tasks/+page')],
	['/dashboards', () => import('../../routes/dashboards/+page')],
	['/ai-traces', () => import('../../routes/ai-traces/+page')],
	['/ai-traces/conversations', () => import('../../routes/ai-traces/conversations/+page')],
	['/monitors', () => import('../../routes/monitors/+page')],
	['/monitors/42', () => import('../../routes/monitors/[checkId]/+page')]
] as const;

describe('alert links through login', () => {
	it.each(routes)('preserves the destination and project for %s', async (path, importRoute) => {
		const { load } = await importRoute();
		const routeLoad = load as Load;
		const destination = `${path}?projectId=alert-project&preset=1h`;
		let redirected = false;
		try {
			await routeLoad({
				url: new URL(destination, 'https://traceway.example.com'),
				params: { exceptionHash: 'abc', checkId: '42' }
			} as unknown as Parameters<Load>[0]);
		} catch (error) {
			redirected = true;
			const redirect = error as { status: number; location: string };
			expect(redirect.status).toBe(302);
			const login = new URL(redirect.location, 'https://traceway.example.com');
			expect(login.pathname).toBe('/login');
			expect(
				authenticatedLandingPath(
					login.searchParams.get('returnTo'),
					[],
					[{ id: 'other-project', organizationId: null }]
				)
			).toBe(destination);
		}
		expect(redirected).toBe(true);
	});
});
