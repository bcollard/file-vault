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
			'border-destructive/50 bg-destructive/10 text-destructive [&_svg]:text-destructive',
		warning:
			'border-warning/40 bg-warning/10 text-foreground [&_svg]:text-warning',
		success:
			'border-success/40 bg-success/10 text-foreground [&_svg]:text-success'
	};
</script>

<div role="alert" class={cn('relative w-full rounded-xl border p-4 text-sm', variants[variant], className)}>
	{@render children()}
</div>
