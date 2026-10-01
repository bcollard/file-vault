<script lang="ts">
	import { enhance } from '$app/forms';
	import { Upload, Settings2, Sparkles } from 'lucide-svelte';
	import Card from '$lib/components/ui/Card.svelte';
	import CardHeader from '$lib/components/ui/CardHeader.svelte';
	import CardTitle from '$lib/components/ui/CardTitle.svelte';
	import CardDescription from '$lib/components/ui/CardDescription.svelte';
	import CardContent from '$lib/components/ui/CardContent.svelte';
	import CardFooter from '$lib/components/ui/CardFooter.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import Label from '$lib/components/ui/Label.svelte';
	import Alert from '$lib/components/ui/Alert.svelte';
	import CopyField from '$lib/components/CopyField.svelte';
	import { humanSize, shortTime } from '$lib/utils';

	let { data, form } = $props();

	let submitting = $state(false);
	let advanced = $state(false);
	let ttlHours = $state(data.limits.defaultTtlHours);
	let maxUses = $state(data.limits.defaultMaxUses);
	let selectedFile = $state<File | null>(null);
</script>

{#if form?.success}
	<Card>
		<CardHeader>
			<CardTitle>
				<span class="flex items-center gap-2">
					<Sparkles class="h-5 w-5" />
					Share ready
				</span>
			</CardTitle>
			<CardDescription>
				{form.share.filename} &middot; {humanSize(form.share.size)} &middot; expires {shortTime(form.share.expiresAt)}
			</CardDescription>
		</CardHeader>
		<CardContent class="space-y-5">
			<CopyField
				label="Download URL"
				value={form.share.downloadUrl}
			/>
			<p class="-mt-3 text-xs text-muted-foreground">
				{form.share.downloadMaxUses === 1
					? 'One-time use'
					: `${form.share.downloadMaxUses} uses`} &middot; expires {shortTime(form.share.downloadExpiresAt)}
			</p>

			<CopyField
				variant="sensitive"
				label="Admin URL"
				value={form.share.adminUrl}
			/>
		</CardContent>
		<CardFooter class="justify-between">
			<Button variant="outline" href="/">Share another file</Button>
			<Button href={form.share.adminUrl}>Open admin console</Button>
		</CardFooter>
	</Card>
{:else}
	<Card>
		<CardHeader>
			<CardTitle>Share a file</CardTitle>
			<CardDescription>
				You'll get two URLs: a download link to send your recipient and an admin link to manage the
				share.
			</CardDescription>
		</CardHeader>
		<form
			method="POST"
			enctype="multipart/form-data"
			use:enhance={() => {
				submitting = true;
				return async ({ update }) => {
					await update();
					submitting = false;
				};
			}}
		>
			<CardContent class="space-y-5">
				{#if form?.error}
					<Alert variant="destructive">{form.error}</Alert>
				{/if}

				<div class="space-y-2">
					<Label for="file">File</Label>
					<Input
						id="file"
						name="file"
						type="file"
						required
						class="cursor-pointer"
						onchange={(e) => {
							const target = e.currentTarget as HTMLInputElement;
							selectedFile = target.files?.[0] ?? null;
						}}
					/>
					{#if selectedFile}
						<p class="text-xs text-muted-foreground">
							{selectedFile.name} &middot; {humanSize(selectedFile.size)}
						</p>
					{/if}
				</div>

				<details bind:open={advanced} class="rounded-lg border bg-muted/30">
					<summary
						class="flex cursor-pointer select-none items-center justify-between gap-2 p-3 text-sm font-medium"
					>
						<span class="flex items-center gap-2">
							<Settings2 class="h-4 w-4" />
							Advanced settings
						</span>
						<span class="text-xs text-muted-foreground">
							{ttlHours}h &middot; {maxUses} use{maxUses === 1 ? '' : 's'}
						</span>
					</summary>
					<div class="space-y-4 border-t p-4">
						<div class="space-y-2">
							<div class="flex items-center justify-between">
								<Label for="ttl_hours">Link expires after</Label>
								<span class="text-sm font-mono text-muted-foreground">
									{ttlHours}h ({(ttlHours / 24).toFixed(1)}d)
								</span>
							</div>
							<input
								id="ttl_hours"
								name="ttl_hours"
								type="range"
								min={data.limits.minTtlHours}
								max={data.limits.maxTtlHours}
								bind:value={ttlHours}
								class="w-full accent-primary"
							/>
							<div class="flex justify-between text-xs text-muted-foreground">
								<span>{data.limits.minTtlHours}h</span>
								<span>{data.limits.maxTtlHours}h</span>
							</div>
						</div>

						<div class="space-y-2">
							<div class="flex items-center justify-between">
								<Label for="max_uses">Maximum downloads</Label>
								<span class="text-sm font-mono text-muted-foreground">
									{maxUses === 1 ? 'one-time' : `${maxUses} downloads`}
								</span>
							</div>
							<input
								id="max_uses"
								name="max_uses"
								type="range"
								min="1"
								max={Math.min(data.limits.maxUsesPerToken, 20)}
								bind:value={maxUses}
								class="w-full accent-primary"
							/>
							<p class="text-xs text-muted-foreground">
								{maxUses === 1
									? "Strongest default — one click and the link's done."
									: 'The link works until the counter hits zero.'}
							</p>
						</div>
					</div>
				</details>
			</CardContent>
			<CardFooter>
				<Button type="submit" class="w-full" disabled={submitting}>
					<Upload class="h-4 w-4" />
					{submitting ? 'Uploading…' : 'Upload'}
				</Button>
			</CardFooter>
		</form>
	</Card>
{/if}
