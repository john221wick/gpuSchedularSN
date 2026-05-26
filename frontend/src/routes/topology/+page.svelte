<script>
	import { onMount } from 'svelte';
	import { GetTopology } from '$lib/api.js';

	let topo = $state(null);

	async function refresh() {
		try {
			topo = await GetTopology();
		} catch (e) {
			console.error('Topology refresh failed:', e);
		}
	}

	onMount(() => { refresh(); });

	function bwColor(val) {
		if (val >= 400) return 'bg-emerald-500/30 text-emerald-300';
		if (val >= 100) return 'bg-emerald-500/15 text-emerald-400';
		if (val > 0) return 'bg-amber-500/15 text-amber-400';
		return 'bg-transparent text-[#3a3a5c]';
	}

	function linkTypeColor(type) {
		switch (type) {
			case 'NVLink': return 'bg-emerald-500/15 text-emerald-400 border-emerald-500/30';
			case 'XGMI': return 'bg-blue-500/15 text-blue-400 border-blue-500/30';
			case 'XeLink': return 'bg-purple-500/15 text-purple-400 border-purple-500/30';
			case 'PCIe': return 'bg-amber-500/15 text-amber-400 border-amber-500/30';
			default: return 'bg-[#2a2a4a] text-[#8888aa] border-[#3a3a5c]';
		}
	}
</script>

<div class="p-6 space-y-6">
	<div>
		<h1 class="text-xl font-semibold text-white">Topology</h1>
		<p class="text-sm text-[#8888aa] mt-1">GPU interconnect bandwidth matrix</p>
	</div>

	{#if topo}
		{#if topo.numGPUs < 2}
			<div class="bg-[#16162a] rounded-xl border border-[#2a2a4a] p-8 text-center">
				<p class="text-[#8888aa]">Single GPU detected — no topology to display</p>
			</div>
		{:else}
			<!-- Bandwidth matrix -->
			<div class="bg-[#16162a] rounded-xl border border-[#2a2a4a] p-5 overflow-x-auto">
				<h2 class="text-sm font-semibold text-[#8888aa] uppercase tracking-wider mb-4">Bandwidth Matrix (GB/s)</h2>
				<table class="w-auto">
					<thead>
						<tr>
							<th class="w-16 p-2"></th>
							{#each Array(topo.numGPUs) as _, j}
								<th class="p-2 text-center text-xs font-mono text-[#8888aa] min-w-[64px]">GPU {j}</th>
							{/each}
						</tr>
					</thead>
					<tbody>
						{#each Array(topo.numGPUs) as _, i}
							<tr>
								<td class="p-2 text-xs font-mono text-[#8888aa]">GPU {i}</td>
								{#each Array(topo.numGPUs) as _, j}
									<td class="p-1">
										{#if i === j}
											<div class="text-center text-xs py-1.5 px-2 rounded text-[#3a3a5c]">—</div>
										{:else}
											<div class="text-center text-xs py-1.5 px-2 rounded font-mono font-medium {bwColor(topo.bandwidth[i][j])}">
												{topo.bandwidth[i][j] > 0 ? topo.bandwidth[i][j].toFixed(0) : '—'}
											</div>
										{/if}
									</td>
								{/each}
							</tr>
						{/each}
					</tbody>
				</table>
			</div>

			<!-- Links list -->
			<div class="bg-[#16162a] rounded-xl border border-[#2a2a4a] p-5">
				<h2 class="text-sm font-semibold text-[#8888aa] uppercase tracking-wider mb-4">Links</h2>
				<div class="space-y-2">
					{#each topo.links as link}
						<div class="flex items-center gap-3 py-2 px-3 rounded-lg bg-[#1a1a2e]">
							<span class="text-sm font-mono text-white">GPU {link.gpuA}</span>
							<svg class="w-4 h-4 text-[#6666aa]" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
								<path stroke-linecap="round" stroke-linejoin="round" d="M8 7h12m0 0l-4-4m4 4l-4 4m0 6H4m0 0l4 4m-4-4l4-4" />
							</svg>
							<span class="text-sm font-mono text-white">GPU {link.gpuB}</span>
							<span class="text-[11px] px-2 py-0.5 rounded-md border font-medium {linkTypeColor(link.type)}">
								{link.type}
							</span>
							<span class="text-sm text-[#8888aa] ml-auto font-mono">{link.bandwidthGBps.toFixed(0)} GB/s</span>
						</div>
					{/each}
				</div>
			</div>
		{/if}
	{:else}
		<div class="flex items-center justify-center h-64 text-[#6666aa]">Loading...</div>
	{/if}
</div>
