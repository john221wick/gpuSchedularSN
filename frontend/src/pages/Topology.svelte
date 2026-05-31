<script>
	import { onMount } from 'svelte';
	import { GetTopology, GetClusterTopology } from '../lib/api.js';

	let { remoteMode = false } = $props();
	let topo = $state(null);
	let clusterTopo = $state(null);

	async function refresh() {
		try {
			if (remoteMode) {
				clusterTopo = await GetClusterTopology();
				topo = null;
			} else {
				topo = await GetTopology();
				clusterTopo = null;
			}
		} catch (e) {
			console.error('Topology refresh failed:', e);
		}
	}

	onMount(() => { refresh(); });

	function bwColor(val) {
		if (val >= 400) return 'bg-emerald-500/20 text-emerald-500';
		if (val >= 100) return 'bg-emerald-500/10 text-emerald-500';
		if (val > 0) return 'bg-amber-500/10 text-amber-500';
		return '';
	}

	function linkTypeColor(type) {
		switch (type) {
			case 'NVLink': return 'bg-emerald-500/10 text-emerald-500';
			case 'XGMI': return 'bg-blue-500/10 text-blue-500';
			case 'XeLink': return 'bg-purple-500/10 text-purple-500';
			case 'PCIe': return 'bg-amber-500/10 text-amber-500';
			default: return '';
		}
	}
</script>

<div class="p-8 space-y-6 max-w-[1200px]">
	<div>
		<h1 class="text-lg font-semibold" style="color: var(--text-primary);">Topology</h1>
		<p class="text-[13px] mt-0.5" style="color: var(--text-tertiary);">
			{remoteMode ? 'Per-node GPU interconnect topology' : 'GPU interconnect bandwidth matrix'}
		</p>
	</div>

	{#if remoteMode}
		<!-- Cluster: per-node topology -->
		{#if clusterTopo && clusterTopo.nodes && clusterTopo.nodes.length > 0}
			{#each clusterTopo.nodes as node (node.nodeID)}
				<div class="space-y-4">
					<div class="flex items-center gap-2">
						<h2 class="text-[14px] font-semibold" style="color: var(--text-primary);">{node.nodeName}</h2>
						<span class="text-[12px]" style="color: var(--text-secondary);">{node.numGPUs} GPUs</span>
					</div>

					{#if node.numGPUs < 2}
						<div class="rounded-lg p-6 text-center" style="background: var(--bg-secondary); border: 1px solid var(--border);">
							<p class="text-[13px]" style="color: var(--text-secondary);">Single GPU — no topology to display</p>
						</div>
					{:else}
						<div class="rounded-lg p-5 overflow-x-auto" style="background: var(--bg-secondary); border: 1px solid var(--border);">
							<h3 class="text-[12px] font-medium uppercase tracking-wider mb-4" style="color: var(--text-tertiary);">Bandwidth Matrix (GB/s)</h3>
							<table class="w-auto">
								<thead>
									<tr>
										<th class="w-16 p-2"></th>
										{#each Array(node.numGPUs) as _, j}
											<th class="p-2 text-center text-[11px] font-[JetBrains_Mono,monospace] min-w-[64px]" style="color: var(--text-tertiary);">GPU {j}</th>
										{/each}
									</tr>
								</thead>
								<tbody>
									{#each Array(node.numGPUs) as _, i}
										<tr>
											<td class="p-2 text-[11px] font-[JetBrains_Mono,monospace]" style="color: var(--text-tertiary);">GPU {i}</td>
											{#each Array(node.numGPUs) as _, j}
												<td class="p-1">
													{#if i === j}
														<div class="text-center text-[11px] py-1.5 px-2 rounded" style="color: var(--text-muted);">—</div>
													{:else}
														<div class="text-center text-[11px] py-1.5 px-2 rounded font-[JetBrains_Mono,monospace] font-medium {bwColor(node.bandwidth[i][j])}" style="{bwColor(node.bandwidth[i][j]) === '' ? 'color: var(--text-muted)' : ''}">
															{node.bandwidth[i][j] > 0 ? node.bandwidth[i][j].toFixed(0) : '—'}
														</div>
													{/if}
												</td>
											{/each}
										</tr>
									{/each}
								</tbody>
							</table>
						</div>

						{#if node.links && node.links.length > 0}
							<div class="rounded-lg p-5" style="background: var(--bg-secondary); border: 1px solid var(--border);">
								<h3 class="text-[12px] font-medium uppercase tracking-wider mb-4" style="color: var(--text-tertiary);">Links</h3>
								<div class="space-y-1">
									{#each node.links as link}
										<div class="flex items-center gap-3 py-2 px-3 rounded-md transition-colors" role="listitem"
											onmouseenter={(e) => e.currentTarget.style.background = 'var(--hover-bg)'}
											onmouseleave={(e) => e.currentTarget.style.background = 'none'}>
											<span class="text-[13px] font-[JetBrains_Mono,monospace]" style="color: var(--text-primary);">GPU {link.gpuA}</span>
											<span style="color: var(--text-muted);">→</span>
											<span class="text-[13px] font-[JetBrains_Mono,monospace]" style="color: var(--text-primary);">GPU {link.gpuB}</span>
											<span class="text-[11px] px-2 py-0.5 rounded font-medium {linkTypeColor(link.type)}" style="{linkTypeColor(link.type) === '' ? 'background: var(--bg-tertiary); color: var(--text-tertiary)' : ''}">
												{link.type}
											</span>
											<span class="text-[12px] ml-auto font-[JetBrains_Mono,monospace]" style="color: var(--text-secondary);">{link.bandwidthGBps.toFixed(0)} GB/s</span>
										</div>
									{/each}
								</div>
							</div>
						{/if}
					{/if}
				</div>
			{/each}
		{:else}
			<div class="flex items-center justify-center h-64" style="color: var(--text-muted);">
				{clusterTopo ? 'No nodes connected' : 'Loading...'}
			</div>
		{/if}

	{:else}
		<!-- Local: single topology -->
		{#if topo}
			{#if topo.numGPUs < 2}
				<div class="rounded-lg p-8 text-center" style="background: var(--bg-secondary); border: 1px solid var(--border);">
					<p class="text-[13px]" style="color: var(--text-secondary);">Single GPU detected — no topology to display</p>
				</div>
			{:else}
				<div class="rounded-lg p-5 overflow-x-auto" style="background: var(--bg-secondary); border: 1px solid var(--border);">
					<h2 class="text-[12px] font-medium uppercase tracking-wider mb-4" style="color: var(--text-tertiary);">Bandwidth Matrix (GB/s)</h2>
					<table class="w-auto">
						<thead>
							<tr>
								<th class="w-16 p-2"></th>
								{#each Array(topo.numGPUs) as _, j}
									<th class="p-2 text-center text-[11px] font-[JetBrains_Mono,monospace] min-w-[64px]" style="color: var(--text-tertiary);">GPU {j}</th>
								{/each}
							</tr>
						</thead>
						<tbody>
							{#each Array(topo.numGPUs) as _, i}
								<tr>
									<td class="p-2 text-[11px] font-[JetBrains_Mono,monospace]" style="color: var(--text-tertiary);">GPU {i}</td>
									{#each Array(topo.numGPUs) as _, j}
										<td class="p-1">
											{#if i === j}
												<div class="text-center text-[11px] py-1.5 px-2 rounded" style="color: var(--text-muted);">—</div>
											{:else}
												<div class="text-center text-[11px] py-1.5 px-2 rounded font-[JetBrains_Mono,monospace] font-medium {bwColor(topo.bandwidth[i][j])}" style="{bwColor(topo.bandwidth[i][j]) === '' ? 'color: var(--text-muted)' : ''}">
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

				<div class="rounded-lg p-5" style="background: var(--bg-secondary); border: 1px solid var(--border);">
					<h2 class="text-[12px] font-medium uppercase tracking-wider mb-4" style="color: var(--text-tertiary);">Links</h2>
					<div class="space-y-1">
						{#each topo.links as link}
							<div class="flex items-center gap-3 py-2 px-3 rounded-md transition-colors" role="listitem"
								onmouseenter={(e) => e.currentTarget.style.background = 'var(--hover-bg)'}
								onmouseleave={(e) => e.currentTarget.style.background = 'none'}>
								<span class="text-[13px] font-[JetBrains_Mono,monospace]" style="color: var(--text-primary);">GPU {link.gpuA}</span>
								<span style="color: var(--text-muted);">→</span>
								<span class="text-[13px] font-[JetBrains_Mono,monospace]" style="color: var(--text-primary);">GPU {link.gpuB}</span>
								<span class="text-[11px] px-2 py-0.5 rounded font-medium {linkTypeColor(link.type)}" style="{linkTypeColor(link.type) === '' ? 'background: var(--bg-tertiary); color: var(--text-tertiary)' : ''}">
									{link.type}
								</span>
								<span class="text-[12px] ml-auto font-[JetBrains_Mono,monospace]" style="color: var(--text-secondary);">{link.bandwidthGBps.toFixed(0)} GB/s</span>
							</div>
						{/each}
					</div>
				</div>
			{/if}
		{:else}
			<div class="flex items-center justify-center h-64" style="color: var(--text-muted);">Loading...</div>
		{/if}
	{/if}
</div>
