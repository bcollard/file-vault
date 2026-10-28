import { defineEnvVars } from '@sveltejs/kit/env';

// All optional: the server reads them at startup, with fallbacks in
// lib/server/config.ts and lib/server/api.ts.
const optional = (value: string | undefined) => value;

export const variables = defineEnvVars({
	API_URL: { schema: optional },
	API_AUDIENCE: { schema: optional },
	PREVIEW_MODE: { schema: optional },
	PUBLIC_DOMAIN: { schema: optional },
	PORT: { schema: optional },
	DEFAULT_TOKEN_TTL_HOURS: { schema: optional },
	MIN_TOKEN_TTL_HOURS: { schema: optional },
	MAX_TOKEN_TTL_HOURS: { schema: optional },
	DEFAULT_MAX_USES: { schema: optional },
	MAX_USES_PER_TOKEN: { schema: optional }
});
