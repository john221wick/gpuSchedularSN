<script>
	import { onMount } from 'svelte';
	import { GetRunningJobs, GetQueueLength, KillJob } from '../lib/api.js';

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

	function statusStyle(status) {
		switch (status) {
			case 'Running': return 'background: var(--bg-tertiary); color: var(--text-primary);';
			case 'Queued': return 'background: var(--bg-tertiary); color: var(--text-tertiary);';
			case 'Failed': return 'background: rgba(239,68,68,0.1); color: rgb(239,68,68);';
			default: return 'background: var(--bg-tertiary); color: var(--text-tertiary);';
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

<div class="p-8 space-y-6 max-w-[1200px]">
	<div class="flex items-center justify-between">
		<div>
			<h1 class="text-lg font-semibold" style="color: var(--text-primary);">Jobs</h1>
			<p class="text-[13px] mt-0.5" style="color: var(--text-tertiary);">
				{jobs.length} running · {queueLen} queued
			</p>
		</div>
		<a
			href="#/submit"
			class="inline-flex items-center gap-1.5 px-3.5 py-1.5 rounded-md text-[13px] font-medium transition-colors"
			style="background: var(--accent); color: var(--accent-text);"
		>
			Run Job
		</a>
	</div>

	{#if jobs.length > 0}
		<div class="rounded-lg overflow-hidden" style="background: var(--bg-secondary); border: 1px solid var(--border);">
			<table class="w-full">
				<thead>
					<tr style="border-bottom: 1px solid var(--border);">
						<th class="text-left px-4 py-3 text-[11px] font-medium uppercase tracking-wider" style="color: var(--text-tertiary);">Job ID</th>
						<th class="text-left px-4 py-3 text-[11px] font-medium uppercase tracking-wider" style="color: var(--text-tertiary);">Command</th>
						<th class="text-left px-4 py-3 text-[11px] font-medium uppercase tracking-wider" style="color: var(--text-tertiary);">GPUs</th>
						<th class="text-left px-4 py-3 text-[11px] font-medium uppercase tracking-wider" style="color: var(--text-tertiary);">Status</th>
						<th class="text-left px-4 py-3 text-[11px] font-medium uppercase tracking-wider" style="color: var(--text-tertiary);">Started</th>
						<th class="text-right px-4 py-3 text-[11px] font-medium uppercase tracking-wider" style="color: var(--text-tertiary);">Actions</th>
					</tr>
				</thead>
				<tbody>
					{#each jobs as job}
						<tr class="transition-colors" style="border-bottom: 1px solid var(--border);"
							onmouseenter={(e) => e.currentTarget.style.background = 'var(--hover-bg)'}
							onmouseleave={(e) => e.currentTarget.style.background = 'none'}>
							<td class="px-4 py-3 text-[11px] font-[JetBrains_Mono,monospace]" style="color: var(--text-tertiary);">{job.id.slice(0, 16)}…</td>
							<td class="px-4 py-3 text-[13px] font-[JetBrains_Mono,monospace]" style="color: var(--text-primary);">{job.command}</td>
							<td class="px-4 py-3">
								<div class="flex gap-1">
									{#each job.gpuIDs || [] as gpuId}
										<span class="text-[11px] px-1.5 py-0.5 rounded font-[JetBrains_Mono,monospace]" style="background: var(--bg-tertiary); color: var(--text-tertiary);">{gpuId}</span>
									{/each}
								</div>
							</td>
							<td class="px-4 py-3">
								<span class="text-[11px] px-2 py-0.5 rounded font-medium" style="{statusStyle(job.status)}">
									{job.status}
								</span>
							</td>
							<td class="px-4 py-3 text-[12px]" style="color: var(--text-tertiary);">{timeAgo(job.startedAt)}</td>
							<td class="px-4 py-3 text-right">
								{#if job.status === 'Running'}
									<button
										onclick={() => handleKill(job.id)}
										class="text-[11px] px-2 py-1 rounded bg-red-500/10 text-red-500 hover:bg-red-500/20 transition-colors font-medium cursor-pointer"
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
		<div class="rounded-lg flex flex-col items-center justify-center py-16" style="background: var(--bg-secondary); border: 1px solid var(--border);">
			<p class="text-[13px]" style="color: var(--text-tertiary);">No running jobs</p>
			<a href="#/submit" class="mt-2 text-[13px] hover:underline" style="color: var(--text-primary);">Submit a job</a>
		</div>
	{/if}
</div>
