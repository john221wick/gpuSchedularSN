<script>
	import { onMount } from 'svelte';
	import { GetDashboard, GetDevices } from '$lib/api.js';

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

<div class="p-6 space-y-6">
	<!-- Header -->
	<div>
		<h1 class="text-xl font-semibold text-white">Dashboard</h1>
		<p class="text-sm text-[#8888aa] mt-1">System overview</p>
	</div>

	{#if dashboard}
		<!-- Stats cards row -->
		<div class="grid grid-cols-4 gap-4">
			<!-- Total GPUs -->
			<div class="bg-[#16162a] rounded-xl border border-[#2a2a4a] p-4">
				<div class="text-[11px] font-medium uppercase tracking-wider text-[#6666aa]">Total GPUs</div>
				<div class="text-3xl font-bold text-white mt-1">{dashboard.totalGPUs}</div>
				<div class="text-xs text-[#8888aa] mt-1">{dashboard.freeGPUs} available</div>
			</div>

			<!-- Running Jobs -->
			<div class="bg-[#16162a] rounded-xl border border-[#2a2a4a] p-4">
				<div class="text-[11px] font-medium uppercase tracking-wider text-[#6666aa]">Running</div>
				<div class="text-3xl font-bold text-white mt-1">{dashboard.runningJobs}</div>
				<div class="text-xs text-[#8888aa] mt-1">{dashboard.queuedJobs} queued</div>
			</div>

			<!-- Avg Utilization -->
			<div class="bg-[#16162a] rounded-xl border border-[#2a2a4a] p-4">
				<div class="text-[11px] font-medium uppercase tracking-wider text-[#6666aa]">Avg Utilization</div>
				<div class="text-3xl font-bold text-white mt-1">{dashboard.avgUtil.toFixed(0)}%</div>
				<div class="mt-2 h-1.5 bg-[#2a2a4a] rounded-full overflow-hidden">
					<div
						class="h-full rounded-full transition-all duration-500"
						class:bg-emerald-500={dashboard.avgUtil < 50}
						class:bg-amber-500={dashboard.avgUtil >= 50 && dashboard.avgUtil < 80}
						class:bg-red-500={dashboard.avgUtil >= 80}
						style="width: {dashboard.avgUtil}%"
					></div>
				</div>
			</div>

			<!-- VRAM -->
			<div class="bg-[#16162a] rounded-xl border border-[#2a2a4a] p-4">
				<div class="text-[11px] font-medium uppercase tracking-wider text-[#6666aa]">VRAM</div>
				<div class="text-3xl font-bold text-white mt-1">
					{vramPct(dashboard.totalVRAMMB, dashboard.usedVRAMMB)}%
				</div>
				<div class="text-xs text-[#8888aa] mt-1">
					{(dashboard.usedVRAMMB / 1024).toFixed(1)} / {(dashboard.totalVRAMMB / 1024).toFixed(1)} GB
				</div>
			</div>
		</div>

		<!-- GPU cards -->
		<div>
			<h2 class="text-sm font-semibold text-[#8888aa] uppercase tracking-wider mb-3">GPUs</h2>
			<div class="grid grid-cols-2 gap-3">
				{#each devices as gpu}
					<div class="bg-[#16162a] rounded-xl border border-[#2a2a4a] p-4 flex items-start gap-4">
						<!-- Status dot -->
						<div class="mt-1">
							<div
								class="w-2.5 h-2.5 rounded-full"
								class:bg-emerald-500={!gpu.allocated}
								class:bg-amber-500={gpu.allocated}
							></div>
						</div>

						<div class="flex-1 min-w-0">
							<div class="flex items-center justify-between">
								<span class="text-sm font-medium text-white truncate">{gpu.name}</span>
								<span class="text-[11px] px-1.5 py-0.5 rounded bg-[#2a2a4a] text-[#8888aa]">GPU {gpu.id}</span>
							</div>

							<div class="mt-2 grid grid-cols-3 gap-3 text-xs">
								<div>
									<span class="text-[#6666aa]">VRAM</span>
									<div class="text-white mt-0.5">{(gpu.vramUsedMB / 1024).toFixed(1)} / {(gpu.vramTotalMB / 1024).toFixed(1)} GB</div>
								</div>
								<div>
									<span class="text-[#6666aa]">Util</span>
									<div class="text-white mt-0.5">{gpu.utilizationPct.toFixed(0)}%</div>
								</div>
								<div>
									<span class="text-[#6666aa]">Temp</span>
									<div class="text-white mt-0.5">{gpu.temperatureC}°C</div>
								</div>
							</div>

							<!-- VRAM bar -->
							<div class="mt-2 h-1 bg-[#2a2a4a] rounded-full overflow-hidden">
								<div
									class="h-full bg-[#4f6ef7] rounded-full transition-all duration-500"
									style="width: {vramPct(gpu.vramTotalMB, gpu.vramUsedMB)}%"
								></div>
							</div>
						</div>
					</div>
				{/each}
			</div>
		</div>
	{:else}
		<div class="flex items-center justify-center h-64 text-[#6666aa]">Loading...</div>
	{/if}
</div>
