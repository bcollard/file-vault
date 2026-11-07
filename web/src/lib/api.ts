// Browser client for the Go API. Every token goes in a POST body so it never
// appears in a URL the server, Cloud Run or the load balancer would log.

export type Limits = {
	default_ttl_hours: number;
	min_ttl_hours: number;
	max_ttl_hours: number;
	default_max_uses: number;
	max_uses_per_token: number;
	max_upload_bytes: number;
	share_ttl_hours: number;
};

export type CreatedShare = { admin_token: string; upload_url: string; expires_at: string };

export type Finalized = {
	download_token: string;
	download_expires_at: string;
	max_uses: number;
	expires_at: string;
};

export type Peek = {
	meta: string;
	size: number;
	expires_at: string;
	uses_remaining: number;
	max_uses: number;
};

export type TokenStatus = {
	short_id: string;
	status: 'active' | 'exhausted' | 'revoked' | 'expired';
	created_at: string;
	expires_at: string;
	max_uses: number;
	uses_remaining: number;
	first_used_at?: string;
	last_used_at?: string;
	last_used_from_ip?: string;
};

export type AdminShare = {
	share_id: string;
	meta: string;
	size: number;
	status: 'pending' | 'ready';
	created_at: string;
	expires_at: string;
	tokens: TokenStatus[];
};

export type Issued = { download_token: string; expires_at: string; max_uses: number };

export class ApiError extends Error {
	readonly status: number;
	constructor(status: number, message: string) {
		super(message);
		this.name = 'ApiError';
		this.status = status;
	}
	get notFound() {
		return this.status === 404;
	}
}

async function call<T>(path: string, body?: unknown): Promise<T> {
	let resp: Response;
	try {
		resp = await fetch(path, {
			method: body === undefined ? 'GET' : 'POST',
			headers: body === undefined ? undefined : { 'content-type': 'application/json' },
			body: body === undefined ? undefined : JSON.stringify(body),
			cache: 'no-store'
		});
	} catch {
		throw new ApiError(0, 'Could not reach the server.');
	}
	if (resp.status === 204) return undefined as T;
	const data = await resp.json().catch(() => ({}));
	if (!resp.ok) throw new ApiError(resp.status, data?.error ?? resp.statusText);
	return data as T;
}

export const api = {
	config: () => call<Limits>('/api/config'),
	createShare: (b: { size: number; meta: string; ttl_hours: number; max_uses: number }) =>
		call<CreatedShare>('/api/shares', b),
	finalize: (admin_token: string) => call<Finalized>('/api/shares/finalize', { admin_token }),
	peek: (token: string) => call<Peek>('/api/download/peek', { token }),
	burn: (token: string) => call<{ url: string; size: number }>('/api/download/burn', { token }),
	admin: (admin_token: string) => call<AdminShare>('/api/admin', { admin_token }),
	issue: (admin_token: string, ttl_hours: number, max_uses: number) =>
		call<Issued>('/api/admin/issue', { admin_token, ttl_hours, max_uses }),
	revoke: (admin_token: string) => call<{ revoked: number }>('/api/admin/revoke', { admin_token }),
	remove: (admin_token: string) => call<void>('/api/admin/delete', { admin_token })
};
