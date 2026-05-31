<script>
	import { onMount } from 'svelte';
	import { GetDevices, GetClusterDevices } from '../lib/api.js';

	let { remoteMode = false } = $props();
	let devices = $state([]);
	let interval;

	async function refresh() {
		try {
			devices = remoteMode ? (await GetClusterDevices() || []) : await GetDevices();
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
</script>

<div class="p-8 space-y-4 max-w-[1000px]">
	<div>
		<h1 class="text-lg font-semibold" style="color: var(--text-primary);">GPUs</h1>
		<p class="text-[13px] mt-0.5" style="color: var(--text-tertiary);">{devices.length} device{devices.length !== 1 ? 's' : ''}{remoteMode ? ' across cluster' : ''}</p>
	</div>

	<div class="rounded-lg overflow-hidden" style="background: var(--bg-secondary); border: 1px solid var(--border);">
		<table class="w-full">
			<thead>
				<tr style="border-bottom: 1px solid var(--border);">
					<th class="text-left px-4 py-2.5 text-[11px] font-medium uppercase tracking-wider" style="color: var(--text-tertiary);">GPU</th>
					{#if remoteMode}
						<th class="text-left px-4 py-2.5 text-[11px] font-medium uppercase tracking-wider" style="color: var(--text-tertiary);">Node</th>
					{/if}
					<th class="text-left px-4 py-2.5 text-[11px] font-medium uppercase tracking-wider" style="color: var(--text-tertiary);">Status</th>
					<th class="text-left px-4 py-2.5 text-[11px] font-medium uppercase tracking-wider" style="color: var(--text-tertiary);">VRAM</th>
					<th class="text-left px-4 py-2.5 text-[11px] font-medium uppercase tracking-wider" style="color: var(--text-tertiary);">Util</th>
					<th class="text-left px-4 py-2.5 text-[11px] font-medium uppercase tracking-wider" style="color: var(--text-tertiary);">Temp</th>
				</tr>
			</thead>
			<tbody>
				{#each devices as gpu}
					<tr class="transition-colors" style="border-bottom: 1px solid var(--border);"
						onmouseenter={(e) => e.currentTarget.style.background = 'var(--hover-bg)'}
						onmouseleave={(e) => e.currentTarget.style.background = 'none'}>
						<td class="px-4 py-3">
							<div class="flex items-center gap-2">
								<div class="w-1.5 h-1.5 rounded-full" style="background: {gpu.allocated ? 'var(--text-tertiary)' : 'var(--text-primary)'};"></div>
								<span class="text-[13px] font-medium" style="color: var(--text-primary);">{gpu.name}</span>
								<span class="text-[11px] font-[JetBrains_Mono,monospace]" style="color: var(--text-muted);">:{gpu.id}</span>
							</div>
						</td>
						{#if remoteMode}
							<td class="px-4 py-3 text-[12px]" style="color: var(--text-secondary);">{gpu.nodeName || '—'}</td>
						{/if}
						<td class="px-4 py-3">
							<span class="text-[11px] px-2 py-0.5 rounded font-medium"
								style="background: var(--bg-tertiary); color: {gpu.allocated ? 'var(--text-tertiary)' : 'var(--text-primary)'};">
								{gpu.allocated ? 'In Use' : 'Free'}
							</span>
						</td>
						<td class="px-4 py-3">
							<div class="flex items-center gap-2">
								<div class="w-16 h-1 rounded-full overflow-hidden" style="background: var(--bar-bg);">
									<div class="h-full rounded-full transition-all duration-500" style="width: {vramPct(gpu)}%; background: var(--bar-fill);"></div>
								</div>
								<span class="text-[11px] font-[JetBrains_Mono,monospace]" style="color: var(--text-secondary);">{(gpu.vramUsedMB / 1024).toFixed(1)}/{(gpu.vramTotalMB / 1024).toFixed(1)}</span>
							</div>
						</td>
						<td class="px-4 py-3">
							<span class="text-[13px] font-[JetBrains_Mono,monospace]" style="color: var(--text-primary);">{gpu.utilizationPct.toFixed(0)}%</span>
						</td>
						<td class="px-4 py-3">
							<span class="text-[13px] font-[JetBrains_Mono,monospace]" style="color: var(--text-primary);">{gpu.temperatureC}°C</span>
						</td>
					</tr>
				{/each}
			</tbody>
		</table>
	</div>

	{#if devices.length === 0}
		<div class="flex items-center justify-center h-32" style="color: var(--text-muted);">No GPUs detected</div>
	{/if}
</div>
