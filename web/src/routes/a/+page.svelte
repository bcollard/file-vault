<script lang="ts">
	// Admin console. The admin link carries the admin token and the file key in
	// its fragment; the key is needed to show the file name and to build new
	// download links.
	import { onMount } from 'svelte';
	import {
		FileLock,
		Plus,
		Ban,
		Trash,
		Clock,
		CircleCheck,
		TriangleExclamationPoint,
		LoaderCircle
	} from '@lucide/svelte';
	import Card from '#lib/components/ui/Card.svelte';
	import CardHeader from '#lib/components/ui/CardHeader.svelte';
	import CardTitle from '#lib/components/ui/CardTitle.svelte';
	import CardDescription from '#lib/components/ui/CardDescription.svelte';
	import CardContent from '#lib/components/ui/CardContent.svelte';
	import CardFooter from '#lib/components/ui/CardFooter.svelte';
	import Button from '#lib/components/ui/Button.svelte';
	import Badge from '#lib/components/ui/Badge.svelte';
	import Alert from '#lib/components/ui/Alert.svelte';
	import Label from '#lib/components/ui/Label.svelte';
	import CopyField from '#lib/components/CopyField.svelte';
	import { api, ApiError, type AdminShare, type Limits, type TokenStatus } from '#lib/api.ts';
	import { b64urlDecode, decryptMeta, deriveKeys, type FileMeta } from '#lib/crypto.ts';
	import { buildLink, parseFragment, type Capability } from '#lib/links.ts';
	import { humanSize, shortTime, relativeTime, usesLabel } from '#lib/utils.ts';

	let view = $state<'loading' | 'invalid' | 'gone' | 'ready' | 'deleted'>('loading');
	let share = $state<AdminShare | null>(null);
	let meta = $state<FileMeta | null>(null);
	let limits = $state<Limits | null>(null);
	let issueOpen = $state(false);
	let ttlHours = $state(48);
	let maxUses = $state(1);
	let busy = $state(false);
	let error = $state<string | null>(null);
	let issued = $state<{ url: string; expiresAt: string; maxUses: number } | null>(null);
	let revoked = $state<number | null>(null);
	let cap: Capability | null = null;

	const badgeVariant = (s: TokenStatus['status']) =>
		s === 'active'
			? ('success' as const)
			: s === 'exhausted'
				? ('muted' as const)
				: s === 'expired'
					? ('destructive' as const)
					: ('warning' as const);

	async function refresh() {
		if (!cap) return;
		share = await api.admin(cap.token);
	}

	onMount(async () => {
		cap = parseFragment(location.hash);
		if (!cap) {
			view = 'invalid';
			return;
		}
		try {
			const keys = await deriveKeys(b64urlDecode(cap.key));
			const [s, l] = await Promise.all([api.admin(cap.token), api.config()]);
			share = s;
			limits = l;
			ttlHours = l.default_ttl_hours;
			maxUses = l.default_max_uses;
			meta = await decryptMeta(keys, s.meta);
			view = 'ready';
		} catch (e) {
			view = e instanceof ApiError && e.notFound ? 'gone' : 'invalid';
		}
	});

	async function act(fn: () => Promise<void>) {
		if (!cap) return;
		busy = true;
		error = null;
		try {
			await fn();
		} catch (e) {
			if (e instanceof ApiError && e.notFound) view = 'gone';
			else error = e instanceof ApiError ? e.message : 'Request failed.';
		} finally {
			busy = false;
		}
	}

	const issue = () =>
		act(async () => {
			const r = await api.issue(cap!.token, ttlHours, maxUses);
			issued = { url: buildLink('d', r.download_token, cap!.key), expiresAt: r.expires_at, maxUses: r.max_uses };
			revoked = null;
			issueOpen = false;
			await refresh();
		});

	const revoke = () => {
		if (!confirm('Revoke every active download link?')) return;
		return act(async () => {
			revoked = (await api.revoke(cap!.token)).revoked;
			issued = null;
			await refresh();
		});
	};

	const remove = () => {
		if (!confirm('Delete this share permanently? The file and every link are removed.')) return;
		return act(async () => {
			await api.remove(cap!.token);
			view = 'deleted';
		});
	};
</script>

<svelte:head>
	<title>Admin · file-vault</title>
</svelte:head>

{#if view === 'loading'}
	<div class="flex justify-center py-16 text-muted-foreground">
		<LoaderCircle class="h-6 w-6 animate-spin" />
	</div>
{:else if view === 'deleted'}
	<Card>
		<CardHeader>
			<CardTitle>
				<span class="flex items-center gap-2">
					<CircleCheck class="h-5 w-5 text-success" />
					Share deleted
				</span>
			</CardTitle>
			<CardDescription>The encrypted file and every download link are gone.</CardDescription>
		</CardHeader>
		<CardFooter>
			<Button href="/">Share another file</Button>
		</CardFooter>
	</Card>
{:else if view === 'invalid' || view === 'gone'}
	<Card>
		<CardHeader>
			<CardTitle>
				<span class="flex items-center gap-2">
					<TriangleExclamationPoint class="h-5 w-5 text-warning" />
					{view === 'gone' ? 'This share no longer exists' : 'This admin link is incomplete'}
				</span>
			</CardTitle>
			<CardDescription>
				{view === 'gone'
					? 'It expired or was deleted.'
					: 'The part after the # is missing or damaged.'}
			</CardDescription>
		</CardHeader>
		<CardFooter>
			<Button variant="outline" href="/">Share a file</Button>
		</CardFooter>
	</Card>
{:else if share && meta && limits}
	<div class="space-y-6">
		<Card>
			<CardHeader>
				<div class="flex items-start gap-4">
					<span
						class="flex h-11 w-11 shrink-0 items-center justify-center rounded-lg bg-accent text-accent-foreground"
					>
						<FileLock class="h-5 w-5" />
					</span>
					<div class="min-w-0 space-y-1.5">
						<CardTitle class="break-all">{meta.name}</CardTitle>
						<CardDescription>
							{humanSize(meta.size)} &middot; created {shortTime(share.created_at)} &middot; deleted
							{relativeTime(share.expires_at)}
						</CardDescription>
					</div>
				</div>
			</CardHeader>

			<CardContent class="space-y-4">
				{#if share.status === 'pending'}
					<Alert variant="warning">
						The upload never finished, so there is nothing to download. Delete this share and
						upload the file again.
					</Alert>
				{/if}
				{#if error}
					<Alert variant="destructive">{error}</Alert>
				{/if}
				{#if issued}
					<Alert variant="success">
						<div class="space-y-2">
							<CopyField label="New download link" value={issued.url} />
							<p class="text-xs text-muted-foreground">
								{usesLabel(issued.maxUses)} &middot; expires {shortTime(issued.expiresAt)}
							</p>
						</div>
					</Alert>
				{/if}
				{#if revoked !== null}
					<Alert variant="warning">
						<span class="flex items-center gap-2">
							<TriangleExclamationPoint class="h-4 w-4" />
							Revoked {revoked} active link{revoked === 1 ? '' : 's'}.
						</span>
					</Alert>
				{/if}

				{#if share.status === 'ready'}
					<details bind:open={issueOpen} class="rounded-xl border bg-muted/30">
						<summary
							class="flex cursor-pointer select-none items-center justify-between p-3 text-sm font-medium"
						>
							<span class="flex items-center gap-2">
								<Plus class="h-4 w-4" />
								Issue a new download link
							</span>
							<span class="text-xs font-normal text-muted-foreground">
								{ttlHours}h &middot; {usesLabel(maxUses)}
							</span>
						</summary>
						<div class="space-y-4 border-t p-4">
							<div class="space-y-2">
								<div class="flex items-center justify-between">
									<Label for="issue_ttl">Expires after</Label>
									<span class="font-mono text-sm text-muted-foreground">{ttlHours}h</span>
								</div>
								<input
									id="issue_ttl"
									type="range"
									min={limits.min_ttl_hours}
									max={limits.max_ttl_hours}
									bind:value={ttlHours}
									class="w-full accent-primary"
								/>
							</div>
							<div class="space-y-2">
								<div class="flex items-center justify-between">
									<Label for="issue_max">Maximum downloads</Label>
									<span class="font-mono text-sm text-muted-foreground">{maxUses}</span>
								</div>
								<input
									id="issue_max"
									type="range"
									min="1"
									max={Math.min(limits.max_uses_per_token, 20)}
									bind:value={maxUses}
									class="w-full accent-primary"
								/>
							</div>
							<Button class="w-full" disabled={busy} onclick={issue}>
								{busy ? 'Issuing…' : 'Generate link'}
							</Button>
						</div>
					</details>
				{/if}
			</CardContent>
		</Card>

		<Card>
			<CardHeader>
				<CardTitle class="text-base">Download links</CardTitle>
				<CardDescription>
					{share.tokens.length === 0
						? 'No links issued yet.'
						: `${share.tokens.length} link${share.tokens.length === 1 ? '' : 's'} total`}
				</CardDescription>
			</CardHeader>
			{#if share.tokens.length > 0}
				<CardContent>
					<div class="overflow-x-auto rounded-xl border">
						<table class="w-full text-sm">
							<thead class="bg-muted/40">
								<tr class="text-left text-xs uppercase tracking-wider text-muted-foreground">
									<th class="px-3 py-2 font-medium">ID</th>
									<th class="px-3 py-2 font-medium">Status</th>
									<th class="px-3 py-2 font-medium">Left</th>
									<th class="px-3 py-2 font-medium">Detail</th>
								</tr>
							</thead>
							<tbody class="divide-y">
								{#each share.tokens as t (t.short_id)}
									<tr>
										<td class="px-3 py-2 font-mono text-xs">{t.short_id}</td>
										<td class="px-3 py-2">
											<Badge variant={badgeVariant(t.status)}>{t.status}</Badge>
										</td>
										<td class="px-3 py-2 font-mono text-xs">{t.uses_remaining}/{t.max_uses}</td>
										<td class="px-3 py-2 text-xs text-muted-foreground">
											{#if t.status === 'active' && !t.last_used_at}
												<span class="inline-flex items-center gap-1">
													<Clock class="h-3 w-3" />
													expires {relativeTime(t.expires_at)}
												</span>
											{:else if t.last_used_at}
												last used {shortTime(t.last_used_at)} &middot; {t.last_used_from_ip}
											{:else if t.status === 'expired'}
												expired {relativeTime(t.expires_at)}
											{:else}
												—
											{/if}
										</td>
									</tr>
								{/each}
							</tbody>
						</table>
					</div>
				</CardContent>
			{/if}
		</Card>

		<Card class="border-destructive/40">
			<CardHeader>
				<CardTitle class="text-base">Revoke or delete</CardTitle>
				<CardDescription>These can't be undone.</CardDescription>
			</CardHeader>
			<CardContent class="flex flex-col gap-3 sm:flex-row">
				<Button variant="outline" class="flex-1" disabled={busy} onclick={revoke}>
					<Ban class="h-4 w-4" />
					Revoke active links
				</Button>
				<Button variant="destructive" class="flex-1" disabled={busy} onclick={remove}>
					<Trash class="h-4 w-4" />
					Delete share
				</Button>
			</CardContent>
		</Card>
	</div>
{/if}
