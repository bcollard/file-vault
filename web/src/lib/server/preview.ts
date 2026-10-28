import type {
	AdminShare,
	IssueInput,
	IssueResult,
	ShareResponse,
	TokenStatus,
	UploadInput
} from '#lib/types.ts';
import type { API } from './api';

// PreviewClient returns canned data without touching the Go API. Used by
// `make web-dev` so the UI is browsable without GCP credentials.
//
// State is process-local — a refresh keeps your "share" but a server restart
// resets everything. Good enough for visual checks.
export class PreviewClient implements API {
	private tokens: TokenStatus[] = [];
	private deleted = false;

	async createShare(input: UploadInput): Promise<ShareResponse> {
		const now = new Date();
		const dlExp = new Date(now.getTime() + input.ttlHours * 3600 * 1000);
		const shareExp = new Date(now.getTime() + 7 * 24 * 3600 * 1000);
		this.deleted = false;
		this.tokens = [
			{
				short_id: 'a3f9c2b1',
				status: 'active',
				created_at: now.toISOString(),
				expires_at: dlExp.toISOString(),
				max_uses: input.maxUses,
				uses_remaining: input.maxUses
			}
		];
		// Drain the upload so the browser sees normal completion.
		if (input.file instanceof Blob) {
			await input.file.arrayBuffer().catch(() => null);
		}
		return {
			share_id: 'preview-share-id',
			filename: input.filename || 'unnamed',
			size: (input.file as Blob)?.size ?? 0,
			expires_at: shareExp.toISOString(),
			download_token: 'preview-download-token',
			download_token_expires: dlExp.toISOString(),
			download_token_max: input.maxUses,
			admin_token: 'preview-admin-token'
		};
	}

	async getAdminShare(): Promise<AdminShare> {
		if (this.deleted) throw { kind: 'not_found' };
		// Seed if the user reloaded admin without uploading first.
		if (this.tokens.length === 0) this.seed();
		const now = new Date();
		return {
			share_id: 'preview-share-id',
			filename: 'quarterly-report.pdf',
			size: 2 * 1024 * 1024,
			content_type: 'application/pdf',
			created_at: new Date(now.getTime() - 2 * 3600 * 1000).toISOString(),
			expires_at: new Date(now.getTime() + 5 * 24 * 3600 * 1000).toISOString()
		};
	}

	async listTokens(): Promise<TokenStatus[]> {
		if (this.deleted) throw { kind: 'not_found' };
		if (this.tokens.length === 0) this.seed();
		return [...this.tokens];
	}

	async issueDownloadToken(_admin: string, input: IssueInput): Promise<IssueResult> {
		const now = new Date();
		const exp = new Date(now.getTime() + input.ttlHours * 3600 * 1000);
		const id = Math.random().toString(16).slice(2, 10);
		this.tokens.unshift({
			short_id: id,
			status: 'active',
			created_at: now.toISOString(),
			expires_at: exp.toISOString(),
			max_uses: input.maxUses,
			uses_remaining: input.maxUses
		});
		return {
			download_token: `preview-dl-${id}`,
			expires_at: exp.toISOString(),
			max_uses: input.maxUses
		};
	}

	async revokeTokens(): Promise<{ revoked: number }> {
		let n = 0;
		this.tokens = this.tokens.map((t) => {
			if (t.status === 'active') {
				n++;
				return { ...t, status: 'revoked' };
			}
			return t;
		});
		return { revoked: n };
	}

	async deleteShare(): Promise<void> {
		this.deleted = true;
		this.tokens = [];
	}

	private seed() {
		const now = new Date();
		const t = (off: number) => new Date(now.getTime() + off).toISOString();
		this.tokens = [
			{
				short_id: 'a3f9c2b1',
				status: 'active',
				created_at: t(-15 * 60_000),
				expires_at: t(48 * 3600_000),
				max_uses: 1,
				uses_remaining: 1
			},
			{
				short_id: '8c4d7e02',
				status: 'exhausted',
				created_at: t(-90 * 60_000),
				expires_at: t(46 * 3600_000),
				max_uses: 3,
				uses_remaining: 0,
				first_used_at: t(-60 * 60_000),
				last_used_at: t(-30 * 60_000),
				last_used_from_ip: '203.0.113.42'
			},
			{
				short_id: '1f5e9d3a',
				status: 'revoked',
				created_at: t(-3 * 3600_000),
				expires_at: t(45 * 3600_000),
				max_uses: 1,
				uses_remaining: 1
			},
			{
				short_id: '5b8a4c7f',
				status: 'expired',
				created_at: t(-72 * 3600_000),
				expires_at: t(-24 * 3600_000),
				max_uses: 1,
				uses_remaining: 1
			}
		];
	}
}
