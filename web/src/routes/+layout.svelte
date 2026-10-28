<script lang="ts">
	import '../app.css';
	import ThemeToggle from '#lib/components/ThemeToggle.svelte';
	import { ShieldCheck } from 'lucide-svelte';
	import type { Snippet } from 'svelte';

	let { children }: { children: Snippet } = $props();
</script>

<div class="flex min-h-screen flex-col">
	<header class="border-b">
		<div class="container flex h-14 items-center justify-between">
			<a href="/" class="flex items-center gap-2 text-sm font-semibold">
				<ShieldCheck class="h-5 w-5" />
				file-vault
			</a>
			<ThemeToggle />
		</div>
	</header>

	<main class="container flex-1 py-8 md:py-12">
		<div class="mx-auto w-full max-w-xl">
			{@render children()}
		</div>
	</main>

	<footer class="border-t">
		<div class="container py-4 text-center text-xs text-muted-foreground">
			one-time file sharing &middot; capability URLs only
		</div>
	</footer>
</div>
