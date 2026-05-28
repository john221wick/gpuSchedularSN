<script>
	import { onMount, tick } from 'svelte';
	import { GetAppLogs, GetAppLogCount } from '../lib/api.js';

	let entries = $state([]);
	let offset = $state(0);
	let autoScroll = $state(true);
	let interval;

	async function fetchLogs() {
		try {
			const newEntries = await GetAppLogs(offset);
			if (newEntries && newEntries.length > 0) {
				entries = [...entries, ...newEntries];
				offset = entries.length;
				if (autoScroll) {
					await tick();
					const el = document.getElementById('app-log-container');
					if (el) el.scrollTop = el.scrollHeight;
				}
			}
		} catch (e) {
			console.error('Fetch logs failed:', e);
		}
	}

	function clearLogs() {
		entries = [];
		offset = 0;
	}

	function handleScroll(e) {
		const el = e.target;
		const atBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 40;
		autoScroll = atBottom;
	}

	onMount(() => {
		fetchLogs();
		interval = setInterval(fetchLogs, 1000);
		return () => clearInterval(interval);
	});
</script>

<div class="p-8 space-y-4 h-full flex flex-col max-w-[1200px]">
	<div class="flex items-center justify-between">
		<div>
			<h1 class="text-lg font-semibold" style="color: var(--text-primary);">Logs</h1>
			<p class="text-[13px] mt-0.5" style="color: var(--text-tertiary);">
				{entries.length} entries
			</p>
		</div>
		<div class="flex items-center gap-2">
			<button
				onclick={clearLogs}
				class="px-3 py-1.5 rounded-md text-[13px] font-medium transition-colors cursor-pointer"
				style="background: var(--bg-tertiary); color: var(--text-secondary);"
			>
				Clear
			</button>
			<label class="flex items-center gap-1.5 text-[12px] cursor-pointer" style="color: var(--text-secondary);">
				<input type="checkbox" bind:checked={autoScroll} class="w-3 h-3 rounded" />
				Auto-scroll
			</label>
		</div>
	</div>

	<div
		id="app-log-container"
		onscroll={handleScroll}
		class="flex-1 rounded-lg p-4 overflow-auto font-[JetBrains_Mono,monospace] text-[11px] leading-[1.7] min-h-0"
		style="background: #0d0d0d; color: #d4d4d4; border: 1px solid var(--border);"
	>
		{#if entries.length === 0}
			<span style="color: var(--text-muted);">No logs yet...</span>
		{:else}
			{#each entries as entry, i}
				<div class="flex gap-3 hover:bg-white/5 px-1 rounded">
					<span class="shrink-0 select-none" style="color: #636363;">{entry.time}</span>
					<span style="color: {entry.message.includes('failed') || entry.message.includes('error') || entry.message.includes('Error') ? '#ef4444' : entry.message.includes('Warning') ? '#eab308' : '#d4d4d4'};">{entry.message}</span>
				</div>
			{/each}
		{/if}
	</div>
</div>
