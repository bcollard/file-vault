import { env } from '$env/dynamic/private';

function intOr(name: string, fallback: number): number {
	const v = env[name];
	if (!v) return fallback;
	const n = parseInt(v, 10);
	return Number.isFinite(n) ? n : fallback;
}

// Same bounds as the Go API. Surfaced to the UI so the upload form's slider
// stays in sync with what the backend will accept.
export const config = {
	publicDomain: env.PUBLIC_DOMAIN || `localhost:${env.PORT || '5173'}`,
	previewMode: !!env.PREVIEW_MODE,
	defaultTokenTTLHours: intOr('DEFAULT_TOKEN_TTL_HOURS', 48),
	minTokenTTLHours: intOr('MIN_TOKEN_TTL_HOURS', 1),
	maxTokenTTLHours: intOr('MAX_TOKEN_TTL_HOURS', 168),
	defaultMaxUses: intOr('DEFAULT_MAX_USES', 1),
	maxUsesPerToken: intOr('MAX_USES_PER_TOKEN', 100)
};

export function downloadUrl(token: string): string {
	return `https://${config.publicDomain}/d/${token}`;
}

export function adminUrl(token: string): string {
	return `https://${config.publicDomain}/a/${token}`;
}
