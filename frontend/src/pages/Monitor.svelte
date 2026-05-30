<script>
	import { onMount } from 'svelte';
	import { GetClusterMonitor } from '../lib/api.js';

	let { remoteMode = false } = $props();

	let nodes = $state([]);
	let loading = $state(false);
	let auto = $state(false);
	let lastUpdated = $state('');
	let interval;

	async function refresh() {
		loading = true;
		try {
			nodes = (await GetClusterMonitor()) || [];
			lastUpdated = new Date().toLocaleTimeString();
		} catch (e) {
			console.error('Monitor refresh failed:', e);
		} finally {
			loading = false;
		}
	}

	onMount(() => {
		refresh();
		return () => clearInterval(interval);
	});

	// Auto-poll toggle (off by default; docker stats add latency).
	$effect(() => {
		clearInterval(interval);
		if (auto) interval = setInterval(refresh, 5000);
	});

	function clamp(n) {
		return Math.max(0, Math.min(100, n || 0));
	}
	function memPct(used, total) {
		return total ? clamp((used / total) * 100) : 0;
	}
	function fmtGB(mb) {
		return ((mb || 0) / 1024).toFixed(1);
	}
	function fmtUptime(s) {
		s = Math.floor(s || 0);
		const d = Math.floor(s / 86400);
		const h = Math.floor((s % 86400) / 3600);
		const m = Math.floor((s % 3600) / 60);
		if (d > 0) return `${d}d ${h}h`;
		if (h > 0) return `${h}h ${m}m`;
		return `${m}m`;
	}
</script>

<div class="p-8 space-y-5 max-w-[1100px]">
	<!-- Header -->
	<div class="flex items-start justify-between gap-4">
		<div>
			<h1 class="text-lg font-semibold" style="color: var(--text-primary);">Monitor</h1>
			<p class="text-[13px] mt-0.5" style="color: var(--text-tertiary);">
				{nodes.length} node{nodes.length !== 1 ? 's' : ''}{lastUpdated ? ` · updated ${lastUpdated}` : ''}
			</p>
		</div>
		<div class="flex items-center gap-2 shrink-0">
			<!-- Auto toggle -->
			<button
				onclick={() => (auto = !auto)}
				class="flex items-center gap-2 px-2.5 h-8 rounded-lg text-[12.5px] font-medium cursor-pointer transition-colors"
				style="background: var(--bg-secondary); border: 1px solid var(--border); color: {auto ? 'var(--text-primary)' : 'var(--text-tertiary)'};"
				title="Auto-refresh every 5s"
			>
				<span class="relative inline-block w-7 h-4 rounded-full transition-colors"
					style="background: {auto ? 'var(--accent)' : 'var(--bar-bg)'};">
					<span class="absolute top-0.5 w-3 h-3 rounded-full transition-all"
						style="left: {auto ? '14px' : '2px'}; background: #fff;"></span>
				</span>
				Auto
			</button>
			<!-- Refresh -->
			<button
				onclick={refresh}
				disabled={loading}
				class="flex items-center gap-2 px-3 h-8 rounded-lg text-[12.5px] font-medium cursor-pointer transition-opacity disabled:opacity-60"
				style="background: var(--accent); color: var(--accent-text);"
			>
				<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor"
					stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"
					class={loading ? 'animate-spin' : ''}>
					<path d="M21 12a9 9 0 1 1-2.64-6.36" /><polyline points="21 3 21 9 15 9" />
				</svg>
				{loading ? 'Refreshing' : 'Refresh'}
			</button>
		</div>
	</div>

	{#if !remoteMode}
		<div class="flex items-center justify-center h-40 text-[13px]" style="color: var(--text-muted);">
			Monitor is available in Remote mode.
		</div>
	{:else if nodes.length === 0}
		<div class="flex items-center justify-center h-40 text-[13px]" style="color: var(--text-muted);">
			No connected nodes. Connect a node to monitor it.
		</div>
	{:else}
		{#each nodes as node (node.nodeID)}
			<div class="rounded-xl overflow-hidden" style="background: var(--bg-secondary); border: 1px solid var(--border);">
				<!-- Node header -->
				<div class="flex items-center gap-2.5 px-4 py-3" style="border-bottom: 1px solid var(--border);">
					<div class="w-2 h-2 rounded-full" style="background: {node.reachable ? '#3fb950' : '#f85149'};"></div>
					<span class="text-[13.5px] font-semibold" style="color: var(--text-primary);">{node.nodeName || node.nodeID}</span>
					{#if node.host?.hostname}
						<span class="text-[12px] font-[JetBrains_Mono,monospace]" style="color: var(--text-muted);">{node.host.hostname}</span>
					{/if}
					<div class="flex-1"></div>
					{#if node.reachable && node.host?.cpuCores}
						<span class="text-[11px]" style="color: var(--text-tertiary);">{node.host.cpuCores} cores · up {fmtUptime(node.host.uptimeSeconds)}</span>
					{/if}
				</div>

				{#if !node.reachable}
					<div class="px-4 py-4 text-[12.5px]" style="color: #f85149;">{node.error || 'Node unreachable'}</div>
				{:else}
					<!-- Host stats -->
					<div class="grid grid-cols-3 gap-px" style="background: var(--border);">
						<!-- CPU -->
						<div class="p-4" style="background: var(--bg-secondary);">
							<div class="text-[10.5px] font-semibold uppercase tracking-wider" style="color: var(--text-muted);">CPU</div>
							<div class="text-[20px] font-semibold mt-1 font-[JetBrains_Mono,monospace]" style="color: var(--text-primary);">{(node.host.cpuPercent || 0).toFixed(0)}<span class="text-[13px]" style="color: var(--text-tertiary);">%</span></div>
							<div class="w-full h-1 rounded-full overflow-hidden mt-2" style="background: var(--bar-bg);">
								<div class="h-full rounded-full transition-all duration-500" style="width: {clamp(node.host.cpuPercent)}%; background: var(--bar-fill);"></div>
							</div>
							<div class="text-[11px] mt-1.5" style="color: var(--text-tertiary);">load {(node.host.loadAvg?.[0] || 0).toFixed(2)}</div>
						</div>
						<!-- Memory -->
						<div class="p-4" style="background: var(--bg-secondary);">
							<div class="text-[10.5px] font-semibold uppercase tracking-wider" style="color: var(--text-muted);">Memory</div>
							<div class="text-[20px] font-semibold mt-1 font-[JetBrains_Mono,monospace]" style="color: var(--text-primary);">{memPct(node.host.memUsedMB, node.host.memTotalMB).toFixed(0)}<span class="text-[13px]" style="color: var(--text-tertiary);">%</span></div>
							<div class="w-full h-1 rounded-full overflow-hidden mt-2" style="background: var(--bar-bg);">
								<div class="h-full rounded-full transition-all duration-500" style="width: {memPct(node.host.memUsedMB, node.host.memTotalMB)}%; background: var(--bar-fill);"></div>
							</div>
							<div class="text-[11px] mt-1.5 font-[JetBrains_Mono,monospace]" style="color: var(--text-tertiary);">{fmtGB(node.host.memUsedMB)} / {fmtGB(node.host.memTotalMB)} GB</div>
						</div>
						<!-- GPUs summary -->
						<div class="p-4" style="background: var(--bg-secondary);">
							<div class="text-[10.5px] font-semibold uppercase tracking-wider" style="color: var(--text-muted);">GPUs</div>
							<div class="text-[20px] font-semibold mt-1 font-[JetBrains_Mono,monospace]" style="color: var(--text-primary);">{node.gpus?.length || 0}</div>
							<div class="text-[11px] mt-2.5" style="color: var(--text-tertiary);">
								{node.containers?.available ? `${node.containers.containers?.length || 0} container${(node.containers.containers?.length || 0) !== 1 ? 's' : ''}` : 'docker n/a'}
							</div>
						</div>
					</div>

					<!-- GPU rows -->
					{#if node.gpus?.length}
						<div class="px-4 pt-3 pb-1 text-[10.5px] font-semibold uppercase tracking-wider" style="color: var(--text-muted); border-top: 1px solid var(--border);">GPU Utilization</div>
						<div class="px-4 pb-3 space-y-2">
							{#each node.gpus as gpu}
								<div class="flex items-center gap-3">
									<span class="text-[12.5px] font-medium w-44 truncate" style="color: var(--text-primary);">{gpu.name}<span class="font-[JetBrains_Mono,monospace]" style="color: var(--text-muted);"> :{gpu.id}</span></span>
									<div class="flex-1 h-1.5 rounded-full overflow-hidden" style="background: var(--bar-bg);">
										<div class="h-full rounded-full transition-all duration-500" style="width: {clamp(gpu.utilizationPct)}%; background: var(--bar-fill);"></div>
									</div>
									<span class="text-[12px] font-[JetBrains_Mono,monospace] w-10 text-right" style="color: var(--text-secondary);">{(gpu.utilizationPct || 0).toFixed(0)}%</span>
									<span class="text-[12px] font-[JetBrains_Mono,monospace] w-24 text-right" style="color: var(--text-tertiary);">{fmtGB(gpu.vramUsedMB)}/{fmtGB(gpu.vramTotalMB)}G</span>
									<span class="text-[12px] font-[JetBrains_Mono,monospace] w-12 text-right" style="color: var(--text-tertiary);">{gpu.temperatureC}°C</span>
								</div>
							{/each}
						</div>
					{/if}

					<!-- Containers -->
					<div class="px-4 pt-3 pb-1 text-[10.5px] font-semibold uppercase tracking-wider" style="color: var(--text-muted); border-top: 1px solid var(--border);">Containers</div>
					{#if !node.containers?.available}
						<div class="px-4 pb-4 text-[12.5px]" style="color: var(--text-muted);">
							Docker not available on this node{node.containers?.error ? ` (${node.containers.error})` : ''}.
						</div>
					{:else if node.containers.error}
						<div class="px-4 pb-4 text-[12.5px]" style="color: var(--text-muted);">Docker error: {node.containers.error} (is the daemon running?)</div>
					{:else if !node.containers.containers?.length}
						<div class="px-4 pb-4 text-[12.5px]" style="color: var(--text-muted);">No running containers.</div>
					{:else}
						<div class="px-4 pb-4 overflow-x-auto">
							<table class="w-full">
								<thead>
									<tr>
										{#each ['Name', 'Image', 'Status', 'CPU', 'Memory'] as h}
											<th class="text-left py-1.5 pr-4 text-[10.5px] font-medium uppercase tracking-wider" style="color: var(--text-tertiary);">{h}</th>
										{/each}
									</tr>
								</thead>
								<tbody>
									{#each node.containers.containers as c (c.id)}
										<tr style="border-top: 1px solid var(--border);">
											<td class="py-2 pr-4 text-[12.5px] font-medium" style="color: var(--text-primary);">{c.name}</td>
											<td class="py-2 pr-4 text-[12px] font-[JetBrains_Mono,monospace]" style="color: var(--text-secondary);">{c.image}</td>
											<td class="py-2 pr-4 text-[12px]" style="color: var(--text-tertiary);">{c.status}</td>
											<td class="py-2 pr-4 text-[12px] font-[JetBrains_Mono,monospace]" style="color: var(--text-secondary);">{(c.cpuPercent || 0).toFixed(1)}%</td>
											<td class="py-2 pr-4 text-[12px] font-[JetBrains_Mono,monospace]" style="color: var(--text-secondary);">{fmtGB(c.memUsedMB)}/{fmtGB(c.memLimitMB)}G</td>
										</tr>
									{/each}
								</tbody>
							</table>
						</div>
					{/if}
				{/if}
			</div>
		{/each}
	{/if}
</div>
