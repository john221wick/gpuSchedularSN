<script>
	import { onMount } from 'svelte';
	import { clearCommandPath, getCommandPath, saveCommandPath } from '../lib/preferences.js';

	let { dark, toggleTheme, remoteMode = false } = $props();

	let commandPath = $state('');
	let saved = $state(false);
	let saveTimer;

	onMount(() => {
		commandPath = getCommandPath();
		return () => clearTimeout(saveTimer);
	});

	function savePath() {
		commandPath = saveCommandPath(commandPath);
		saved = true;
		clearTimeout(saveTimer);
		saveTimer = setTimeout(() => { saved = false; }, 1500);
	}

	function clearPath() {
		commandPath = clearCommandPath();
		saved = true;
		clearTimeout(saveTimer);
		saveTimer = setTimeout(() => { saved = false; }, 1500);
	}
</script>

<div class="p-8 space-y-6 max-w-[760px]">
	<div>
		<h1 class="text-lg font-semibold" style="color: var(--text-primary);">Settings</h1>
		<p class="text-[13px] mt-0.5" style="color: var(--text-tertiary);">Preferences</p>
	</div>

	<section class="rounded-lg p-5 space-y-4" style="background: var(--bg-secondary); border: 1px solid var(--border);">
		<div class="flex items-center justify-between gap-4">
			<div>
				<h2 class="text-[13px] font-semibold" style="color: var(--text-primary);">Theme</h2>
				<p class="text-[12px] mt-0.5" style="color: var(--text-tertiary);">{dark ? 'Dark' : 'Light'}</p>
			</div>
			<button
				type="button"
				onclick={toggleTheme}
				class="rounded-md px-3 py-1.5 text-[13px] font-medium transition-colors cursor-pointer"
				style="background: var(--accent); color: var(--accent-text);"
			>
				{dark ? 'Light mode' : 'Dark mode'}
			</button>
		</div>
	</section>

	{#if !remoteMode}
	<section class="rounded-lg p-5 space-y-4" style="background: var(--bg-secondary); border: 1px solid var(--border);">
		<div>
			<h2 class="text-[13px] font-semibold" style="color: var(--text-primary);">Path Variable</h2>
			<p class="text-[12px] mt-0.5" style="color: var(--text-tertiary);">Base folder or file path</p>
		</div>

		<div>
			<label for="command-path" class="block text-[11px] font-medium uppercase tracking-wider mb-1.5" style="color: var(--text-tertiary);">
				Path
			</label>
			<input
				id="command-path"
				type="text"
				bind:value={commandPath}
				placeholder="/path/to/folder"
				autocomplete="off" autocorrect="off" autocapitalize="off" spellcheck="false" data-form-type="other"
				class="w-full rounded-md px-3 py-2.5 text-[13px] font-[JetBrains_Mono,monospace] focus:outline-none focus:ring-1"
				style="background: var(--input-bg); border: 1px solid var(--border); color: var(--text-primary);"
			/>
		</div>

		<div class="flex flex-wrap items-center gap-2">
			<button
				type="button"
				onclick={savePath}
				class="rounded-md px-3 py-1.5 text-[13px] font-medium transition-colors cursor-pointer"
				style="background: var(--accent); color: var(--accent-text);"
			>
				Save
			</button>
			<button
				type="button"
				onclick={clearPath}
				class="rounded-md px-3 py-1.5 text-[13px] font-medium transition-colors cursor-pointer"
				style="background: var(--bg-tertiary); color: var(--text-secondary);"
			>
				Clear
			</button>
			{#if saved}
				<span class="text-[12px]" style="color: var(--text-tertiary);">Saved</span>
			{/if}
		</div>
	</section>
	{/if}
</div>
