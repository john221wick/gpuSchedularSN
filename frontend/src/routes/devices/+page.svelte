<script>
	import { onMount } from 'svelte';
	import { GetDevices } from '$lib/api.js';

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
		if (pct < 30) return 'text-emerald-400';
		if (pct < 70) return 'text-amber-400';
		return 'text-red-400';
	}

	function tempColor(c) {
		if (c < 50) return 'text-emerald-400';
		if (c < 75) return 'text-amber-400';
		return 'text-red-400';
	}
</script>

<div class="p-6 space-y-6">
	<div>
		<h1 class="text-xl font-semibold text-white">GPUs</h1>
		<p class="text-sm text-[#8888aa] mt-1">{devices.length} device{devices.length !== 1 ? 's' : ''} detected</p>
	</div>

	<div class="space-y-3">
		{#each devices as gpu}
			<div class="bg-[#16162a] rounded-xl border border-[#2a2a4a] p-5">
				<div class="flex items-center justify-between mb-4">
					<div class="flex items-center gap-3">
						<div
							class="w-3 h-3 rounded-full"
							class:bg-emerald-500={!gpu.allocated}
							class:bg-amber-500={gpu.allocated}
						></div>
						<h3 class="text-[15px] font-semibold text-white">{gpu.name}</h3>
					</div>
					<div class="flex items-center gap-2">
						<span class="text-[11px] px-2 py-0.5 rounded-md font-medium
							{gpu.allocated ? 'bg-amber-500/10 text-amber-400' : 'bg-emerald-500/10 text-emerald-400'}">
							{gpu.allocated ? 'In Use' : 'Available'}
						</span>
						<span class="text-[11px] px-2 py-0.5 rounded-md bg-[#2a2a4a] text-[#8888aa] font-mono">
							GPU:{gpu.id}
						</span>
						<span class="text-[11px] px-2 py-0.5 rounded-md bg-[#2a2a4a] text-[#8888aa]">
							{gpu.vendor}
						</span>
					</div>
				</div>

				<!-- Stats grid -->
				<div class="grid grid-cols-4 gap-4">
					<!-- VRAM -->
					<div class="space-y-2">
						<div class="flex items-baseline justify-between">
							<span class="text-[11px] font-medium uppercase tracking-wider text-[#6666aa]">VRAM</span>
							<span class="text-xs text-[#8888aa]">{vramPct(gpu)}%</span>
						</div>
						<div class="h-2 bg-[#2a2a4a] rounded-full overflow-hidden">
							<div
								class="h-full bg-[#4f6ef7] rounded-full transition-all duration-500"
								style="width: {vramPct(gpu)}%"
							></div>
						</div>
						<div class="text-xs text-[#8888aa]">
							{(gpu.vramUsedMB / 1024).toFixed(1)} / {(gpu.vramTotalMB / 1024).toFixed(1)} GB
						</div>
					</div>

					<!-- Utilization -->
					<div class="space-y-2">
						<span class="text-[11px] font-medium uppercase tracking-wider text-[#6666aa]">Utilization</span>
						<div class="text-2xl font-bold {utilColor(gpu.utilizationPct)}">{gpu.utilizationPct.toFixed(0)}%</div>
					</div>

					<!-- Temperature -->
					<div class="space-y-2">
						<span class="text-[11px] font-medium uppercase tracking-wider text-[#6666aa]">Temperature</span>
						<div class="text-2xl font-bold {tempColor(gpu.temperatureC)}">{gpu.temperatureC}°C</div>
					</div>

					<!-- Vendor -->
					<div class="space-y-2">
						<span class="text-[11px] font-medium uppercase tracking-wider text-[#6666aa]">Vendor</span>
						<div class="text-sm text-white font-medium">{gpu.vendor}</div>
						<div class="text-xs text-[#6666aa]">Index: {gpu.id}</div>
					</div>
				</div>
			</div>
		{/each}
	</div>

	{#if devices.length === 0}
		<div class="flex items-center justify-center h-64 text-[#6666aa]">No GPUs detected</div>
	{/if}
</div>
