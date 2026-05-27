<script>
	import { onMount } from 'svelte';
	import { GetDashboard, GetDevices } from '../lib/api.js';

	let dashboard = $state(null);
	let devices = $state([]);
	let interval;

	async function refresh() {
		try {
			dashboard = await GetDashboard();
			devices = await GetDevices();
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

<div class="p-8 space-y-8 max-w-[1200px]">
	<div>
		<h1 class="text-lg font-semibold" style="color: var(--text-primary);">Dashboard</h1>
		<p class="text-[13px] mt-0.5" style="color: var(--text-tertiary);">System overview</p>
	</div>

	{#if dashboard}
		<div class="grid grid-cols-4 gap-3">
			<div class="rounded-lg p-4" style="background: var(--bg-secondary); border: 1px solid var(--border);">
				<div class="text-[11px] font-medium uppercase tracking-wider" style="color: var(--text-tertiary);">Total GPUs</div>
				<div class="text-2xl font-semibold mt-2 font-[JetBrains_Mono,monospace]" style="color: var(--text-primary);">{dashboard.totalGPUs}</div>
				<div class="text-[12px] mt-1" style="color: var(--text-secondary);">{dashboard.freeGPUs} available</div>
			</div>

			<div class="rounded-lg p-4" style="background: var(--bg-secondary); border: 1px solid var(--border);">
				<div class="text-[11px] font-medium uppercase tracking-wider" style="color: var(--text-tertiary);">Running</div>
				<div class="text-2xl font-semibold mt-2 font-[JetBrains_Mono,monospace]" style="color: var(--text-primary);">{dashboard.runningJobs}</div>
				<div class="text-[12px] mt-1" style="color: var(--text-secondary);">{dashboard.queuedJobs} queued</div>
			</div>

			<div class="rounded-lg p-4" style="background: var(--bg-secondary); border: 1px solid var(--border);">
				<div class="text-[11px] font-medium uppercase tracking-wider" style="color: var(--text-tertiary);">Avg Utilization</div>
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

		<div>
			<h2 class="text-[12px] font-medium uppercase tracking-wider mb-3" style="color: var(--text-tertiary);">GPUs</h2>
			<div class="space-y-2">
				{#each devices as gpu}
					<div class="rounded-lg p-4" style="background: var(--bg-secondary); border: 1px solid var(--border);">
						<div class="flex items-center justify-between">
							<div class="flex items-center gap-3">
								<div
									class="w-1.5 h-1.5 rounded-full"
									style="background: {gpu.allocated ? 'var(--text-tertiary)' : 'var(--text-primary)'};"
								></div>
								<span class="text-[13px] font-medium" style="color: var(--text-primary);">{gpu.name}</span>
							</div>
							<span class="text-[11px] font-[JetBrains_Mono,monospace]" style="color: var(--text-tertiary);">gpu:{gpu.id}</span>
						</div>

						<div class="mt-3 grid grid-cols-3 gap-6 text-[12px] pl-[18px]">
							<div>
								<span style="color: var(--text-tertiary);">VRAM</span>
								<span class="ml-2 font-[JetBrains_Mono,monospace] text-[11px]" style="color: var(--text-secondary);">{(gpu.vramUsedMB / 1024).toFixed(1)} / {(gpu.vramTotalMB / 1024).toFixed(1)} GB</span>
							</div>
							<div>
								<span style="color: var(--text-tertiary);">Util</span>
								<span class="ml-2 font-[JetBrains_Mono,monospace] text-[11px]" style="color: var(--text-secondary);">{gpu.utilizationPct.toFixed(0)}%</span>
							</div>
							<div>
								<span style="color: var(--text-tertiary);">Temp</span>
								<span class="ml-2 font-[JetBrains_Mono,monospace] text-[11px]" style="color: var(--text-secondary);">{gpu.temperatureC}°C</span>
							</div>
						</div>

						<div class="mt-2 ml-[18px] h-[3px] rounded-full overflow-hidden" style="background: var(--bar-bg);">
							<div
								class="h-full rounded-full transition-all duration-500"
								style="width: {vramPct(gpu.vramTotalMB, gpu.vramUsedMB)}%; background: var(--bar-fill);"
							></div>
						</div>
					</div>
				{/each}
			</div>
		</div>
	{:else}
		<div class="flex items-center justify-center h-64" style="color: var(--text-muted);">Loading...</div>
	{/if}
</div>
