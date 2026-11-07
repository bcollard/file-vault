<script lang="ts">
	import { cn } from '#lib/utils.ts';
	import type { HTMLInputAttributes } from 'svelte/elements';

	let {
		class: className = '',
		value = $bindable(),
		...rest
	}: { class?: string; value?: HTMLInputAttributes['value'] } & HTMLInputAttributes = $props();
</script>

<input
	bind:value
	class={cn(
		'flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50 file:border-0 file:bg-transparent file:text-sm file:font-medium file:text-foreground',
		className
	)}
	{...rest}
/>
