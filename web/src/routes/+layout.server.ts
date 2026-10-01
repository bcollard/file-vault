import { config } from '$lib/server/config';

export const load = async () => {
	return {
		previewMode: config.previewMode
	};
};
