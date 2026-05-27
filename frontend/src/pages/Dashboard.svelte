<script>
	import { onMount } from 'svelte';
	import { GetDashboard } from '../lib/api.js';

	let dashboard = $state(null);
	let interval;

	async function refresh() {
		try {
			dashboard = await GetDashboard();
		} catch (e) {
			console.error('Dashboard refresh failed:', e);
		}
	}

	onMount(() => {
		refresh();
		interval = setInterval(refresh, 2000);
		return () => clearInterval(interval);
	});

	function vramPct(total, used) {
		if (!total) return 0;
		return Math.round((used / total) * 100);
	}
</script>

<div class="p-8 space-y-6 max-w-[1000px]">
	<div>
		<h1 class="text-lg font-semibold" style="color: var(--text-primary);">Dashboard</h1>
		<p class="text-[13px] mt-0.5" style="color: var(--text-tertiary);">System overview</p>
	</div>

	{#if dashboard}
		<div class="grid grid-cols-4 gap-3">
			<div class="rounded-lg p-4" style="background: var(--bg-secondary); border: 1px solid var(--border);">
				<div class="text-[11px] font-medium uppercase tracking-wider" style="color: var(--text-tertiary);">GPUs</div>
				<div class="text-2xl font-semibold mt-2 font-[JetBrains_Mono,monospace]" style="color: var(--text-primary);">{dashboard.totalGPUs}</div>
				<div class="text-[12px] mt-1" style="color: var(--text-secondary);">{dashboard.freeGPUs} free</div>
			</div>

			<div class="rounded-lg p-4" style="background: var(--bg-secondary); border: 1px solid var(--border);">
				<div class="text-[11px] font-medium uppercase tracking-wider" style="color: var(--text-tertiary);">Jobs</div>
				<div class="text-2xl font-semibold mt-2 font-[JetBrains_Mono,monospace]" style="color: var(--text-primary);">{dashboard.runningJobs}</div>
				<div class="text-[12px] mt-1" style="color: var(--text-secondary);">{dashboard.queuedJobs} queued</div>
			</div>

			<div class="rounded-lg p-4" style="background: var(--bg-secondary); border: 1px solid var(--border);">
				<div class="text-[11px] font-medium uppercase tracking-wider" style="color: var(--text-tertiary);">Utilization</div>
				<div class="text-2xl font-semibold mt-2 font-[JetBrains_Mono,monospace]" style="color: var(--text-primary);">{dashboard.avgUtil.toFixed(0)}%</div>
				<div class="mt-2 h-1 rounded-full overflow-hidden" style="background: var(--bar-bg);">
					<div
						class="h-full rounded-full transition-all duration-500"
						class:bg-red-500={dashboard.avgUtil >= 90}
						class:bg-amber-500={dashboard.avgUtil >= 70 && dashboard.avgUtil < 90}
						style="width: {dashboard.avgUtil}%; {dashboard.avgUtil < 70 ? 'background: var(--bar-fill)' : ''}"
					></div>
				</div>
			</div>

			<div class="rounded-lg p-4" style="background: var(--bg-secondary); border: 1px solid var(--border);">
				<div class="text-[11px] font-medium uppercase tracking-wider" style="color: var(--text-tertiary);">VRAM</div>
				<div class="text-2xl font-semibold mt-2 font-[JetBrains_Mono,monospace]" style="color: var(--text-primary);">
					{vramPct(dashboard.totalVRAMMB, dashboard.usedVRAMMB)}%
				</div>
				<div class="text-[12px] mt-1" style="color: var(--text-secondary);">
					{(dashboard.usedVRAMMB / 1024).toFixed(1)} / {(dashboard.totalVRAMMB / 1024).toFixed(1)} GB
				</div>
			</div>
		</div>
	{:else}
		<div class="flex items-center justify-center h-32" style="color: var(--text-muted);">Loading...</div>
	{/if}
</div>
