<script>
	import { onMount } from 'svelte';
	import { GetAppVersion, CheckAppUpdate, DownloadAndApplyUpdate, UninstallDesktopApp } from '../lib/api.js';
	import { EventsOn } from '../../wailsjs/runtime/runtime.js';
	import { clearCommandPath, getCommandPath, saveCommandPath } from '../lib/preferences.js';

	let { dark, toggleTheme, remoteMode = false } = $props();

	let commandPath = $state('');
	let saved = $state(false);
	let saveTimer;

	// App update state
	let currentVersion = $state('');
	let latestVersion = $state('');
	let updateAvailable = $state(false);
	let checking = $state(false);
	let updating = $state(false);
	let updateStatus = $state('');
	let progress = $state(0);

	// Uninstall state
	let uninstalling = $state(false);
	let confirmUninstall = $state(false);

	onMount(() => {
		commandPath = getCommandPath();
		const off = EventsOn('app-update-progress', (p) => {
			progress = typeof p === 'number' ? p : 0;
		});
		loadVersion();
		return () => { clearTimeout(saveTimer); if (off) off(); };
	});

	async function loadVersion() {
		try { currentVersion = await GetAppVersion(); } catch { /* ignore */ }
		checkUpdate();
	}

	async function checkUpdate() {
		if (checking) return;
		checking = true;
		updateStatus = 'Checking for updates...';
		try {
			const info = await CheckAppUpdate();
			currentVersion = info?.current || currentVersion;
			latestVersion = info?.latest || '';
			updateAvailable = !!info?.available;
			updateStatus = updateAvailable ? `Update available: ${latestVersion}` : 'Up to date';
		} catch (error) {
			updateStatus = error?.message || String(error);
		} finally {
			checking = false;
		}
	}

	async function updateApp() {
		if (updating) return;
		updating = true;
		progress = 0;
		updateStatus = 'Downloading update...';
		try {
			await DownloadAndApplyUpdate();
			updateStatus = 'Update applied. Restarting...';
		} catch (error) {
			updateStatus = error?.message || String(error);
			updating = false;
		}
	}

	async function uninstallApp() {
		if (uninstalling) return;
		if (!confirmUninstall) { confirmUninstall = true; return; }
		uninstalling = true;
		updateStatus = 'Uninstalling...';
		try {
			const result = await UninstallDesktopApp();
			updateStatus = result?.message || 'Uninstalled.';
		} catch (error) {
			updateStatus = error?.message || String(error);
			uninstalling = false;
			confirmUninstall = false;
		}
	}

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

	<section class="rounded-lg p-5 space-y-4" style="background: var(--bg-secondary); border: 1px solid var(--border);">
		<div class="flex items-center justify-between gap-4">
			<div>
				<h2 class="text-[13px] font-semibold" style="color: var(--text-primary);">App Update</h2>
				<p class="text-[12px] mt-0.5" style="color: var(--text-tertiary);">
					Current version: <span style="color: var(--text-secondary);">{currentVersion || '—'}</span>
				</p>
				<p class="text-[12px] mt-0.5 max-w-[420px]" style="color: var(--text-tertiary);">{updateStatus}</p>
			</div>
			<div class="flex items-center gap-2">
				<button
					type="button"
					onclick={checkUpdate}
					disabled={checking || updating}
					class="rounded-md px-3 py-1.5 text-[13px] font-medium transition-colors cursor-pointer disabled:cursor-not-allowed disabled:opacity-70"
					style="background: var(--bg-tertiary); color: var(--text-secondary);"
				>
					{checking ? 'Checking...' : 'Check'}
				</button>
				<button
					type="button"
					onclick={updateApp}
					disabled={updating || checking || !updateAvailable}
					aria-busy={updating}
					class="rounded-md px-3 py-1.5 text-[13px] font-medium transition-colors cursor-pointer disabled:cursor-not-allowed disabled:opacity-50"
					style="background: var(--accent); color: var(--accent-text);"
				>
					{updating ? 'Updating...' : 'Update now'}
				</button>
			</div>
		</div>

		{#if updating}
			<div class="space-y-1">
				<div class="h-2 w-full rounded-full overflow-hidden" style="background: var(--bg-tertiary);">
					<div class="h-full rounded-full transition-all duration-150" style="width: {progress}%; background: var(--accent);"></div>
				</div>
				<p class="text-[11px] text-right" style="color: var(--text-tertiary);">{progress}%</p>
			</div>
		{/if}
	</section>

	<section class="rounded-lg p-5 space-y-4" style="background: var(--bg-secondary); border: 1px solid var(--border);">
		<div class="flex items-center justify-between gap-4">
			<div>
				<h2 class="text-[13px] font-semibold" style="color: var(--text-primary);">Uninstall</h2>
				<p class="text-[12px] mt-0.5 max-w-[420px]" style="color: var(--text-tertiary);">
					{confirmUninstall ? 'This removes the app, its launcher/icon and saved settings. Click again to confirm.' : 'Remove GPU Scheduler from this computer.'}
				</p>
			</div>
			<div class="flex items-center gap-2">
				{#if confirmUninstall && !uninstalling}
					<button
						type="button"
						onclick={() => (confirmUninstall = false)}
						class="rounded-md px-3 py-1.5 text-[13px] font-medium transition-colors cursor-pointer"
						style="background: var(--bg-tertiary); color: var(--text-secondary);"
					>
						Cancel
					</button>
				{/if}
				<button
					type="button"
					onclick={uninstallApp}
					disabled={uninstalling}
					aria-busy={uninstalling}
					class="rounded-md px-3 py-1.5 text-[13px] font-medium transition-colors cursor-pointer disabled:cursor-not-allowed disabled:opacity-70"
					style="background: #b91c1c; color: #fff;"
				>
					{uninstalling ? 'Uninstalling...' : confirmUninstall ? 'Confirm uninstall' : 'Uninstall'}
				</button>
			</div>
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
