<script lang="ts">
	import { cn } from '#lib/utils.ts';
	import type { Snippet } from 'svelte';
	import type { HTMLButtonAttributes, HTMLAnchorAttributes } from 'svelte/elements';

	type Variant = 'default' | 'secondary' | 'outline' | 'ghost' | 'destructive';
	type Size = 'default' | 'sm' | 'lg' | 'icon';

	type ButtonProps = {
		variant?: Variant;
		size?: Size;
		class?: string;
		children: Snippet;
		href?: string;
	} & Omit<HTMLButtonAttributes, 'class' | 'children'> &
		Omit<HTMLAnchorAttributes, 'class' | 'children'>;

	let {
		variant = 'default',
		size = 'default',
		class: className = '',
		children,
		href,
		...rest
	}: ButtonProps = $props();

	const variants: Record<Variant, string> = {
		default: 'bg-primary text-primary-foreground shadow-sm hover:bg-primary/90',
		secondary: 'bg-secondary text-secondary-foreground hover:bg-secondary/80',
		outline: 'border border-input bg-background hover:bg-accent hover:text-accent-foreground',
		ghost: 'hover:bg-accent hover:text-accent-foreground',
		destructive: 'bg-destructive text-destructive-foreground hover:bg-destructive/90'
	};

	const sizes: Record<Size, string> = {
		default: 'h-10 rounded-lg px-4 py-2',
		sm: 'h-9 rounded-md px-3 text-xs',
		lg: 'h-11 rounded-lg px-8 text-base',
		icon: 'h-10 w-10'
	};

	const classes = $derived(
		cn(
			'inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-md text-sm font-medium ring-offset-background transition-colors focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:pointer-events-none disabled:opacity-50',
			variants[variant],
			sizes[size],
			className
		)
	);
</script>

{#if href !== undefined}
	<a {href} class={classes} {...rest}>
		{@render children()}
	</a>
{:else}
	<button class={classes} {...rest}>
		{@render children()}
	</button>
{/if}
