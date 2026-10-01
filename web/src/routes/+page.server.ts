import { fail, type Actions } from '@sveltejs/kit';
import { getApi } from '$lib/server/api';
import { config, downloadUrl, adminUrl } from '$lib/server/config';
import type { ApiError } from '$lib/types';

export const load = async () => {
	return {
		limits: {
			defaultTtlHours: config.defaultTokenTTLHours,
			minTtlHours: config.minTokenTTLHours,
			maxTtlHours: config.maxTokenTTLHours,
			defaultMaxUses: config.defaultMaxUses,
			maxUsesPerToken: config.maxUsesPerToken
		}
	};
};

export const actions: Actions = {
	default: async ({ request }) => {
		const form = await request.formData();
		const file = form.get('file');
		if (!(file instanceof File) || file.size === 0) {
			return fail(400, { error: 'Please pick a file.' });
		}

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
			const resp = await getApi().createShare({
				file,
				filename: file.name,
				contentType: file.type,
				ttlHours,
				maxUses
			});
			return {
				success: true as const,
				share: {
					filename: resp.filename,
					size: resp.size,
					expiresAt: resp.expires_at,
					downloadUrl: downloadUrl(resp.download_token),
					downloadExpiresAt: resp.download_token_expires,
					downloadMaxUses: resp.download_token_max,
					adminUrl: adminUrl(resp.admin_token)
				}
			};
		} catch (e) {
			const err = e as ApiError;
			if (err.kind === 'bad_request') return fail(400, { error: 'Upload rejected by the server.' });
			return fail(502, { error: 'Could not reach the backend.' });
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
