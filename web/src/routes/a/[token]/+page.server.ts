import { fail, error, type Actions } from '@sveltejs/kit';
import { getApi } from '#lib/server/api.ts';
import { config, downloadUrl } from '#lib/server/config.ts';
import type { ApiError } from '#lib/types.ts';

export const load = async ({ params }) => {
	const api = getApi();
	try {
		const [share, tokens] = await Promise.all([
			api.getAdminShare(params.token),
			api.listTokens(params.token)
		]);
		return {
			share,
			tokens,
			limits: {
				defaultTtlHours: config.defaultTokenTTLHours,
				minTtlHours: config.minTokenTTLHours,
				maxTtlHours: config.maxTokenTTLHours,
				defaultMaxUses: config.defaultMaxUses,
				maxUsesPerToken: config.maxUsesPerToken
			}
		};
	} catch (e) {
		const err = e as ApiError;
		if (err?.kind === 'not_found') throw error(404, 'Not found');
		throw error(502, 'Backend unreachable');
	}
};

export const actions: Actions = {
	issue: async ({ params, request }) => {
		const form = await request.formData();
		const ttlHours = clamp(
			intOr(form.get('ttl_hours'), config.defaultTokenTTLHours),
			config.minTokenTTLHours,
			config.maxTokenTTLHours
		);
		const maxUses = clamp(
			intOr(form.get('max_uses'), config.defaultMaxUses),
			1,
			config.maxUsesPerToken
		);
		try {
			const result = await getApi().issueDownloadToken(params.token!, { ttlHours, maxUses });
			return {
				issued: {
					url: downloadUrl(result.download_token),
					expiresAt: result.expires_at,
					maxUses: result.max_uses
				}
			};
		} catch (e) {
			const err = e as ApiError;
			if (err.kind === 'not_found') throw error(404, 'Not found');
			return fail(502, { error: 'Could not issue link.' });
		}
	},

	revoke: async ({ params }) => {
		try {
			const { revoked } = await getApi().revokeTokens(params.token!);
			return { revoked };
		} catch (e) {
			const err = e as ApiError;
			if (err.kind === 'not_found') throw error(404, 'Not found');
			return fail(502, { error: 'Could not revoke.' });
		}
	},

	delete: async ({ params }) => {
		try {
			await getApi().deleteShare(params.token!);
			return { deleted: true };
		} catch (e) {
			const err = e as ApiError;
			if (err.kind === 'not_found') throw error(404, 'Not found');
			return fail(502, { error: 'Could not delete.' });
		}
	}
};

function intOr(v: FormDataEntryValue | null, def: number): number {
	if (typeof v !== 'string') return def;
	const n = parseInt(v, 10);
	return Number.isFinite(n) ? n : def;
}

function clamp(v: number, lo: number, hi: number): number {
	if (v < lo) return lo;
	if (v > hi) return hi;
	return v;
}
