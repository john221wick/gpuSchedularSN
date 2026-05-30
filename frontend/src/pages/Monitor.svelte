<script>
	import { onMount } from 'svelte';
	import { GetClusterMonitor } from '../lib/api.js';

	let { remoteMode = false } = $props();

	let nodes = $state([]);
	let loading = $state(false);
	let auto = $state(false);
	let lastUpdated = $state('');
	let interval;

	// Expandable detail panel: key = `${nodeID}:${cpu|memory|gpu}`. Sort desc by default.
	let expandedKey = $state('');
	let sortAsc = $state(false);

	function toggle(nodeID, which) {
		const key = nodeID + ':' + which;
		expandedKey = expandedKey === key ? '' : key;
	}
	function isOpen(nodeID, which) {
		return expandedKey === nodeID + ':' + which;
	}
	function sortedProcs(list, metric) {
		const arr = [...(list || [])];
		arr.sort((a, b) => {
			const av = metric === 'cpu' ? (a.cpuPercent || 0) : (a.memMB || 0);
			const bv = metric === 'cpu' ? (b.cpuPercent || 0) : (b.memMB || 0);
			return sortAsc ? av - bv : bv - av;
		});
		return arr.slice(0, 30);
	}
	function sortedGpuProcs(list) {
		const arr = [...(list || [])];
		arr.sort((a, b) => (sortAsc ? (a.memMB || 0) - (b.memMB || 0) : (b.memMB || 0) - (a.memMB || 0)));
		return arr;
	}
	function fmtMem(mb) {
		mb = mb || 0;
		return mb >= 1024 ? (mb / 1024).toFixed(1) + 'G' : Math.round(mb) + 'M';
	}

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

{#snippet procTable(list)}
	<div class="overflow-x-auto">
		<table class="w-full">
			<thead>
				<tr>
					<th class="text-left py-1.5 pr-4 text-[10.5px] font-medium uppercase tracking-wider" style="color: var(--text-tertiary);">PID</th>
					<th class="text-left py-1.5 pr-4 text-[10.5px] font-medium uppercase tracking-wider" style="color: var(--text-tertiary);">Command</th>
					<th class="text-right py-1.5 pr-4 text-[10.5px] font-medium uppercase tracking-wider" style="color: var(--text-tertiary);">CPU%</th>
					<th class="text-right py-1.5 text-[10.5px] font-medium uppercase tracking-wider" style="color: var(--text-tertiary);">Mem</th>
				</tr>
			</thead>
			<tbody>
				{#each list as p (p.pid)}
					<tr style="border-top: 1px solid var(--border);">
						<td class="py-1.5 pr-4 text-[12px] font-[JetBrains_Mono,monospace]" style="color: var(--text-tertiary);">{p.pid}</td>
						<td class="py-1.5 pr-4 text-[12.5px] truncate max-w-[280px]" style="color: var(--text-primary);">{p.command}</td>
						<td class="py-1.5 pr-4 text-[12px] font-[JetBrains_Mono,monospace] text-right" style="color: var(--text-secondary);">{(p.cpuPercent || 0).toFixed(1)}</td>
						<td class="py-1.5 text-[12px] font-[JetBrains_Mono,monospace] text-right" style="color: var(--text-secondary);">{fmtMem(p.memMB)}</td>
					</tr>
				{/each}
			</tbody>
		</table>
		{#if list.length === 0}
			<div class="py-3 text-[12px]" style="color: var(--text-muted);">No process data.</div>
		{/if}
	</div>
{/snippet}

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
					<!-- Machine info -->
					{#if node.host.osName || node.host.kernel || node.host.arch || node.host.cpuModel}
						<div class="flex flex-wrap items-center gap-x-5 gap-y-1 px-4 py-2.5 text-[11.5px]" style="border-bottom: 1px solid var(--border); color: var(--text-secondary);">
							{#if node.host.osName}<span><span style="color: var(--text-muted);">OS</span> {node.host.osName}</span>{/if}
							{#if node.host.kernel}<span><span style="color: var(--text-muted);">kernel</span> <span class="font-[JetBrains_Mono,monospace]">{node.host.kernel}</span></span>{/if}
							{#if node.host.arch}<span><span style="color: var(--text-muted);">arch</span> <span class="font-[JetBrains_Mono,monospace]">{node.host.arch}</span></span>{/if}
							{#if node.host.cpuModel}<span><span style="color: var(--text-muted);">CPU</span> {node.host.cpuModel}</span>{/if}
						</div>
					{/if}

					<!-- Host stat tiles (click to expand) -->
					<div class="grid grid-cols-3 gap-px" style="background: var(--border);">
						<!-- CPU -->
						<button type="button" onclick={() => toggle(node.nodeID, 'cpu')}
							class="p-4 text-left cursor-pointer transition-colors"
							style="background: {isOpen(node.nodeID, 'cpu') ? 'var(--hover-bg)' : 'var(--bg-secondary)'};"
							onmouseenter={(e) => { if (!isOpen(node.nodeID, 'cpu')) e.currentTarget.style.background = 'var(--hover-bg)'; }}
							onmouseleave={(e) => { if (!isOpen(node.nodeID, 'cpu')) e.currentTarget.style.background = 'var(--bg-secondary)'; }}>
							<div class="flex items-center justify-between">
								<span class="text-[10.5px] font-semibold uppercase tracking-wider" style="color: var(--text-muted);">CPU</span>
								<svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" style="color: var(--text-muted); transition: transform .2s; transform: rotate({isOpen(node.nodeID, 'cpu') ? 180 : 0}deg);"><polyline points="6 9 12 15 18 9"/></svg>
							</div>
							<div class="text-[20px] font-semibold mt-1 font-[JetBrains_Mono,monospace]" style="color: var(--text-primary);">{(node.host.cpuPercent || 0).toFixed(0)}<span class="text-[13px]" style="color: var(--text-tertiary);">%</span></div>
							<div class="w-full h-1 rounded-full overflow-hidden mt-2" style="background: var(--bar-bg);">
								<div class="h-full rounded-full transition-all duration-500" style="width: {clamp(node.host.cpuPercent)}%; background: var(--bar-fill);"></div>
							</div>
							<div class="text-[11px] mt-1.5" style="color: var(--text-tertiary);">load {(node.host.loadAvg?.[0] || 0).toFixed(2)} · {node.host.cpuCores || 0} cores</div>
						</button>
						<!-- Memory -->
						<button type="button" onclick={() => toggle(node.nodeID, 'memory')}
							class="p-4 text-left cursor-pointer transition-colors"
							style="background: {isOpen(node.nodeID, 'memory') ? 'var(--hover-bg)' : 'var(--bg-secondary)'};"
							onmouseenter={(e) => { if (!isOpen(node.nodeID, 'memory')) e.currentTarget.style.background = 'var(--hover-bg)'; }}
							onmouseleave={(e) => { if (!isOpen(node.nodeID, 'memory')) e.currentTarget.style.background = 'var(--bg-secondary)'; }}>
							<div class="flex items-center justify-between">
								<span class="text-[10.5px] font-semibold uppercase tracking-wider" style="color: var(--text-muted);">Memory</span>
								<svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" style="color: var(--text-muted); transition: transform .2s; transform: rotate({isOpen(node.nodeID, 'memory') ? 180 : 0}deg);"><polyline points="6 9 12 15 18 9"/></svg>
							</div>
							<div class="text-[20px] font-semibold mt-1 font-[JetBrains_Mono,monospace]" style="color: var(--text-primary);">{memPct(node.host.memUsedMB, node.host.memTotalMB).toFixed(0)}<span class="text-[13px]" style="color: var(--text-tertiary);">%</span></div>
							<div class="w-full h-1 rounded-full overflow-hidden mt-2" style="background: var(--bar-bg);">
								<div class="h-full rounded-full transition-all duration-500" style="width: {memPct(node.host.memUsedMB, node.host.memTotalMB)}%; background: var(--bar-fill);"></div>
							</div>
							<div class="text-[11px] mt-1.5 font-[JetBrains_Mono,monospace]" style="color: var(--text-tertiary);">{fmtGB(node.host.memUsedMB)} / {fmtGB(node.host.memTotalMB)} GB</div>
						</button>
						<!-- GPUs -->
						<button type="button" onclick={() => toggle(node.nodeID, 'gpu')}
							class="p-4 text-left cursor-pointer transition-colors"
							style="background: {isOpen(node.nodeID, 'gpu') ? 'var(--hover-bg)' : 'var(--bg-secondary)'};"
							onmouseenter={(e) => { if (!isOpen(node.nodeID, 'gpu')) e.currentTarget.style.background = 'var(--hover-bg)'; }}
							onmouseleave={(e) => { if (!isOpen(node.nodeID, 'gpu')) e.currentTarget.style.background = 'var(--bg-secondary)'; }}>
							<div class="flex items-center justify-between">
								<span class="text-[10.5px] font-semibold uppercase tracking-wider" style="color: var(--text-muted);">GPUs</span>
								<svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" style="color: var(--text-muted); transition: transform .2s; transform: rotate({isOpen(node.nodeID, 'gpu') ? 180 : 0}deg);"><polyline points="6 9 12 15 18 9"/></svg>
							</div>
							<div class="text-[20px] font-semibold mt-1 font-[JetBrains_Mono,monospace]" style="color: var(--text-primary);">{node.gpus?.length || 0}</div>
							<div class="text-[11px] mt-2.5" style="color: var(--text-tertiary);">{node.gpuProcesses?.length || 0} GPU proc{(node.gpuProcesses?.length || 0) !== 1 ? 's' : ''}</div>
						</button>
					</div>

					<!-- Expanded detail panel -->
					{#if expandedKey.startsWith(node.nodeID + ':')}
						<div class="px-4 py-3" style="border-top: 1px solid var(--border);">
							<div class="flex items-center justify-between mb-3">
								<span class="text-[11px] font-semibold uppercase tracking-wider" style="color: var(--text-muted);">
									{isOpen(node.nodeID, 'cpu') ? 'CPU cores & top processes' : isOpen(node.nodeID, 'memory') ? 'Top processes by memory' : 'GPU processes'}
								</span>
								<button type="button" onclick={() => (sortAsc = !sortAsc)}
									class="text-[11px] px-2 py-1 rounded font-medium cursor-pointer"
									style="background: var(--bg-tertiary); color: var(--text-secondary);">
									{sortAsc ? 'Ascending ↑' : 'Descending ↓'}
								</button>
							</div>

							{#if isOpen(node.nodeID, 'cpu')}
								{#if node.host.perCoreCPU?.length}
									<div class="grid grid-cols-8 gap-1.5 mb-4">
										{#each node.host.perCoreCPU as c, i}
											<div class="rounded p-1.5" style="background: var(--bg-tertiary);">
												<div class="text-[9px] font-[JetBrains_Mono,monospace]" style="color: var(--text-muted);">c{i}</div>
												<div class="text-[12px] font-[JetBrains_Mono,monospace]" style="color: var(--text-primary);">{(c || 0).toFixed(0)}%</div>
												<div class="h-1 rounded-full mt-1 overflow-hidden" style="background: var(--bar-bg);"><div class="h-full rounded-full" style="width:{clamp(c)}%; background: var(--bar-fill);"></div></div>
											</div>
										{/each}
									</div>
								{/if}
								{@render procTable(sortedProcs(node.processes, 'cpu'))}
							{:else if isOpen(node.nodeID, 'memory')}
								{@render procTable(sortedProcs(node.processes, 'mem'))}
							{:else if isOpen(node.nodeID, 'gpu')}
								{#if node.gpuProcesses?.length}
									<div class="overflow-x-auto">
										<table class="w-full">
											<thead><tr>
												<th class="text-left py-1.5 pr-4 text-[10.5px] font-medium uppercase tracking-wider" style="color: var(--text-tertiary);">PID</th>
												<th class="text-left py-1.5 pr-4 text-[10.5px] font-medium uppercase tracking-wider" style="color: var(--text-tertiary);">Process</th>
												<th class="text-right py-1.5 text-[10.5px] font-medium uppercase tracking-wider" style="color: var(--text-tertiary);">GPU Mem</th>
											</tr></thead>
											<tbody>
												{#each sortedGpuProcs(node.gpuProcesses) as p (p.pid)}
													<tr style="border-top: 1px solid var(--border);">
														<td class="py-1.5 pr-4 text-[12px] font-[JetBrains_Mono,monospace]" style="color: var(--text-tertiary);">{p.pid}</td>
														<td class="py-1.5 pr-4 text-[12.5px] truncate max-w-[320px]" style="color: var(--text-primary);">{p.name}</td>
														<td class="py-1.5 text-[12px] font-[JetBrains_Mono,monospace] text-right" style="color: var(--text-secondary);">{fmtMem(p.memMB)}</td>
													</tr>
												{/each}
											</tbody>
										</table>
									</div>
								{:else}
									<div class="text-[12.5px]" style="color: var(--text-muted);">No GPU processes (nvidia-smi unavailable or no compute apps).</div>
								{/if}
							{/if}
						</div>
					{/if}

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
