import { redirect } from '@sveltejs/kit';
import { authState } from '$lib/state/auth.svelte';
import type { PageLoad } from './$types';

export const prerender = false;

export const load: PageLoad = ({ params, url }) => {
	if (!authState.isAuthenticated) {
		throw redirect(302, `/login?returnTo=${encodeURIComponent(url.pathname + url.search)}`);
	}
	return {
		exceptionHash: params.exceptionHash
	};
};
