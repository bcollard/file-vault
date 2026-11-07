<script lang="ts">
	import { cn } from '#lib/utils.ts';
	import type { Snippet } from 'svelte';

	let { class: className = '', children }: { class?: string; children: Snippet } = $props();
</script>

<div class={cn('rounded-2xl border bg-card text-card-foreground shadow-sm shadow-black/5', className)}>
	{@render children()}
</div>
