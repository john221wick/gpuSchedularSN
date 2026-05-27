<script>
	import { onMount } from 'svelte';
	import { GetDevices } from '../lib/api.js';

	let devices = $state([]);
	let interval;

	async function refresh() {
		try {
			devices = await GetDevices();
		} catch (e) {
			console.error('Devices refresh failed:', e);
		}
	}

	onMount(() => {
		refresh();
		interval = setInterval(refresh, 2000);
		return () => clearInterval(interval);
	});

	function vramPct(d) {
		if (!d.vramTotalMB) return 0;
		return Math.round((d.vramUsedMB / d.vramTotalMB) * 100);
	}

	function utilColor(pct) {
		if (pct < 70) return '';
		if (pct < 90) return 'text-amber-500';
		return 'text-red-500';
	}

	function tempColor(c) {
		if (c < 75) return '';
		if (c < 90) return 'text-amber-500';
		return 'text-red-500';
	}
</script>

<div class="p-8 space-y-6 max-w-[1200px]">
	<div>
		<h1 class="text-lg font-semibold" style="color: var(--text-primary);">GPUs</h1>
		<p class="text-[13px] mt-0.5" style="color: var(--text-tertiary);">{devices.length} device{devices.length !== 1 ? 's' : ''} detected</p>
	</div>

	<div class="space-y-2">
		{#each devices as gpu}
			<div class="rounded-lg p-5" style="background: var(--bg-secondary); border: 1px solid var(--border);">
				<div class="flex items-center justify-between mb-4">
					<div class="flex items-center gap-3">
						<div
							class="w-1.5 h-1.5 rounded-full"
							style="background: {gpu.allocated ? 'var(--text-tertiary)' : 'var(--text-primary)'};"
						></div>
						<h3 class="text-[14px] font-medium" style="color: var(--text-primary);">{gpu.name}</h3>
					</div>
					<div class="flex items-center gap-2">
						<span class="text-[11px] px-2 py-0.5 rounded font-medium"
							style="background: var(--bg-tertiary); color: {gpu.allocated ? 'var(--text-tertiary)' : 'var(--text-primary)'};">
							{gpu.allocated ? 'In Use' : 'Available'}
						</span>
						<span class="text-[11px] px-2 py-0.5 rounded font-[JetBrains_Mono,monospace]" style="background: var(--bg-tertiary); color: var(--text-tertiary);">
							gpu:{gpu.id}
						</span>
						<span class="text-[11px] px-2 py-0.5 rounded" style="background: var(--bg-tertiary); color: var(--text-tertiary);">
							{gpu.vendor}
						</span>
					</div>
				</div>

				<div class="grid grid-cols-4 gap-4">
					<div class="space-y-2">
						<div class="flex items-baseline justify-between">
							<span class="text-[11px] font-medium uppercase tracking-wider" style="color: var(--text-tertiary);">VRAM</span>
							<span class="text-[11px] font-[JetBrains_Mono,monospace]" style="color: var(--text-secondary);">{vramPct(gpu)}%</span>
						</div>
						<div class="h-1 rounded-full overflow-hidden" style="background: var(--bar-bg);">
							<div
								class="h-full rounded-full transition-all duration-500"
								style="width: {vramPct(gpu)}%; background: var(--bar-fill);"
							></div>
						</div>
						<div class="text-[11px] font-[JetBrains_Mono,monospace]" style="color: var(--text-secondary);">
							{(gpu.vramUsedMB / 1024).toFixed(1)} / {(gpu.vramTotalMB / 1024).toFixed(1)} GB
						</div>
					</div>

					<div class="space-y-2">
						<span class="text-[11px] font-medium uppercase tracking-wider" style="color: var(--text-tertiary);">Utilization</span>
						<div class="text-xl font-semibold font-[JetBrains_Mono,monospace] {utilColor(gpu.utilizationPct)}" style="{utilColor(gpu.utilizationPct) === '' ? 'color: var(--text-primary)' : ''}">{gpu.utilizationPct.toFixed(0)}%</div>
					</div>

					<div class="space-y-2">
						<span class="text-[11px] font-medium uppercase tracking-wider" style="color: var(--text-tertiary);">Temperature</span>
						<div class="text-xl font-semibold font-[JetBrains_Mono,monospace] {tempColor(gpu.temperatureC)}" style="{tempColor(gpu.temperatureC) === '' ? 'color: var(--text-primary)' : ''}">{gpu.temperatureC}°C</div>
					</div>

					<div class="space-y-2">
						<span class="text-[11px] font-medium uppercase tracking-wider" style="color: var(--text-tertiary);">Vendor</span>
						<div class="text-[13px] font-medium" style="color: var(--text-primary);">{gpu.vendor}</div>
						<div class="text-[11px] font-[JetBrains_Mono,monospace]" style="color: var(--text-tertiary);">idx {gpu.id}</div>
					</div>
				</div>
			</div>
		{/each}
	</div>

	{#if devices.length === 0}
		<div class="flex items-center justify-center h-64" style="color: var(--text-muted);">No GPUs detected</div>
	{/if}
</div>
