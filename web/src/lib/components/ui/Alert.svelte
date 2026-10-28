<script lang="ts">
	import { cn } from '#lib/utils.ts';
	import type { Snippet } from 'svelte';

	type Variant = 'default' | 'destructive' | 'warning' | 'success';

	let {
		variant = 'default',
		class: className = '',
		children
	}: { variant?: Variant; class?: string; children: Snippet } = $props();

	const variants: Record<Variant, string> = {
		default: 'border bg-background text-foreground',
		destructive:
			'border-destructive/50 bg-destructive/10 text-destructive [&>svg]:text-destructive',
		warning:
			'border-[hsl(var(--warning)/0.4)] bg-[hsl(var(--warning)/0.1)] text-[hsl(var(--warning))]',
		success:
			'border-[hsl(var(--success)/0.4)] bg-[hsl(var(--success)/0.1)] text-[hsl(var(--success))]'
	};
</script>

<div role="alert" class={cn('relative w-full rounded-lg border p-4 text-sm', variants[variant], className)}>
	{@render children()}
</div>
