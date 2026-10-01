<script lang="ts">
	import { cn } from '$lib/utils';
	import type { Snippet } from 'svelte';

	type Variant = 'default' | 'secondary' | 'outline' | 'success' | 'warning' | 'destructive' | 'muted';

	let {
		variant = 'default',
		class: className = '',
		children
	}: { variant?: Variant; class?: string; children: Snippet } = $props();

	const variants: Record<Variant, string> = {
		default: 'border-transparent bg-primary text-primary-foreground',
		secondary: 'border-transparent bg-secondary text-secondary-foreground',
		outline: 'text-foreground',
		success: 'border-transparent bg-[hsl(var(--success)/0.15)] text-[hsl(var(--success))]',
		warning: 'border-transparent bg-[hsl(var(--warning)/0.15)] text-[hsl(var(--warning))]',
		destructive: 'border-transparent bg-destructive/15 text-destructive',
		muted: 'border-transparent bg-muted text-muted-foreground'
	};
</script>

<span
	class={cn(
		'inline-flex items-center rounded-md border px-2 py-0.5 text-xs font-medium',
		variants[variant],
		className
	)}
>
	{@render children()}
</span>
