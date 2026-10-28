<script lang="ts">
	import { enhance } from '$app/forms';
	import { invalidateAll } from '$app/navigation';
	import {
		FileText,
		Plus,
		Ban,
		Trash2,
		Clock,
		ArrowRight,
		CheckCircle2,
		AlertTriangle
	} from 'lucide-svelte';
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
	import { humanSize, shortTime, relativeTime } from '#lib/utils.ts';
	import type { TokenStatus } from '#lib/types.ts';

	let { data, form } = $props();

	let issueOpen = $state(false);
	let ttlHours = $state(data.limits.defaultTtlHours);
	let maxUses = $state(data.limits.defaultMaxUses);
	let busy = $state(false);

	const badgeVariant = (s: TokenStatus['status']) =>
		s === 'active'
			? ('success' as const)
			: s === 'exhausted'
				? ('muted' as const)
				: s === 'expired'
					? ('destructive' as const)
					: ('warning' as const);
</script>

{#if form?.deleted}
	<Card>
		<CardHeader>
			<CardTitle>
				<span class="flex items-center gap-2">
					<CheckCircle2 class="h-5 w-5" />
					Share deleted
				</span>
			</CardTitle>
			<CardDescription>The file and all download links are gone.</CardDescription>
		</CardHeader>
		<CardFooter>
			<Button href="/">Share another file</Button>
		</CardFooter>
	</Card>
{:else}
	<div class="space-y-6">
		<Card>
			<CardHeader>
				<div class="flex items-start justify-between gap-4">
					<div>
						<CardTitle>
							<span class="flex items-center gap-2">
								<FileText class="h-5 w-5" />
								{data.share.filename}
							</span>
						</CardTitle>
						<CardDescription>
							{humanSize(data.share.size)} &middot; created {shortTime(data.share.created_at)} &middot;
							expires {relativeTime(data.share.expires_at)}
						</CardDescription>
					</div>
				</div>
			</CardHeader>

			<CardContent class="space-y-4">
				{#if form?.error}
					<Alert variant="destructive">{form.error}</Alert>
				{/if}

				{#if form?.issued}
					<Alert variant="success">
						<div class="space-y-2">
							<div class="text-xs font-semibold uppercase tracking-wider">
								New download link
							</div>
							<CopyField value={form.issued.url} />
							<p class="text-xs">
								{form.issued.maxUses === 1
									? 'One-time use'
									: `${form.issued.maxUses} uses`} &middot; expires {shortTime(form.issued.expiresAt)}
							</p>
						</div>
					</Alert>
				{/if}

				{#if form?.revoked !== undefined}
					<Alert variant="warning">
						<div class="flex items-center gap-2">
							<AlertTriangle class="h-4 w-4" />
							Revoked {form.revoked} active link{form.revoked === 1 ? '' : 's'}.
						</div>
					</Alert>
				{/if}

				<form
					method="POST"
					action="?/issue"
					use:enhance={() => {
						busy = true;
						return async ({ update }) => {
							await update();
							busy = false;
							issueOpen = false;
							invalidateAll();
						};
					}}
				>
					<details bind:open={issueOpen} class="rounded-lg border bg-muted/30">
						<summary
							class="flex cursor-pointer select-none items-center justify-between p-3 text-sm font-medium"
						>
							<span class="flex items-center gap-2">
								<Plus class="h-4 w-4" />
								Issue a new download link
							</span>
							<span class="text-xs text-muted-foreground">
								{ttlHours}h &middot; {maxUses} use{maxUses === 1 ? '' : 's'}
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
									name="ttl_hours"
									type="range"
									min={data.limits.minTtlHours}
									max={data.limits.maxTtlHours}
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
									name="max_uses"
									type="range"
									min="1"
									max={Math.min(data.limits.maxUsesPerToken, 20)}
									bind:value={maxUses}
									class="w-full accent-primary"
								/>
							</div>
							<Button type="submit" class="w-full" disabled={busy}>
								<ArrowRight class="h-4 w-4" />
								{busy ? 'Issuing…' : 'Generate'}
							</Button>
						</div>
					</details>
				</form>
			</CardContent>
		</Card>

		<Card>
			<CardHeader>
				<CardTitle>Download links</CardTitle>
				<CardDescription>
					{data.tokens.length === 0
						? 'No links issued yet.'
						: `${data.tokens.length} link${data.tokens.length === 1 ? '' : 's'} total`}
				</CardDescription>
			</CardHeader>
			<CardContent>
				{#if data.tokens.length === 0}
					<p class="text-sm italic text-muted-foreground">
						Issue your first link above.
					</p>
				{:else}
					<div class="overflow-hidden rounded-md border">
						<table class="w-full text-sm">
							<thead class="bg-muted/40">
								<tr class="text-left text-xs uppercase tracking-wider text-muted-foreground">
									<th class="px-3 py-2 font-medium">ID</th>
									<th class="px-3 py-2 font-medium">Status</th>
									<th class="px-3 py-2 font-medium">Uses</th>
									<th class="px-3 py-2 font-medium">Detail</th>
								</tr>
							</thead>
							<tbody class="divide-y">
								{#each data.tokens as t (t.short_id)}
									<tr>
										<td class="px-3 py-2 font-mono text-xs">{t.short_id}</td>
										<td class="px-3 py-2">
											<Badge variant={badgeVariant(t.status)}>{t.status}</Badge>
										</td>
										<td class="px-3 py-2 font-mono text-xs">
											{t.uses_remaining}/{t.max_uses}
										</td>
										<td class="px-3 py-2 text-xs text-muted-foreground">
											{#if t.status === 'active'}
												<span class="inline-flex items-center gap-1">
													<Clock class="h-3 w-3" />
													expires {relativeTime(t.expires_at)}
												</span>
											{:else if t.last_used_at}
												{shortTime(t.last_used_at)} &middot; {t.last_used_from_ip}
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
				{/if}
			</CardContent>
		</Card>

		<Card class="border-destructive/40">
			<CardHeader>
				<CardTitle class="text-base">Danger zone</CardTitle>
				<CardDescription>Irreversible operations.</CardDescription>
			</CardHeader>
			<CardContent class="space-y-3">
				<form
					method="POST"
					action="?/revoke"
					use:enhance={({ cancel }) => {
						if (!confirm('Revoke all active download links?')) {
							cancel();
							return;
						}
						busy = true;
						return async ({ update }) => {
							await update();
							busy = false;
							invalidateAll();
						};
					}}
				>
					<Button type="submit" variant="outline" class="w-full" disabled={busy}>
						<Ban class="h-4 w-4" />
						Revoke active links
					</Button>
				</form>
				<form
					method="POST"
					action="?/delete"
					use:enhance={({ cancel }) => {
						if (
							!confirm(
								'Delete this share permanently? The file and every link will be removed.'
							)
						) {
							cancel();
							return;
						}
						busy = true;
						return async ({ update }) => {
							await update();
							busy = false;
						};
					}}
				>
					<Button type="submit" variant="destructive" class="w-full" disabled={busy}>
						<Trash2 class="h-4 w-4" />
						Delete share
					</Button>
				</form>
			</CardContent>
		</Card>
	</div>
{/if}
