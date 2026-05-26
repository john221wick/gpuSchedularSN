<script>
	import { onMount } from 'svelte';
	import { SubmitJob, GetDevices } from '$lib/api.js';
	import { goto } from '$app/navigation';

	let command = $state('');
	let numGPUs = $state(1);
	let minVRAM = $state(0);
	let vramUnit = $state('GB');
	let priority = $state(10);
	let totalGPUs = $state(0);
	let submitting = $state(false);
	let error = $state('');
	let success = $state('');

	onMount(async () => {
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
			const jobID = await SubmitJob({
				command: command,
				numGPUs: numGPUs,
				minVRAMMB: minVRAMMB,
				priority: priority
			});
			success = `Job submitted: ${jobID}`;
			command = '';
			numGPUs = 1;
			minVRAM = 0;
			priority = 10;
			setTimeout(() => goto('/jobs'), 1500);
		} catch (e) {
			error = e?.message || String(e);
		} finally {
			submitting = false;
		}
	}
</script>

<div class="p-6 max-w-2xl">
	<div class="mb-6">
		<h1 class="text-xl font-semibold text-white">Run Job</h1>
		<p class="text-sm text-[#8888aa] mt-1">Submit a command to run on allocated GPUs</p>
	</div>

	<form onsubmit={handleSubmit} class="bg-[#16162a] rounded-xl border border-[#2a2a4a] p-6 space-y-5">
		<!-- Command -->
		<div>
			<label for="command" class="block text-[11px] font-medium uppercase tracking-wider text-[#6666aa] mb-1.5">
				Command
			</label>
			<input
				id="command"
				type="text"
				bind:value={command}
				placeholder="e.g. python train.py --epochs 10"
				required
				class="w-full bg-[#1a1a2e] border border-[#2a2a4a] rounded-lg px-3 py-2.5 text-sm text-white font-mono
					placeholder-[#4a4a6a] focus:outline-none focus:border-[#4f6ef7] focus:ring-1 focus:ring-[#4f6ef7]"
			/>
		</div>

		<!-- GPU count + VRAM row -->
		<div class="grid grid-cols-2 gap-4">
			<div>
				<label for="gpus" class="block text-[11px] font-medium uppercase tracking-wider text-[#6666aa] mb-1.5">
					Number of GPUs
				</label>
				<input
					id="gpus"
					type="number"
					bind:value={numGPUs}
					min="1"
					max={totalGPUs || 8}
					class="w-full bg-[#1a1a2e] border border-[#2a2a4a] rounded-lg px-3 py-2.5 text-sm text-white
						focus:outline-none focus:border-[#4f6ef7] focus:ring-1 focus:ring-[#4f6ef7]"
				/>
				<p class="text-[11px] text-[#6666aa] mt-1">{totalGPUs} GPU{totalGPUs !== 1 ? 's' : ''} available</p>
			</div>

			<div>
				<label for="vram" class="block text-[11px] font-medium uppercase tracking-wider text-[#6666aa] mb-1.5">
					Min VRAM per GPU
				</label>
				<div class="flex gap-2">
					<input
						id="vram"
						type="number"
						bind:value={minVRAM}
						min="0"
						class="flex-1 bg-[#1a1a2e] border border-[#2a2a4a] rounded-lg px-3 py-2.5 text-sm text-white
							focus:outline-none focus:border-[#4f6ef7] focus:ring-1 focus:ring-[#4f6ef7]"
					/>
					<select
						bind:value={vramUnit}
						class="bg-[#1a1a2e] border border-[#2a2a4a] rounded-lg px-2 py-2.5 text-sm text-white
							focus:outline-none focus:border-[#4f6ef7]"
					>
						<option value="GB">GB</option>
						<option value="MB">MB</option>
					</select>
				</div>
			</div>
		</div>

		<!-- Priority -->
		<div>
			<label for="priority" class="block text-[11px] font-medium uppercase tracking-wider text-[#6666aa] mb-1.5">
				Priority (lower = higher)
			</label>
			<input
				id="priority"
				type="number"
				bind:value={priority}
				min="1"
				max="100"
				class="w-32 bg-[#1a1a2e] border border-[#2a2a4a] rounded-lg px-3 py-2.5 text-sm text-white
					focus:outline-none focus:border-[#4f6ef7] focus:ring-1 focus:ring-[#4f6ef7]"
			/>
		</div>

		<!-- Messages -->
		{#if error}
			<div class="flex items-center gap-2 px-3 py-2 rounded-lg bg-red-500/10 border border-red-500/20 text-sm text-red-400">
				<svg class="w-4 h-4 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
					<path stroke-linecap="round" stroke-linejoin="round" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
				</svg>
				{error}
			</div>
		{/if}

		{#if success}
			<div class="flex items-center gap-2 px-3 py-2 rounded-lg bg-emerald-500/10 border border-emerald-500/20 text-sm text-emerald-400">
				<svg class="w-4 h-4 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
					<path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" />
				</svg>
				{success}
			</div>
		{/if}

		<!-- Submit -->
		<button
			type="submit"
			disabled={submitting || !command.trim()}
			class="w-full py-2.5 rounded-lg text-sm font-medium bg-[#4f6ef7] text-white
				hover:bg-[#3d5bd4] disabled:opacity-40 disabled:cursor-not-allowed transition-colors cursor-pointer"
		>
			{submitting ? 'Submitting...' : 'Submit Job'}
		</button>
	</form>
</div>
