import { GoogleAuth } from 'google-auth-library';
import { API_AUDIENCE, API_URL, PREVIEW_MODE } from '$app/env/private';
import { PreviewClient } from './preview';
import type {
	AdminShare,
	ApiError,
	IssueInput,
	IssueResult,
	ShareResponse,
	TokenStatus,
	UploadInput
} from '$lib/types';

export interface API {
	createShare(input: UploadInput): Promise<ShareResponse>;
	getAdminShare(adminToken: string): Promise<AdminShare>;
	listTokens(adminToken: string): Promise<TokenStatus[]>;
	issueDownloadToken(adminToken: string, input: IssueInput): Promise<IssueResult>;
	revokeTokens(adminToken: string): Promise<{ revoked: number }>;
	deleteShare(adminToken: string): Promise<void>;
}

export function apiError(kind: ApiError['kind']): ApiError {
	return { kind };
}

class HttpClient implements API {
	private auth: GoogleAuth;

	constructor(
		private readonly apiUrl: string,
		audience: string
	) {
		this.auth = new GoogleAuth();
		// Cache the ID-token client for the lifetime of the process; under the hood
		// it refreshes tokens automatically before they expire.
		this.audience = audience;
	}

	private audience: string;
	private clientPromise?: ReturnType<GoogleAuth['getIdTokenClient']>;

	private getClient() {
		if (!this.clientPromise) {
			this.clientPromise = this.auth.getIdTokenClient(this.audience);
		}
		return this.clientPromise;
	}

	private async request(path: string, init?: RequestInit): Promise<Response> {
		const client = await this.getClient();
		// Get the Authorization header (signed ID token) from google-auth-library
		// and attach it to a normal fetch — that way we keep request/response
		// shape compatible with the rest of SvelteKit.
		const headers = await client.getRequestHeaders(this.apiUrl + path);
		const merged = new Headers(init?.headers);
		for (const [k, v] of Object.entries(headers)) merged.set(k, String(v));
		const resp = await fetch(this.apiUrl + path, { ...init, headers: merged });
		return resp;
	}

	async createShare(input: UploadInput): Promise<ShareResponse> {
		const fd = new FormData();
		fd.append('file', input.file as File, input.filename);
		fd.append('ttl_hours', String(input.ttlHours));
		fd.append('max_uses', String(input.maxUses));
		const resp = await this.request('/api/shares', { method: 'POST', body: fd });
		if (!resp.ok) {
			if (resp.status === 400 || resp.status === 413) throw apiError('bad_request');
			throw apiError('upstream');
		}
		return (await resp.json()) as ShareResponse;
	}

	async getAdminShare(adminToken: string): Promise<AdminShare> {
		const resp = await this.request(`/api/admin/${encodeURIComponent(adminToken)}`);
		if (resp.status === 404) throw apiError('not_found');
		if (!resp.ok) throw apiError('upstream');
		return (await resp.json()) as AdminShare;
	}

	async listTokens(adminToken: string): Promise<TokenStatus[]> {
		const resp = await this.request(`/api/admin/${encodeURIComponent(adminToken)}/tokens`);
		if (resp.status === 404) throw apiError('not_found');
		if (!resp.ok) throw apiError('upstream');
		return (await resp.json()) as TokenStatus[];
	}

	async issueDownloadToken(adminToken: string, input: IssueInput): Promise<IssueResult> {
		const params = new URLSearchParams({
			ttl_hours: String(input.ttlHours),
			max_uses: String(input.maxUses)
		});
		const resp = await this.request(`/api/admin/${encodeURIComponent(adminToken)}/tokens`, {
			method: 'POST',
			headers: { 'content-type': 'application/x-www-form-urlencoded' },
			body: params.toString()
		});
		if (resp.status === 404) throw apiError('not_found');
		if (!resp.ok) throw apiError('upstream');
		return (await resp.json()) as IssueResult;
	}

	async revokeTokens(adminToken: string): Promise<{ revoked: number }> {
		const resp = await this.request(`/api/admin/${encodeURIComponent(adminToken)}/revoke`, {
			method: 'POST'
		});
		if (resp.status === 404) throw apiError('not_found');
		if (!resp.ok) throw apiError('upstream');
		return (await resp.json()) as { revoked: number };
	}

	async deleteShare(adminToken: string): Promise<void> {
		const resp = await this.request(`/api/admin/${encodeURIComponent(adminToken)}`, {
			method: 'DELETE'
		});
		if (resp.status === 404) throw apiError('not_found');
		if (resp.status !== 204) throw apiError('upstream');
	}
}

let cached: API | undefined;

export function getApi(): API {
	if (cached) return cached;
	if (PREVIEW_MODE) {
		cached = new PreviewClient();
		return cached;
	}
	if (!API_URL) {
		throw new Error('API_URL not set (and PREVIEW_MODE not enabled)');
	}
	cached = new HttpClient(API_URL, API_AUDIENCE || API_URL);
	return cached;
}
