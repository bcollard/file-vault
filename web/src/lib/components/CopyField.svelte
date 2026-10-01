<script lang="ts">
	import { Copy, Check } from 'lucide-svelte';
	import { cn } from '$lib/utils';

	let {
		value,
		label = undefined,
		variant = 'default',
		class: className = ''
	}: {
		value: string;
		label?: string;
		variant?: 'default' | 'sensitive';
		class?: string;
	} = $props();

	let copied = $state(false);

	async function copy() {
		try {
			await navigator.clipboard.writeText(value);
			copied = true;
			setTimeout(() => (copied = false), 1500);
		} catch {
			// no-op
		}
	}
</script>

<div class={cn('space-y-1.5', className)}>
	{#if label}
		<div class="flex items-center gap-2">
			<span
				class={cn(
					'text-xs font-semibold uppercase tracking-wider',
					variant === 'sensitive' ? 'text-destructive' : 'text-muted-foreground'
				)}
			>
				{label}
			</span>
			{#if variant === 'sensitive'}
				<span class="text-xs text-destructive">— save it now, you won't see it again</span>
			{/if}
		</div>
	{/if}
	<div
		class={cn(
			'flex items-center gap-2 rounded-md border bg-muted/40 p-2 font-mono text-xs',
			variant === 'sensitive' && 'border-destructive/40 bg-destructive/5'
		)}
	>
		<button
			type="button"
			class="flex-1 cursor-text overflow-hidden whitespace-nowrap text-left text-foreground"
			onclick={copy}
			title="Click to copy"
		>
			{value}
		</button>
		<button
			type="button"
			class="inline-flex h-7 w-7 shrink-0 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground"
			onclick={copy}
			aria-label={copied ? 'Copied' : 'Copy to clipboard'}
		>
			{#if copied}
				<Check class="h-4 w-4" />
			{:else}
				<Copy class="h-4 w-4" />
			{/if}
		</button>
	</div>
</div>
