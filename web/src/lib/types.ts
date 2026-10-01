export type ShareResponse = {
	share_id: string;
	filename: string;
	size: number;
	expires_at: string;
	download_token: string;
	download_token_expires: string;
	download_token_max: number;
	admin_token: string;
};

export type AdminShare = {
	share_id: string;
	filename: string;
	size: number;
	content_type: string;
	created_at: string;
	expires_at: string;
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

export type IssueResult = {
	download_token: string;
	expires_at: string;
	max_uses: number;
};

export type ApiError = { kind: 'not_found' | 'bad_request' | 'upstream' };

export type UploadInput = {
	file: File | Blob;
	filename: string;
	contentType: string;
	ttlHours: number;
	maxUses: number;
};

export type IssueInput = {
	ttlHours: number;
	maxUses: number;
};
