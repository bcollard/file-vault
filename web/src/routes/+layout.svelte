<script lang="ts">
	import '../app.css';
	import ThemeToggle from '#lib/components/ThemeToggle.svelte';
	import { ShieldCheck, LockKeyhole } from '@lucide/svelte';
	import type { Snippet } from 'svelte';

	let { children }: { children: Snippet } = $props();
</script>

<svelte:head>
	<title>file-vault</title>
</svelte:head>

<div class="flex min-h-screen flex-col">
	<header>
		<div class="container flex h-16 items-center justify-between">
			<a href="/" class="flex items-center gap-2 font-semibold tracking-tight">
				<span class="flex h-8 w-8 items-center justify-center rounded-lg bg-primary text-primary-foreground">
					<ShieldCheck class="h-4.5 w-4.5" />
				</span>
				file-vault
			</a>
			<ThemeToggle />
		</div>
	</header>

	<main class="container flex-1 py-6 md:py-12">
		<div class="mx-auto w-full max-w-xl">
			{@render children()}
		</div>
	</main>

	<footer>
		<div
			class="container flex items-center justify-center gap-1.5 py-6 text-center text-xs text-muted-foreground"
		>
			<LockKeyhole class="h-3.5 w-3.5" />
			End-to-end encrypted in your browser. The server never sees your file, its name, or the key.
		</div>
	</footer>
</div>
