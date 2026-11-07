<script lang="ts">
	// Recipient page. Loading it never consumes a use: it only peeks at the
	// token. The burn happens when the recipient clicks Download, so link
	// previews (Slack, Outlook Safe Links, iMessage) can't use up the link.
	import { onMount } from 'svelte';
	import { Download, FileLock, CircleCheck, TriangleExclamationPoint, LoaderCircle } from '@lucide/svelte';
	import Card from '#lib/components/ui/Card.svelte';
	import CardHeader from '#lib/components/ui/CardHeader.svelte';
	import CardTitle from '#lib/components/ui/CardTitle.svelte';
	import CardDescription from '#lib/components/ui/CardDescription.svelte';
	import CardContent from '#lib/components/ui/CardContent.svelte';
	import CardFooter from '#lib/components/ui/CardFooter.svelte';
	import Button from '#lib/components/ui/Button.svelte';
	import Alert from '#lib/components/ui/Alert.svelte';
	import Progress from '#lib/components/ui/Progress.svelte';
	import { api, ApiError, type Peek } from '#lib/api.ts';
	import { b64urlDecode, deriveKeys, decryptMeta, type FileMeta, type Keys } from '#lib/crypto.ts';
	import { downloadDecrypted, saveBlob } from '#lib/transfer.ts';
	import { parseFragment } from '#lib/links.ts';
	import { humanSize, relativeTime } from '#lib/utils.ts';

	type View = 'loading' | 'invalid' | 'gone' | 'ready' | 'downloading' | 'done' | 'failed';

	let view = $state<View>('loading');
	let meta = $state<FileMeta | null>(null);
	let peek = $state<Peek | null>(null);
	let progress = $state(0);
	let message = $state('');
	let token = '';
	let keys: Keys | null = null;

	onMount(async () => {
		const cap = parseFragment(location.hash);
		if (!cap) {
			view = 'invalid';
			return;
		}
		token = cap.token;
		try {
			keys = await deriveKeys(b64urlDecode(cap.key));
			peek = await api.peek(token);
			meta = await decryptMeta(keys, peek.meta);
			view = 'ready';
		} catch (e) {
			view = e instanceof ApiError && e.notFound ? 'gone' : 'invalid';
		}
	});

	async function download() {
		if (!keys || !meta) return;
		view = 'downloading';
		progress = 0;
		try {
			const burn = await api.burn(token);
			if (peek) peek.uses_remaining -= 1;
			const blob = await downloadDecrypted(burn.url, keys, burn.size, meta, (d, t) => (progress = d / t));
			saveBlob(blob, meta.name);
			view = 'done';
		} catch (e) {
			if (e instanceof ApiError && e.notFound) {
				view = 'gone';
				return;
			}
			view = 'failed';
			message =
				e instanceof ApiError
					? e.message
					: 'The download or decryption failed. This use of the link has been counted.';
		}
	}
</script>

<svelte:head>
	<title>Download · file-vault</title>
</svelte:head>

{#if view === 'loading'}
	<div class="flex justify-center py-16 text-muted-foreground">
		<LoaderCircle class="h-6 w-6 animate-spin" />
	</div>
{:else if view === 'invalid' || view === 'gone'}
	<Card>
		<CardHeader>
			<CardTitle>
				<span class="flex items-center gap-2">
					<TriangleExclamationPoint class="h-5 w-5 text-warning" />
					{view === 'gone' ? 'This link no longer works' : 'This link is incomplete'}
				</span>
			</CardTitle>
			<CardDescription>
				{view === 'gone'
					? 'It has expired, been used up, or been revoked by the sender.'
					: 'The part after the # is missing or damaged. Ask the sender to copy the whole link again.'}
			</CardDescription>
		</CardHeader>
		<CardFooter>
			<Button variant="outline" href="/">Share your own file</Button>
		</CardFooter>
	</Card>
{:else if meta && peek}
	<Card>
		<CardHeader>
			<CardTitle>
				{#if view === 'done'}
					<span class="flex items-center gap-2">
						<CircleCheck class="h-5 w-5 text-success" />
						Downloaded and decrypted
					</span>
				{:else}
					Someone shared a file with you
				{/if}
			</CardTitle>
			<CardDescription>Decrypted in your browser with the key from this link.</CardDescription>
		</CardHeader>
		<CardContent class="space-y-5">
			<div class="flex items-center gap-4 rounded-xl border bg-muted/40 p-4">
				<span
					class="flex h-11 w-11 shrink-0 items-center justify-center rounded-lg bg-accent text-accent-foreground"
				>
					<FileLock class="h-5 w-5" />
				</span>
				<div class="min-w-0">
					<div class="break-all font-medium">{meta.name}</div>
					<div class="text-sm text-muted-foreground">
						{humanSize(meta.size)} &middot; link expires {relativeTime(peek.expires_at)}
					</div>
				</div>
			</div>

			{#if view === 'downloading'}
				<Progress value={progress} label="Downloading and decrypting…" />
			{:else if view === 'failed'}
				<Alert variant="destructive">{message}</Alert>
			{:else if view === 'done'}
				<p class="text-sm text-muted-foreground">
					{peek.uses_remaining > 0
						? `The link can be used ${peek.uses_remaining} more time${peek.uses_remaining === 1 ? '' : 's'}.`
						: 'That was the last download. The link no longer works.'}
				</p>
			{:else}
				<p class="text-sm text-muted-foreground">
					{peek.uses_remaining === 1
						? 'This link works for one download. After you click, it stops working.'
						: `This link has ${peek.uses_remaining} downloads left. Clicking uses one.`}
				</p>
			{/if}
		</CardContent>
		{#if view === 'ready' || view === 'failed'}
			<CardFooter>
				<Button size="lg" class="w-full" onclick={download} disabled={peek.uses_remaining < 1}>
					<Download class="h-4 w-4" />
					Download
				</Button>
			</CardFooter>
		{/if}
	</Card>
{/if}
