<script>
	import { onMount } from 'svelte';
	import { GetRunningJobs, GetQueueLength, KillJob } from '$lib/api.js';

	let jobs = $state([]);
	let queueLen = $state(0);
	let interval;

	async function refresh() {
		try {
			jobs = await GetRunningJobs();
			queueLen = await GetQueueLength();
		} catch (e) {
			console.error('Jobs refresh failed:', e);
		}
	}

	async function handleKill(jobId) {
		try {
			await KillJob(jobId);
			await refresh();
		} catch (e) {
			console.error('Kill failed:', e);
		}
	}

	onMount(() => {
		refresh();
		interval = setInterval(refresh, 2000);
		return () => clearInterval(interval);
	});

	function statusColor(status) {
		switch (status) {
			case 'Running': return 'bg-emerald-500/15 text-emerald-400';
			case 'Queued': return 'bg-amber-500/15 text-amber-400';
			case 'Done': return 'bg-[#2a2a4a] text-[#8888aa]';
			case 'Failed': return 'bg-red-500/15 text-red-400';
			default: return 'bg-[#2a2a4a] text-[#8888aa]';
		}
	}

	function timeAgo(isoStr) {
		if (!isoStr || isoStr.startsWith('0001')) return '—';
		const d = new Date(isoStr);
		const now = new Date();
		const sec = Math.floor((now - d) / 1000);
		if (sec < 60) return `${sec}s ago`;
		if (sec < 3600) return `${Math.floor(sec / 60)}m ago`;
		return `${Math.floor(sec / 3600)}h ago`;
	}
</script>

<div class="p-6 space-y-6">
	<div class="flex items-center justify-between">
		<div>
			<h1 class="text-xl font-semibold text-white">Jobs</h1>
			<p class="text-sm text-[#8888aa] mt-1">
				{jobs.length} running · {queueLen} queued
			</p>
		</div>
		<a
			href="/submit"
			class="inline-flex items-center gap-1.5 px-3.5 py-1.5 rounded-lg text-sm font-medium bg-[#4f6ef7] text-white hover:bg-[#3d5bd4] transition-colors"
		>
			<svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
				<path stroke-linecap="round" stroke-linejoin="round" d="M12 4v16m8-8H4" />
			</svg>
			Run Job
		</a>
	</div>

	{#if jobs.length > 0}
		<!-- Jobs table -->
		<div class="bg-[#16162a] rounded-xl border border-[#2a2a4a] overflow-hidden">
			<table class="w-full">
				<thead>
					<tr class="border-b border-[#2a2a4a]">
						<th class="text-left px-4 py-3 text-[11px] font-medium uppercase tracking-wider text-[#6666aa]">Job ID</th>
						<th class="text-left px-4 py-3 text-[11px] font-medium uppercase tracking-wider text-[#6666aa]">Command</th>
						<th class="text-left px-4 py-3 text-[11px] font-medium uppercase tracking-wider text-[#6666aa]">GPUs</th>
						<th class="text-left px-4 py-3 text-[11px] font-medium uppercase tracking-wider text-[#6666aa]">Status</th>
						<th class="text-left px-4 py-3 text-[11px] font-medium uppercase tracking-wider text-[#6666aa]">Started</th>
						<th class="text-right px-4 py-3 text-[11px] font-medium uppercase tracking-wider text-[#6666aa]">Actions</th>
					</tr>
				</thead>
				<tbody>
					{#each jobs as job}
						<tr class="border-b border-[#2a2a4a] last:border-0 hover:bg-[#1e1e3a] transition-colors">
							<td class="px-4 py-3 text-xs font-mono text-[#8888aa]">{job.id.slice(0, 16)}…</td>
							<td class="px-4 py-3 text-sm text-white font-mono">{job.command}</td>
							<td class="px-4 py-3">
								<div class="flex gap-1">
									{#each job.gpuIDs || [] as gpuId}
										<span class="text-[11px] px-1.5 py-0.5 rounded bg-[#2a2a4a] text-[#8888aa] font-mono">{gpuId}</span>
									{/each}
								</div>
							</td>
							<td class="px-4 py-3">
								<span class="text-[11px] px-2 py-0.5 rounded-md font-medium {statusColor(job.status)}">
									{job.status}
								</span>
							</td>
							<td class="px-4 py-3 text-xs text-[#8888aa]">{timeAgo(job.startedAt)}</td>
							<td class="px-4 py-3 text-right">
								{#if job.status === 'Running'}
									<button
										onclick={() => handleKill(job.id)}
										class="text-[11px] px-2 py-1 rounded-md bg-red-500/10 text-red-400 hover:bg-red-500/20 transition-colors font-medium cursor-pointer"
									>
										Kill
									</button>
								{/if}
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{:else}
		<div class="bg-[#16162a] rounded-xl border border-[#2a2a4a] flex flex-col items-center justify-center py-16">
			<svg class="w-12 h-12 text-[#3a3a5c] mb-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1">
				<path stroke-linecap="round" stroke-linejoin="round" d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" />
			</svg>
			<p class="text-[#6666aa] text-sm">No running jobs</p>
			<a href="/submit" class="mt-3 text-sm text-[#4f6ef7] hover:underline">Submit a job</a>
		</div>
	{/if}
</div>
