import { config } from '#lib/server/config.ts';

export const load = async () => {
	return {
		previewMode: config.previewMode
	};
};
