<script>
	import { onMount, tick } from 'svelte';
	import {
		GetRunningJobs, GetCompletedJobs, GetQueuedJobs, GetQueueLength,
		KillJob, RemoveQueuedJob, UpdateQueuedPriority,
		GetClusterRunningJobs, GetClusterCompletedJobs, GetClusterQueuedJobs, ClusterKillJob,
		GetJobLogs
	} from '../lib/api.js';

	let { remoteMode = false } = $props();
	let jobs = $state([]);
	let completed = $state([]);
	let queued = $state([]);
	let queueLen = $state(0);
	let showQueue = $state(true);
	let showCompleted = $state(false);
	let editingPriority = $state(null);
	let editPriorityValue = $state('');
	let pendingKillJob = $state(null);
	let killingJobId = $state(null);
	let killError = $state('');
	let expandedLogs = $state(null);
	let logContent = $state('');
	let logOffset = $state(0);
	let logEOF = $state(false);
	let logError = $state('');
	let logInterval = null;
	let interval;

	async function refresh() {
		try {
			if (remoteMode) {
				jobs = await GetClusterRunningJobs() || [];
				completed = await GetClusterCompletedJobs() || [];
				queued = showQueue ? (await GetClusterQueuedJobs() || []) : queued;
				queueLen = queued.length;
			} else {
				jobs = await GetRunningJobs();
				completed = await GetCompletedJobs();
				queueLen = await GetQueueLength();
				if (showQueue) {
					queued = await GetQueuedJobs();
				}
			}
		} catch (e) {
			console.error('Jobs refresh failed:', e);
		}
	}

	function handleKill(job) {
		pendingKillJob = job;
		killError = '';
	}

	function closeKillConfirm() {
		if (killingJobId) return;
		pendingKillJob = null;
		killError = '';
	}

	async function confirmKill() {
		if (!pendingKillJob) return;

		const jobId = pendingKillJob.id;
		killingJobId = jobId;
		killError = '';

		try {
			if (remoteMode) {
				await ClusterKillJob(jobId);
			} else {
				await KillJob(jobId);
			}
			pendingKillJob = null;
			await refresh();
		} catch (e) {
			console.error('Kill failed:', e);
			killError = e?.message || String(e);
		} finally {
			killingJobId = null;
		}
	}

	async function handleRemoveQueued(jobId) {
		if (!confirm(`Remove job ${jobId.slice(0, 16)}… from queue?`)) return;
		try {
			await RemoveQueuedJob(jobId);
			await refresh();
		} catch (e) {
			console.error('Remove queued job failed:', e);
		}
	}

	function startEditPriority(jobId, currentPriority) {
		editingPriority = jobId;
		editPriorityValue = String(currentPriority);
	}

	async function savePriority(jobId) {
		const val = parseInt(editPriorityValue, 10);
		if (!isNaN(val) && val > 0) {
			try {
				await UpdateQueuedPriority(jobId, val);
				await refresh();
			} catch (e) {
				console.error('Update priority failed:', e);
			}
		}
		editingPriority = null;
	}

	function handlePriorityKeydown(e, jobId) {
		if (e.key === 'Enter') savePriority(jobId);
		if (e.key === 'Escape') editingPriority = null;
	}

	async function toggleLogs(jobId) {
		if (expandedLogs === jobId) {
			expandedLogs = null;
			logContent = '';
			logOffset = 0;
			logEOF = false;
			logError = '';
			if (logInterval) { clearInterval(logInterval); logInterval = null; }
			return;
		}
		expandedLogs = jobId;
		logContent = '';
		logOffset = 0;
		logEOF = false;
		logError = '';
		await fetchLogs(jobId);
		// Auto-refresh logs for running jobs
		if (logInterval) clearInterval(logInterval);
		logInterval = setInterval(() => {
			if (expandedLogs === jobId && !logEOF) fetchLogs(jobId);
		}, 2000);
	}

	async function fetchLogs(jobId) {
		try {
			const result = await GetJobLogs(jobId, logOffset);
			if (result) {
				if (result.data && result.data.length > 0) {
					logContent += result.data;
				}
				if (result.offset !== undefined) {
					logOffset = result.offset;
				}
				logEOF = result.eof || false;
				if (logEOF && logInterval) {
					clearInterval(logInterval);
					logInterval = null;
				}
				await tick();
				const el = document.getElementById('log-container');
				if (el) el.scrollTop = el.scrollHeight;
			}
			logError = '';
		} catch (e) {
			logError = e?.message || String(e);
		}
	}

	async function toggleQueue() {
		showQueue = !showQueue;
		if (showQueue) {
			queued = await GetQueuedJobs();
		}
	}

	onMount(() => {
		refresh();
		interval = setInterval(refresh, 2000);
		return () => {
			clearInterval(interval);
			if (logInterval) clearInterval(logInterval);
		};
	});

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
	<div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
		<div>
			<h1 class="text-lg font-semibold" style="color: var(--text-primary);">Jobs</h1>
			<p class="text-[13px] mt-0.5" style="color: var(--text-tertiary);">
				{jobs.length} running · {queueLen} queued · {completed.length} completed
			</p>
		</div>
		<div class="flex flex-wrap items-center gap-2 sm:gap-3">
			<button
				onclick={toggleQueue}
				class="px-3 py-1.5 rounded-md text-[13px] font-medium transition-colors cursor-pointer"
				style="background: {showQueue ? 'var(--accent)' : 'var(--bg-tertiary)'}; color: {showQueue ? 'var(--accent-text)' : 'var(--text-secondary)'};"
			>
				{showQueue ? 'Hide Queue' : 'Show Queue'}
			</button>
			<button
				onclick={() => showCompleted = !showCompleted}
				class="px-3 py-1.5 rounded-md text-[13px] font-medium transition-colors cursor-pointer"
				style="background: {showCompleted ? 'var(--accent)' : 'var(--bg-tertiary)'}; color: {showCompleted ? 'var(--accent-text)' : 'var(--text-secondary)'};"
			>
				{showCompleted ? 'Hide Completed' : 'Show Completed'}
			</button>
			<a
				href="#/submit"
				class="inline-flex items-center px-3.5 py-1.5 rounded-md text-[13px] font-medium transition-colors"
				style="background: var(--accent); color: var(--accent-text);"
			>
				Run Job
			</a>
		</div>
	</div>

	<!-- Running jobs -->
	{#if jobs.length > 0}
		<div>
			<h2 class="text-[12px] font-medium uppercase tracking-wider mb-2" style="color: var(--text-tertiary);">Running</h2>
			<div class="rounded-lg overflow-hidden" style="background: var(--bg-secondary); border: 1px solid var(--border);">
				<table class="w-full">
					<thead>
						<tr style="border-bottom: 1px solid var(--border);">
							<th class="text-left px-4 py-2.5 text-[11px] font-medium uppercase tracking-wider" style="color: var(--text-tertiary);">Job ID</th>
							<th class="text-left px-4 py-2.5 text-[11px] font-medium uppercase tracking-wider" style="color: var(--text-tertiary);">Command</th>
							{#if remoteMode}
								<th class="text-left px-4 py-2.5 text-[11px] font-medium uppercase tracking-wider" style="color: var(--text-tertiary);">Node</th>
							{/if}
							<th class="text-left px-4 py-2.5 text-[11px] font-medium uppercase tracking-wider" style="color: var(--text-tertiary);">GPUs</th>
							<th class="text-left px-4 py-2.5 text-[11px] font-medium uppercase tracking-wider" style="color: var(--text-tertiary);">Started</th>
							<th class="text-right px-4 py-2.5 text-[11px] font-medium uppercase tracking-wider" style="color: var(--text-tertiary);">Actions</th>
						</tr>
					</thead>
					<tbody>
						{#each jobs as job (job.id)}
							<tr class="transition-colors" style="border-bottom: 1px solid var(--border);"
								onmouseenter={(e) => e.currentTarget.style.background = 'var(--hover-bg)'}
								onmouseleave={(e) => e.currentTarget.style.background = 'none'}>
								<td class="px-4 py-2.5 text-[11px] font-[JetBrains_Mono,monospace]" style="color: var(--text-tertiary);">{job.id.slice(0, 16)}…</td>
								<td class="px-4 py-2.5 text-[13px] font-[JetBrains_Mono,monospace]" style="color: var(--text-primary);">{job.command}</td>
								{#if remoteMode}
									<td class="px-4 py-2.5 text-[12px]" style="color: var(--text-secondary);">{job.nodeName || '—'}</td>
								{/if}
								<td class="px-4 py-2.5">
									<div class="flex gap-1">
										{#each job.gpuIDs || [] as gpuId (gpuId)}
											<span class="text-[11px] px-1.5 py-0.5 rounded font-[JetBrains_Mono,monospace]" style="background: var(--bg-tertiary); color: var(--text-tertiary);">{gpuId}</span>
										{/each}
									</div>
								</td>
								<td class="px-4 py-2.5 text-[12px]" style="color: var(--text-tertiary);">{timeAgo(job.startedAt)}</td>
								<td class="px-4 py-2.5 text-right">
									<div class="flex items-center justify-end gap-1.5">
										{#if remoteMode}
											<button
												onclick={() => toggleLogs(job.id)}
												class="text-[11px] px-2 py-1 rounded transition-colors font-medium cursor-pointer"
												style="background: {expandedLogs === job.id ? 'var(--accent)' : 'var(--bg-tertiary)'}; color: {expandedLogs === job.id ? 'var(--accent-text)' : 'var(--text-secondary)'};"
											>
												Logs
											</button>
										{/if}
										<button
											onclick={() => handleKill(job)}
											class="text-[11px] px-2 py-1 rounded bg-red-500/10 text-red-500 hover:bg-red-500/20 transition-colors font-medium cursor-pointer"
										>
											Kill
										</button>
									</div>
								</td>
							</tr>
							{#if expandedLogs === job.id}
								<tr>
									<td colspan={remoteMode ? 6 : 5} class="px-4 py-3" style="background: var(--bg-primary);">
										{#if logError}
											<div class="rounded-md px-3 py-2 text-[12px] mb-2" style="background: rgba(239,68,68,0.1); color: rgb(239,68,68);">
												{logError}
											</div>
										{/if}
										<div
											id="log-container"
											class="rounded-md p-3 text-[11px] font-[JetBrains_Mono,monospace] leading-relaxed overflow-auto max-h-[300px] whitespace-pre-wrap"
											style="background: #0d0d0d; color: #d4d4d4; border: 1px solid var(--border);"
										>
											{logContent || (logError ? '' : 'Loading logs...')}
										</div>
										{#if !logEOF}
											<p class="text-[10px] mt-1.5" style="color: var(--text-muted);">Streaming… auto-refreshes every 2s</p>
										{:else}
											<p class="text-[10px] mt-1.5" style="color: var(--text-muted);">End of logs</p>
										{/if}
									</td>
								</tr>
							{/if}
						{/each}
					</tbody>
				</table>
			</div>
		</div>
	{:else}
		<div class="rounded-lg flex flex-col items-center justify-center py-12" style="background: var(--bg-secondary); border: 1px solid var(--border);">
			<p class="text-[13px]" style="color: var(--text-tertiary);">No running jobs</p>
			<a href="#/submit" class="mt-2 text-[13px] hover:underline" style="color: var(--text-primary);">Submit a job</a>
		</div>
	{/if}

	<!-- Completed/failed jobs -->
	{#if showCompleted}
		<div>
			<h2 class="text-[12px] font-medium uppercase tracking-wider mb-2" style="color: var(--text-tertiary);">Completed ({completed.length})</h2>
			{#if completed.length > 0}
				<div class="rounded-lg overflow-hidden" style="background: var(--bg-secondary); border: 1px solid var(--border);">
					<table class="w-full">
						<thead>
							<tr style="border-bottom: 1px solid var(--border);">
								<th class="text-left px-4 py-2.5 text-[11px] font-medium uppercase tracking-wider" style="color: var(--text-tertiary);">Job ID</th>
								<th class="text-left px-4 py-2.5 text-[11px] font-medium uppercase tracking-wider" style="color: var(--text-tertiary);">Command</th>
								<th class="text-left px-4 py-2.5 text-[11px] font-medium uppercase tracking-wider" style="color: var(--text-tertiary);">Status</th>
								<th class="text-left px-4 py-2.5 text-[11px] font-medium uppercase tracking-wider" style="color: var(--text-tertiary);">GPUs</th>
								<th class="text-left px-4 py-2.5 text-[11px] font-medium uppercase tracking-wider" style="color: var(--text-tertiary);">Started</th>
								<th class="text-right px-4 py-2.5 text-[11px] font-medium uppercase tracking-wider" style="color: var(--text-tertiary);">Actions</th>
							</tr>
						</thead>
						<tbody>
							{#each completed as job (job.id)}
								<tr class="transition-colors" style="border-bottom: 1px solid var(--border);"
									onmouseenter={(e) => e.currentTarget.style.background = 'var(--hover-bg)'}
									onmouseleave={(e) => e.currentTarget.style.background = 'none'}>
									<td class="px-4 py-2.5 text-[11px] font-[JetBrains_Mono,monospace]" style="color: var(--text-tertiary);">{job.id.slice(0, 16)}…</td>
									<td class="px-4 py-2.5 text-[13px] font-[JetBrains_Mono,monospace]" style="color: var(--text-primary);">{job.command}</td>
									<td class="px-4 py-2.5">
										<span class="text-[11px] px-1.5 py-0.5 rounded font-medium"
											style="{job.status === 'Failed' ? 'background: rgba(239,68,68,0.1); color: rgb(239,68,68);' : 'background: var(--bg-tertiary); color: var(--text-tertiary);'}">
											{job.status}
										</span>
									</td>
									<td class="px-4 py-2.5">
										<div class="flex gap-1">
											{#each job.gpuIDs || [] as gpuId (gpuId)}
												<span class="text-[11px] px-1.5 py-0.5 rounded font-[JetBrains_Mono,monospace]" style="background: var(--bg-tertiary); color: var(--text-tertiary);">{gpuId}</span>
											{/each}
										</div>
									</td>
									<td class="px-4 py-2.5 text-[12px]" style="color: var(--text-tertiary);">{timeAgo(job.startedAt)}</td>
									<td class="px-4 py-2.5 text-right">
										<button
											onclick={() => toggleLogs(job.id)}
											class="text-[11px] px-2 py-1 rounded transition-colors font-medium cursor-pointer"
											style="background: {expandedLogs === job.id ? 'var(--accent)' : 'var(--bg-tertiary)'}; color: {expandedLogs === job.id ? 'var(--accent-text)' : 'var(--text-secondary)'};"
										>
											Logs
										</button>
									</td>
								</tr>
								{#if expandedLogs === job.id}
									<tr>
										<td colspan="6" class="px-4 py-3" style="background: var(--bg-primary);">
											{#if logError}
												<div class="rounded-md px-3 py-2 text-[12px] mb-2" style="background: rgba(239,68,68,0.1); color: rgb(239,68,68);">
													{logError}
												</div>
											{/if}
											<div
												id="log-container"
												class="rounded-md p-3 text-[11px] font-[JetBrains_Mono,monospace] leading-relaxed overflow-auto max-h-[300px] whitespace-pre-wrap"
												style="background: #0d0d0d; color: #d4d4d4; border: 1px solid var(--border);"
											>
												{logContent || (logError ? '' : 'Loading logs...')}
											</div>
											<p class="text-[10px] mt-1.5" style="color: var(--text-muted);">End of logs</p>
										</td>
									</tr>
								{/if}
							{/each}
						</tbody>
					</table>
				</div>
			{:else}
				<div class="rounded-lg py-8 text-center" style="background: var(--bg-secondary); border: 1px solid var(--border);">
					<p class="text-[13px]" style="color: var(--text-tertiary);">No completed jobs</p>
				</div>
			{/if}
		</div>
	{/if}

	<!-- Queued jobs -->
	{#if showQueue}
		<div>
			<h2 class="text-[12px] font-medium uppercase tracking-wider mb-2" style="color: var(--text-tertiary);">Queue ({queueLen})</h2>
			{#if queued.length > 0}
				<div class="rounded-lg overflow-hidden" style="background: var(--bg-secondary); border: 1px solid var(--border);">
					<table class="w-full">
						<thead>
							<tr style="border-bottom: 1px solid var(--border);">
								<th class="text-left px-4 py-2.5 text-[11px] font-medium uppercase tracking-wider" style="color: var(--text-tertiary);">Job ID</th>
								<th class="text-left px-4 py-2.5 text-[11px] font-medium uppercase tracking-wider" style="color: var(--text-tertiary);">Command</th>
								<th class="text-left px-4 py-2.5 text-[11px] font-medium uppercase tracking-wider" style="color: var(--text-tertiary);">GPUs</th>
								<th class="text-left px-4 py-2.5 text-[11px] font-medium uppercase tracking-wider" style="color: var(--text-tertiary);">Min VRAM</th>
								<th class="text-left px-4 py-2.5 text-[11px] font-medium uppercase tracking-wider" style="color: var(--text-tertiary);">Priority</th>
								<th class="text-left px-4 py-2.5 text-[11px] font-medium uppercase tracking-wider" style="color: var(--text-tertiary);">Submitted</th>
								<th class="text-right px-4 py-2.5 text-[11px] font-medium uppercase tracking-wider" style="color: var(--text-tertiary);">Actions</th>
							</tr>
						</thead>
						<tbody>
							{#each queued as job (job.id)}
								<tr class="transition-colors" style="border-bottom: 1px solid var(--border);"
									onmouseenter={(e) => e.currentTarget.style.background = 'var(--hover-bg)'}
									onmouseleave={(e) => e.currentTarget.style.background = 'none'}>
									<td class="px-4 py-2.5 text-[11px] font-[JetBrains_Mono,monospace]" style="color: var(--text-tertiary);">{job.id.slice(0, 16)}…</td>
									<td class="px-4 py-2.5 text-[13px] font-[JetBrains_Mono,monospace]" style="color: var(--text-primary);">{job.command}</td>
									<td class="px-4 py-2.5 text-[13px] font-[JetBrains_Mono,monospace]" style="color: var(--text-secondary);">{job.numGPUs}</td>
									<td class="px-4 py-2.5 text-[12px] font-[JetBrains_Mono,monospace]" style="color: var(--text-secondary);">
										{job.minVRAMMB ? (job.minVRAMMB / 1024).toFixed(0) + ' GB' : '—'}
									</td>
									<td class="px-4 py-2.5">
										{#if editingPriority === job.id}
											<input
												type="number"
												min="1"
												value={editPriorityValue}
												oninput={(e) => editPriorityValue = e.target.value}
												onkeydown={(e) => handlePriorityKeydown(e, job.id)}
												onblur={() => savePriority(job.id)}
												class="w-16 px-1.5 py-0.5 rounded text-[12px] font-[JetBrains_Mono,monospace] outline-none"
												style="background: var(--input-bg); color: var(--text-primary); border: 1px solid var(--accent);"
											/>
										{:else}
											<button
												onclick={() => startEditPriority(job.id, job.priority)}
												class="text-[12px] font-[JetBrains_Mono,monospace] px-1.5 py-0.5 rounded cursor-pointer transition-colors"
												style="color: var(--text-secondary); background: none;"
												title="Click to edit priority"
											>
												{job.priority}
											</button>
										{/if}
									</td>
									<td class="px-4 py-2.5 text-[12px]" style="color: var(--text-tertiary);">{timeAgo(job.submittedAt)}</td>
									<td class="px-4 py-2.5 text-right">
										<button
											onclick={() => handleRemoveQueued(job.id)}
											class="text-[11px] px-2 py-1 rounded bg-red-500/10 text-red-500 hover:bg-red-500/20 transition-colors font-medium cursor-pointer"
										>
											Remove
										</button>
									</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>
			{:else}
				<div class="rounded-lg py-8 text-center" style="background: var(--bg-secondary); border: 1px solid var(--border);">
					<p class="text-[13px]" style="color: var(--text-tertiary);">Queue empty</p>
				</div>
			{/if}
		</div>
	{/if}

	{#if pendingKillJob}
		<div
			class="fixed inset-0 z-50 flex items-center justify-center bg-black/45 px-4"
		>
			<div
				class="w-full max-w-sm rounded-lg p-5 shadow-xl"
				role="dialog"
				aria-modal="true"
				aria-labelledby="kill-confirm-title"
				tabindex="-1"
				style="background: var(--bg-primary); border: 1px solid var(--border);"
			>
				<h3 id="kill-confirm-title" class="text-[15px] font-semibold" style="color: var(--text-primary);">
					Kill job?
				</h3>
				<p class="mt-2 text-[13px]" style="color: var(--text-secondary);">
					{pendingKillJob.id.slice(0, 16)}…
				</p>
				<p class="mt-2 break-words text-[12px] font-[JetBrains_Mono,monospace]" style="color: var(--text-tertiary);">
					{pendingKillJob.command}
				</p>

				{#if killError}
					<p class="mt-3 rounded-md px-3 py-2 text-[12px]" style="background: rgba(239,68,68,0.1); color: rgb(239,68,68);">
						{killError}
					</p>
				{/if}

				<div class="mt-5 flex justify-end gap-2">
					<button
						type="button"
						onclick={closeKillConfirm}
						disabled={!!killingJobId}
						class="rounded-md px-3 py-1.5 text-[13px] font-medium transition-colors disabled:opacity-60"
						style="background: var(--bg-tertiary); color: var(--text-secondary);"
					>
						Cancel
					</button>
					<button
						type="button"
						onclick={confirmKill}
						disabled={!!killingJobId}
						class="rounded-md bg-red-500 px-3 py-1.5 text-[13px] font-medium text-white transition-colors hover:bg-red-600 disabled:opacity-60"
					>
						{killingJobId === pendingKillJob.id ? 'Killing...' : 'Kill'}
					</button>
				</div>
			</div>
		</div>
	{/if}
</div>
