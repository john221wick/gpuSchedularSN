<script>
	import { onMount } from 'svelte';
	import { SubmitJob, ClusterSubmitJob, GetDevices } from '../lib/api.js';
	import { getCommandPath } from '../lib/preferences.js';

	let { remoteMode = false } = $props();
	let command = $state('');
	let numGPUs = $state(1);
	let minVRAM = $state(0);
	let vramUnit = $state('GB');
	let priority = $state(10);
	let totalGPUs = $state(0);
	let submitting = $state(false);
	let error = $state('');
	let success = $state('');
	let commandPath = $state('');

	onMount(async () => {
		commandPath = getCommandPath();

		try {
			const devices = await GetDevices();
			totalGPUs = devices.length;
		} catch (e) {
			console.error('Failed to get devices:', e);
		}
	});

	async function handleSubmit(e) {
		e.preventDefault();
		error = '';
		success = '';
		submitting = true;

		let minVRAMMB = vramUnit === 'GB' ? minVRAM * 1024 : minVRAM;

		try {
			const submitFn = remoteMode ? ClusterSubmitJob : SubmitJob;
			const jobID = await submitFn({
				command: String(command),
				pathVariable: String(commandPath),
				numGPUs: Number(numGPUs),
				minVRAMMB: Number(minVRAMMB),
				priority: Number(priority)
			});
			success = `Job submitted: ${jobID}`;
			command = '';
			numGPUs = 1;
			minVRAM = 0;
			priority = 10;
			setTimeout(() => { window.location.hash = '#/jobs'; }, 1000);
		} catch (err) {
			error = err?.message || String(err);
			console.error('SubmitJob error:', err);
		} finally {
			submitting = false;
		}
	}
</script>

<div class="flex min-h-full items-start justify-center p-4 sm:p-6 lg:p-8">
	<div class="w-full max-w-2xl">
		<div class="mb-6">
			<h1 class="text-lg font-semibold" style="color: var(--text-primary);">Run Job</h1>
			<p class="text-[13px] mt-0.5" style="color: var(--text-tertiary);">Submit a command to run on allocated GPUs</p>
		</div>

		<form onsubmit={handleSubmit} autocomplete="off" class="rounded-lg p-5 space-y-5 sm:p-6" style="background: var(--bg-secondary); border: 1px solid var(--border);">
			<div>
				<label for="command" class="block text-[11px] font-medium uppercase tracking-wider mb-1.5" style="color: var(--text-tertiary);">
					Command
				</label>
				<input
					id="command"
					type="text"
					bind:value={command}
					placeholder="python3 script.py"
					required
					autocomplete="off" autocorrect="off" autocapitalize="off" spellcheck="false" data-form-type="other"
					class="w-full rounded-md px-3 py-2.5 text-[13px] font-[JetBrains_Mono,monospace] focus:outline-none focus:ring-1"
					style="background: var(--input-bg); border: 1px solid var(--border); color: var(--text-primary);"
				/>
				{#if commandPath && !remoteMode}
					<div class="mt-2 flex flex-wrap items-center gap-2">
						<button
							type="button"
							onclick={() => window.location.hash = '#/settings'}
							class="cursor-pointer rounded-md px-2.5 py-1 text-[12px] font-medium transition-colors"
							style="background: transparent; border: 1px solid var(--border); color: var(--text-secondary);"
						>
							Edit path
						</button>
						<span class="text-[12px]" style="color: var(--text-tertiary);">Path</span>
						<span class="break-all text-[12px] font-[JetBrains_Mono,monospace]" style="color: var(--text-tertiary);">
							{commandPath}
						</span>
					</div>
				{/if}
			</div>

			<div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
				<div class="min-w-0">
					<label for="gpus" class="block text-[11px] font-medium uppercase tracking-wider mb-1.5" style="color: var(--text-tertiary);">
						Number of GPUs
					</label>
					<input
						id="gpus"
						type="number"
						bind:value={numGPUs}
						min="1"
						max={totalGPUs || 8}
						class="w-full rounded-md px-3 py-2.5 text-[13px] focus:outline-none focus:ring-1"
						style="background: var(--input-bg); border: 1px solid var(--border); color: var(--text-primary);"
					/>
					<p class="text-[11px] mt-1" style="color: var(--text-tertiary);">{totalGPUs} available</p>
				</div>

				<div class="min-w-0">
					<label for="vram" class="block text-[11px] font-medium uppercase tracking-wider mb-1.5" style="color: var(--text-tertiary);">
						Min VRAM per GPU
					</label>
					<div class="grid min-w-0 grid-cols-[minmax(0,1fr)_5.25rem] gap-2">
						<input
							id="vram"
							type="number"
							bind:value={minVRAM}
							min="0"
							class="min-w-0 rounded-md px-3 py-2.5 text-[13px] focus:outline-none focus:ring-1"
							style="background: var(--input-bg); border: 1px solid var(--border); color: var(--text-primary);"
						/>
						<select
							bind:value={vramUnit}
							aria-label="VRAM unit"
							class="w-full rounded-md px-2 py-2.5 text-[13px] focus:outline-none"
							style="background: var(--input-bg); border: 1px solid var(--border); color: var(--text-primary);"
						>
							<option value="GB">GB</option>
							<option value="MB">MB</option>
						</select>
					</div>
				</div>
			</div>

			<div>
				<label for="priority" class="block text-[11px] font-medium uppercase tracking-wider mb-1.5" style="color: var(--text-tertiary);">
					Priority (lower = higher)
				</label>
				<input
					id="priority"
					type="number"
					bind:value={priority}
					min="1"
					max="100"
					class="w-full rounded-md px-3 py-2.5 text-[13px] focus:outline-none focus:ring-1 sm:w-32"
					style="background: var(--input-bg); border: 1px solid var(--border); color: var(--text-primary);"
				/>
			</div>

			{#if error}
				<div class="px-3 py-2 rounded-md bg-red-500/10 text-[13px] text-red-500">
					{error}
				</div>
			{/if}

			{#if success}
				<div class="px-3 py-2 rounded-md text-[13px]" style="background: var(--bg-tertiary); color: var(--text-primary);">
					{success}
				</div>
			{/if}

			<button
				type="submit"
				disabled={submitting || !command.trim()}
				class="w-full py-2.5 rounded-md text-[13px] font-medium disabled:opacity-30 disabled:cursor-not-allowed transition-colors cursor-pointer"
				style="background: var(--accent); color: var(--accent-text);"
			>
				{submitting ? 'Submitting...' : 'Submit Job'}
			</button>
		</form>
	</div>
</div>
