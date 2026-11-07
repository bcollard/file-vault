<script lang="ts">
	let { value, label }: { value: number; label?: string } = $props();
	const pct = $derived(Math.round(Math.min(1, Math.max(0, value)) * 100));
</script>

<div class="space-y-1.5">
	{#if label}
		<div class="flex justify-between text-xs text-muted-foreground">
			<span>{label}</span>
			<span class="font-mono">{pct}%</span>
		</div>
	{/if}
	<div
		class="h-2 w-full overflow-hidden rounded-full bg-muted"
		role="progressbar"
		aria-valuemin={0}
		aria-valuemax={100}
		aria-valuenow={pct}
	>
		<div class="h-full rounded-full bg-primary transition-[width] duration-200" style:width="{pct}%"></div>
	</div>
</div>
