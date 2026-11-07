<script lang="ts">
	import { onMount } from 'svelte';
	import { CloudUpload, Settings2, CircleCheck, FileLock, X } from '@lucide/svelte';
	import Card from '#lib/components/ui/Card.svelte';
	import CardHeader from '#lib/components/ui/CardHeader.svelte';
	import CardTitle from '#lib/components/ui/CardTitle.svelte';
	import CardDescription from '#lib/components/ui/CardDescription.svelte';
	import CardContent from '#lib/components/ui/CardContent.svelte';
	import CardFooter from '#lib/components/ui/CardFooter.svelte';
	import Button from '#lib/components/ui/Button.svelte';
	import Label from '#lib/components/ui/Label.svelte';
	import Alert from '#lib/components/ui/Alert.svelte';
	import Progress from '#lib/components/ui/Progress.svelte';
	import CopyField from '#lib/components/CopyField.svelte';
	import { api, ApiError, type Limits } from '#lib/api.ts';
	import { b64urlEncode, ciphertextSize, deriveKeys, encryptMeta, newKey } from '#lib/crypto.ts';
	import { uploadEncrypted } from '#lib/transfer.ts';
	import { buildLink } from '#lib/links.ts';
	import { humanSize, shortTime, usesLabel } from '#lib/utils.ts';

	type Result = {
		name: string;
		size: number;
		downloadUrl: string;
		adminUrl: string;
		downloadExpiresAt: string;
		maxUses: number;
	};

	let limits = $state<Limits | null>(null);
	let file = $state<File | null>(null);
	let ttlHours = $state(48);
	let maxUses = $state(1);
	let advanced = $state(false);
	let dragging = $state(false);
	let phase = $state<'idle' | 'encrypting' | 'finalizing' | 'done'>('idle');
	let progress = $state(0);
	let error = $state<string | null>(null);
	let result = $state<Result | null>(null);
	let input = $state<HTMLInputElement>();

	const busy = $derived(phase === 'encrypting' || phase === 'finalizing');
	const tooBig = $derived(
		!!file && !!limits && ciphertextSize(file.size) > limits.max_upload_bytes
	);

	onMount(async () => {
		try {
			limits = await api.config();
			ttlHours = limits.default_ttl_hours;
			maxUses = limits.default_max_uses;
		} catch {
			error = 'Could not reach the server.';
		}
	});

	function pick(f: File | undefined | null) {
		if (!f || busy) return;
		file = f;
		error = null;
	}

	function onDrop(e: DragEvent) {
		e.preventDefault();
		dragging = false;
		pick(e.dataTransfer?.files?.[0]);
	}

	function guard(e: BeforeUnloadEvent) {
		if (busy) e.preventDefault();
	}

	async function upload() {
		if (!file || !limits || tooBig) return;
		error = null;
		phase = 'encrypting';
		progress = 0;
		try {
			const raw = newKey();
			const keys = await deriveKeys(raw);
			const total = ciphertextSize(file.size);
			const meta = await encryptMeta(keys, {
				name: file.name,
				type: file.type,
				size: file.size
			});
			const created = await api.createShare({
				size: total,
				meta,
				ttl_hours: ttlHours,
				max_uses: maxUses
			});
			await uploadEncrypted(created.upload_url, keys, file, total, (d, t) => (progress = d / t));
			phase = 'finalizing';
			const fin = await api.finalize(created.admin_token);
			const key = b64urlEncode(raw);
			result = {
				name: file.name,
				size: file.size,
				downloadUrl: buildLink('d', fin.download_token, key),
				adminUrl: buildLink('a', created.admin_token, key),
				downloadExpiresAt: fin.download_expires_at,
				maxUses: fin.max_uses
			};
			phase = 'done';
		} catch (e) {
			phase = 'idle';
			error =
				e instanceof ApiError
					? e.status === 413
						? 'This file is larger than the server allows.'
						: e.status === 429
							? 'Too many uploads. Wait a minute and try again.'
							: e.message
					: 'Upload failed. Check your connection and try again.';
		}
	}

	function reset() {
		file = null;
		result = null;
		phase = 'idle';
		progress = 0;
		if (input) input.value = '';
	}
</script>

<svelte:window onbeforeunload={guard} />

{#if phase === 'done' && result}
	<Card>
		<CardHeader>
			<CardTitle>
				<span class="flex items-center gap-2">
					<CircleCheck class="h-5 w-5 text-success" />
					Share ready
				</span>
			</CardTitle>
			<CardDescription>
				{result.name} &middot; {humanSize(result.size)} &middot; encrypted before upload
			</CardDescription>
		</CardHeader>
		<CardContent class="space-y-5">
			<div class="space-y-1.5">
				<CopyField label="Download link — send this to your recipient" value={result.downloadUrl} />
				<p class="text-xs text-muted-foreground">
					{usesLabel(result.maxUses)} &middot; expires {shortTime(result.downloadExpiresAt)}
				</p>
			</div>
			<CopyField variant="sensitive" label="Admin link — keep this for yourself" value={result.adminUrl} />
			<p class="text-xs text-muted-foreground">
				Both links contain the decryption key after the <code class="font-mono">#</code>. Anyone
				with a link can use it, and there is no way to recover a lost one.
			</p>
		</CardContent>
		<CardFooter class="justify-between gap-3">
			<Button variant="outline" onclick={reset}>Share another file</Button>
			<Button href={result.adminUrl}>Open admin console</Button>
		</CardFooter>
	</Card>
{:else}
	<div class="mb-8 space-y-2 text-center">
		<h1 class="text-3xl font-semibold tracking-tight md:text-4xl">Send a file, privately.</h1>
		<p class="text-muted-foreground">
			Encrypted in your browser, deleted after it's downloaded or expires.
		</p>
	</div>

	<Card>
		<CardContent class="space-y-5 pt-6">
			{#if error}
				<Alert variant="destructive">{error}</Alert>
			{/if}

			<button
				type="button"
				class="group flex w-full flex-col items-center justify-center gap-3 rounded-xl border-2 border-dashed px-6 py-10 text-center transition-colors {dragging
					? 'border-primary bg-accent'
					: 'border-input hover:border-primary/60 hover:bg-muted/50'} disabled:cursor-not-allowed disabled:opacity-60"
				disabled={busy}
				onclick={() => input?.click()}
				ondragover={(e) => {
					e.preventDefault();
					dragging = true;
				}}
				ondragleave={() => (dragging = false)}
				ondrop={onDrop}
			>
				{#if file}
					<FileLock class="h-8 w-8 text-primary" />
					<div class="space-y-0.5">
						<div class="max-w-full break-all font-medium">{file.name}</div>
						<div class="text-sm text-muted-foreground">{humanSize(file.size)}</div>
					</div>
					{#if !busy}
						<span class="text-xs text-muted-foreground">Click or drop to choose another file</span>
					{/if}
				{:else}
					<span
						class="flex h-12 w-12 items-center justify-center rounded-full bg-accent text-accent-foreground transition-transform group-hover:scale-105"
					>
						<CloudUpload class="h-6 w-6" />
					</span>
					<div class="space-y-0.5">
						<div class="font-medium">Drop a file here, or click to browse</div>
						{#if limits}
							<div class="text-sm text-muted-foreground">
								Up to {humanSize(limits.max_upload_bytes)}
							</div>
						{/if}
					</div>
				{/if}
			</button>
			<input
				bind:this={input}
				type="file"
				class="hidden"
				onchange={(e) => pick(e.currentTarget.files?.[0])}
			/>

			{#if tooBig && limits}
				<Alert variant="warning">
					This file is over the {humanSize(limits.max_upload_bytes)} limit.
				</Alert>
			{/if}

			{#if busy}
				<Progress
					value={phase === 'finalizing' ? 1 : progress}
					label={phase === 'finalizing' ? 'Finishing up…' : 'Encrypting and uploading…'}
				/>
			{/if}

			{#if limits}
				<details bind:open={advanced} class="rounded-xl border bg-muted/30">
					<summary
						class="flex cursor-pointer select-none items-center justify-between gap-2 p-3 text-sm font-medium"
					>
						<span class="flex items-center gap-2">
							<Settings2 class="h-4 w-4" />
							Link settings
						</span>
						<span class="text-xs font-normal text-muted-foreground">
							{ttlHours}h &middot; {usesLabel(maxUses)}
						</span>
					</summary>
					<div class="space-y-5 border-t p-4">
						<div class="space-y-2">
							<div class="flex items-center justify-between">
								<Label for="ttl_hours">Link expires after</Label>
								<span class="font-mono text-sm text-muted-foreground">
									{ttlHours}h ({(ttlHours / 24).toFixed(1)}d)
								</span>
							</div>
							<input
								id="ttl_hours"
								type="range"
								min={limits.min_ttl_hours}
								max={limits.max_ttl_hours}
								bind:value={ttlHours}
								disabled={busy}
								class="w-full accent-primary"
							/>
						</div>
						<div class="space-y-2">
							<div class="flex items-center justify-between">
								<Label for="max_uses">Maximum downloads</Label>
								<span class="font-mono text-sm text-muted-foreground">{usesLabel(maxUses)}</span>
							</div>
							<input
								id="max_uses"
								type="range"
								min="1"
								max={Math.min(limits.max_uses_per_token, 20)}
								bind:value={maxUses}
								disabled={busy}
								class="w-full accent-primary"
							/>
							<p class="text-xs text-muted-foreground">
								{maxUses === 1
									? 'Strongest setting: the link stops working after one download.'
									: 'The link works until the counter reaches zero.'}
							</p>
						</div>
					</div>
				</details>
			{/if}
		</CardContent>
		<CardFooter class="gap-3">
			{#if file && !busy}
				<Button variant="ghost" size="icon" onclick={reset} aria-label="Clear file">
					<X class="h-4 w-4" />
				</Button>
			{/if}
			<Button size="lg" class="flex-1" disabled={!file || !limits || busy || tooBig} onclick={upload}>
				{busy ? 'Uploading…' : 'Encrypt and upload'}
			</Button>
		</CardFooter>
	</Card>
{/if}
