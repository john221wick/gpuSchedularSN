<script>
	import { onMount } from 'svelte';
	import { GetRemoteMode } from './lib/api.js';
	import Dashboard from './pages/Dashboard.svelte';
	import Devices from './pages/Devices.svelte';
	import Topology from './pages/Topology.svelte';
	import Jobs from './pages/Jobs.svelte';
	import Submit from './pages/Submit.svelte';
	import Settings from './pages/Settings.svelte';
	import Logs from './pages/Logs.svelte';
	import Terminal from './pages/Terminal.svelte';
	import Docs from './pages/Docs.svelte';
	import Monitor from './pages/Monitor.svelte';
	import appicon from './lib/assets/appicon.png';

	let page = $state('dashboard');
	let dark = $state(true);
	let remoteMode = $state(false);
	let collapsed = $state(false);
	let paletteOpen = $state(false);
	let query = $state('');
	let paletteInput;
	let isFullscreen = $state(false);

	const routes = {
		'': 'dashboard',
		'dashboard': 'dashboard',
		'devices': 'devices',
		'topology': 'topology',
		'jobs': 'jobs',
		'submit': 'submit',
		'settings': 'settings',
		'monitor': 'monitor',
		'terminal': 'terminal',
		'logs': 'logs',
		'docs': 'docs'
	};

	const titles = {
		dashboard: 'Dashboard', devices: 'GPUs', topology: 'Topology',
		monitor: 'Monitor', jobs: 'Jobs', submit: 'Run Job', terminal: 'Terminal',
		logs: 'Logs', docs: 'Docs', settings: 'Settings'
	};

	// Lucide-style inline icons (inner SVG)
	const icons = {
		dashboard: '<rect width="7" height="9" x="3" y="3" rx="1"/><rect width="7" height="5" x="14" y="3" rx="1"/><rect width="7" height="9" x="14" y="12" rx="1"/><rect width="7" height="5" x="3" y="16" rx="1"/>',
		devices: '<rect width="16" height="16" x="4" y="4" rx="2"/><rect width="6" height="6" x="9" y="9" rx="1"/><path d="M15 2v2M15 20v2M2 15h2M2 9h2M20 15h2M20 9h2M9 2v2M9 20v2"/>',
		topology: '<circle cx="12" cy="5" r="2.2"/><circle cx="5" cy="19" r="2.2"/><circle cx="19" cy="19" r="2.2"/><path d="M12 7.2v3.3M11 11.5l-4.4 5.6M13 11.5l4.4 5.6"/>',
		nodes: '<rect width="20" height="8" x="2" y="2" rx="2"/><rect width="20" height="8" x="2" y="14" rx="2"/><path d="M6 6h.01M6 18h.01"/>',
		monitor: '<path d="M22 12h-4l-3 9L9 3l-3 9H2"/>',
		jobs: '<line x1="8" y1="6" x2="21" y2="6"/><line x1="8" y1="12" x2="21" y2="12"/><line x1="8" y1="18" x2="21" y2="18"/><line x1="3" y1="6" x2="3.01" y2="6"/><line x1="3" y1="12" x2="3.01" y2="12"/><line x1="3" y1="18" x2="3.01" y2="18"/>',
		submit: '<path d="M6 4.5v15l13-7.5-13-7.5z"/>',
		terminal: '<polyline points="4 17 10 11 4 5"/><line x1="12" y1="19" x2="20" y2="19"/>',
		logs: '<path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/><line x1="16" y1="13" x2="8" y2="13"/><line x1="16" y1="17" x2="8" y2="17"/>',
		docs: '<path d="M4 19.5A2.5 2.5 0 0 1 6.5 17H20"/><path d="M6.5 2H20v20H6.5A2.5 2.5 0 0 1 4 19.5v-15A2.5 2.5 0 0 1 6.5 2z"/>',
		settings: '<path d="M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.1a2 2 0 0 1 1 1.72v.51a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 0 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.39a2 2 0 0 0-.73-2.73l-.15-.08a2 2 0 0 1-1-1.74v-.5a2 2 0 0 1 1-1.74l.15-.09a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2z"/><circle cx="12" cy="12" r="3"/>',
		search: '<circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/>',
		panel: '<rect width="18" height="18" x="3" y="3" rx="2"/><path d="M9 3v18"/>',
		sun: '<circle cx="12" cy="12" r="4"/><path d="M12 2v2M12 20v2M4.93 4.93l1.41 1.41M17.66 17.66l1.41 1.41M2 12h2M20 12h2M6.34 17.66l-1.41 1.41M19.07 4.93l-1.41 1.41"/>',
		moon: '<path d="M12 3a6 6 0 0 0 9 9 9 9 0 1 1-9-9z"/>',
		expand: '<path d="M8 3H5a2 2 0 0 0-2 2v3M21 8V5a2 2 0 0 0-2-2h-3M3 16v3a2 2 0 0 0 2 2h3M16 21h3a2 2 0 0 0 2-2v-3"/>',
		shrink: '<path d="M8 3v3a2 2 0 0 1-2 2H3M21 8h-3a2 2 0 0 1-2-2V3M3 16h3a2 2 0 0 1 2 2v3M16 21v-3a2 2 0 0 1 2-2h3"/>',
		chevron: '<polyline points="9 18 15 12 9 6"/>'
	};

	function handleHash() {
		const hash = window.location.hash.replace('#/', '').replace('#', '');
		page = routes[hash] || 'dashboard';
	}

	onMount(async () => {
		const saved = localStorage.getItem('theme');
		dark = saved ? saved === 'dark' : true;
		applyTheme();

		collapsed = localStorage.getItem('sidebarCollapsed') === '1';

		try { remoteMode = await GetRemoteMode(); } catch {}
		try { if (window.runtime) isFullscreen = await window.runtime.WindowIsFullscreen(); } catch {}

		handleHash();
		window.addEventListener('hashchange', handleHash);
		window.addEventListener('keydown', onKey);
		return () => {
			window.removeEventListener('hashchange', handleHash);
			window.removeEventListener('keydown', onKey);
		};
	});

	function onKey(e) {
		const meta = e.metaKey || e.ctrlKey;
		if (meta && e.key.toLowerCase() === 'b') { e.preventDefault(); toggleSidebar(); }
		else if (meta && e.key.toLowerCase() === 'k') { e.preventDefault(); openPalette(); }
		else if (e.key === 'Escape' && paletteOpen) { paletteOpen = false; }
	}

	function onModeChange(mode) { remoteMode = mode; }

	function applyTheme() {
		document.documentElement.classList.toggle('dark', dark);
		localStorage.setItem('theme', dark ? 'dark' : 'light');
	}

	function toggleTheme() { dark = !dark; applyTheme(); }

	function toggleFullscreen() {
		const rt = window.runtime;
		if (!rt) return;
		if (isFullscreen) rt.WindowUnfullscreen(); else rt.WindowFullscreen();
		isFullscreen = !isFullscreen;
	}

	function toggleSidebar() {
		collapsed = !collapsed;
		localStorage.setItem('sidebarCollapsed', collapsed ? '1' : '0');
	}

	function navigate(route) {
		window.location.hash = '#/' + route;
		paletteOpen = false;
		query = '';
	}

	async function openPalette() {
		paletteOpen = true;
		query = '';
		await Promise.resolve();
		paletteInput?.focus();
	}

	const sections = $derived(remoteMode ? [
		{ label: null, items: [{ route: 'dashboard', label: 'Dashboard', icon: 'dashboard' }] },
		{ label: 'Cluster', items: [
			{ route: 'devices', label: 'GPUs', icon: 'devices' },
			{ route: 'topology', label: 'Topology', icon: 'topology' },
			{ route: 'monitor', label: 'Monitor', icon: 'monitor' },
		]},
		{ label: 'Jobs', items: [
			{ route: 'jobs', label: 'Jobs', icon: 'jobs' },
			{ route: 'submit', label: 'Run Job', icon: 'submit' },
		]},
		{ label: 'Tools', items: [
			{ route: 'terminal', label: 'Terminal', icon: 'terminal' },
			{ route: 'logs', label: 'Logs', icon: 'logs' },
			{ route: 'docs', label: 'Docs', icon: 'docs' },
		]},
	] : [
		{ label: null, items: [{ route: 'dashboard', label: 'Dashboard', icon: 'dashboard' }] },
		{ label: 'Hardware', items: [
			{ route: 'devices', label: 'GPUs', icon: 'devices' },
			{ route: 'topology', label: 'Topology', icon: 'topology' },
		]},
		{ label: 'Jobs', items: [
			{ route: 'jobs', label: 'Jobs', icon: 'jobs' },
			{ route: 'submit', label: 'Run Job', icon: 'submit' },
		]},
		{ label: 'System', items: [
			{ route: 'logs', label: 'Logs', icon: 'logs' },
			{ route: 'docs', label: 'Docs', icon: 'docs' },
		]},
	]);

	const allItems = $derived(sections.flatMap((s) => s.items));
	const filtered = $derived(
		query.trim()
			? allItems.filter((i) => i.label.toLowerCase().includes(query.trim().toLowerCase()))
			: allItems
	);
</script>

{#snippet icon(name, size = 16, stroke = 1.75)}
	<svg width={size} height={size} viewBox="0 0 24 24" fill={name === 'submit' ? 'currentColor' : 'none'}
		stroke="currentColor" stroke-width={stroke} stroke-linecap="round" stroke-linejoin="round">
		{@html icons[name]}
	</svg>
{/snippet}

<div class="flex h-screen overflow-hidden" style="background: var(--bg-primary);">
	<!-- Sidebar -->
	<aside
		class="shrink-0 overflow-hidden transition-[width] duration-200 ease-out"
		style="width: {collapsed ? '0px' : 'var(--sidebar-width)'}; background: var(--bg-sidebar); border-right: 1px solid var(--border-subtle);"
	>
		<div class="flex flex-col h-full" style="width: var(--sidebar-width);">
			<!-- Brand + collapse -->
			<div class="flex items-center justify-between pl-4 pr-2.5 h-[var(--topbar-height)]">
				<div class="flex items-center gap-2">
					<img src={appicon} alt="gpusched" class="w-5 h-5 rounded-[5px]" />
					<span class="text-[13.5px] font-semibold tracking-tight" style="color: var(--text-primary);">gpusched</span>
				</div>
				<button onclick={toggleSidebar} title="Collapse sidebar (⌘B)"
					class="grid place-items-center w-7 h-7 rounded-md cursor-pointer transition-colors"
					style="color: var(--text-tertiary);"
					onmouseenter={(e) => { e.currentTarget.style.background = 'var(--hover-bg)'; e.currentTarget.style.color = 'var(--text-primary)'; }}
					onmouseleave={(e) => { e.currentTarget.style.background = 'transparent'; e.currentTarget.style.color = 'var(--text-tertiary)'; }}>
					{@render icon('panel', 16)}
				</button>
			</div>

			<!-- Primary actions -->
			<div class="px-2.5 pt-1 pb-2 space-y-0.5">
				<button onclick={() => navigate('submit')}
					class="flex items-center gap-2.5 w-full px-2.5 py-2 rounded-lg text-[13px] font-medium cursor-pointer transition-colors"
					style="color: var(--text-primary);"
					onmouseenter={(e) => e.currentTarget.style.background = 'var(--hover-bg)'}
					onmouseleave={(e) => e.currentTarget.style.background = 'transparent'}>
					<span style="color: var(--accent);">{@render icon('submit', 15)}</span>
					Run Job
				</button>
				<button onclick={openPalette}
					class="flex items-center gap-2.5 w-full px-2.5 py-2 rounded-lg text-[13px] font-medium cursor-pointer transition-colors"
					style="color: var(--text-secondary);"
					onmouseenter={(e) => e.currentTarget.style.background = 'var(--hover-bg)'}
					onmouseleave={(e) => e.currentTarget.style.background = 'transparent'}>
					{@render icon('search', 15)}
					<span class="flex-1 text-left">Search</span>
					<span class="text-[10.5px] font-mono px-1.5 py-0.5 rounded" style="background: var(--bg-tertiary); color: var(--text-tertiary);">⌘K</span>
				</button>
			</div>

			<!-- Nav -->
			<nav class="flex-1 px-2.5 py-1 overflow-y-auto">
				{#each sections as section}
					{#if section.label}
						<div class="text-[10.5px] font-semibold uppercase tracking-[0.08em] px-2.5 pt-4 pb-1" style="color: var(--text-muted);">
							{section.label}
						</div>
					{/if}
					{#each section.items as item (item.route)}
						{@const active = page === item.route}
						<button
							onclick={() => navigate(item.route)}
							class="flex items-center gap-2.5 w-full text-left px-2.5 py-1.5 rounded-lg text-[13px] font-medium transition-colors cursor-pointer mb-0.5"
							style="color: {active ? 'var(--text-primary)' : 'var(--text-tertiary)'}; background: {active ? 'var(--active-bg)' : 'transparent'};"
							onmouseenter={(e) => { if (!active) { e.currentTarget.style.color = 'var(--text-primary)'; e.currentTarget.style.background = 'var(--hover-bg)'; }}}
							onmouseleave={(e) => { if (!active) { e.currentTarget.style.color = 'var(--text-tertiary)'; e.currentTarget.style.background = 'transparent'; }}}
						>
							<span style="color: {active ? 'var(--accent)' : 'inherit'};">{@render icon(item.icon, 16)}</span>
							{item.label}
						</button>
					{/each}
				{/each}
			</nav>

			<!-- Bottom: user / settings chip -->
			<div class="p-2.5" style="border-top: 1px solid var(--border-subtle);">
				<button
					onclick={() => navigate('settings')}
					class="flex items-center gap-2.5 w-full px-2 py-1.5 rounded-lg cursor-pointer transition-colors"
					style="background: {page === 'settings' ? 'var(--active-bg)' : 'transparent'};"
					onmouseenter={(e) => { if (page !== 'settings') e.currentTarget.style.background = 'var(--hover-bg)'; }}
					onmouseleave={(e) => { if (page !== 'settings') e.currentTarget.style.background = 'transparent'; }}
				>
					<div class="w-7 h-7 rounded-full grid place-items-center text-[12px] font-semibold shrink-0" style="background: var(--accent); color: var(--accent-text);">L</div>
					<div class="flex-1 min-w-0 text-left">
						<div class="text-[12.5px] font-semibold truncate" style="color: var(--text-primary);">Local Node</div>
						<div class="text-[11px] truncate" style="color: var(--text-tertiary);">{remoteMode ? 'Remote · cluster' : 'Inplace · single'}</div>
					</div>
					<span style="color: var(--text-muted);">{@render icon('settings', 15)}</span>
				</button>
			</div>
		</div>
	</aside>

	<!-- Main column -->
	<div class="flex-1 flex flex-col min-w-0">
		<!-- Top bar -->
		<header class="flex items-center gap-2 px-3 shrink-0 h-[var(--topbar-height)]" style="border-bottom: 1px solid var(--border-subtle); background: var(--bg-primary);">
			{#if collapsed}
				<button onclick={toggleSidebar} title="Open sidebar (⌘B)"
					class="grid place-items-center w-7 h-7 rounded-md cursor-pointer transition-colors"
					style="color: var(--text-tertiary);"
					onmouseenter={(e) => { e.currentTarget.style.background = 'var(--hover-bg)'; e.currentTarget.style.color = 'var(--text-primary)'; }}
					onmouseleave={(e) => { e.currentTarget.style.background = 'transparent'; e.currentTarget.style.color = 'var(--text-tertiary)'; }}>
					{@render icon('panel', 16)}
				</button>
			{/if}
			<div class="flex items-center gap-1.5 text-[13px] font-medium" style="color: var(--text-secondary);">
				<span style="color: var(--text-tertiary);">gpusched</span>
				<span style="color: var(--text-muted);">/</span>
				<span style="color: var(--text-primary);">{titles[page]}</span>
			</div>

			<div class="flex-1"></div>

			<button onclick={openPalette} title="Search (⌘K)"
				class="grid place-items-center w-7 h-7 rounded-md cursor-pointer transition-colors"
				style="color: var(--text-tertiary);"
				onmouseenter={(e) => { e.currentTarget.style.background = 'var(--hover-bg)'; e.currentTarget.style.color = 'var(--text-primary)'; }}
				onmouseleave={(e) => { e.currentTarget.style.background = 'transparent'; e.currentTarget.style.color = 'var(--text-tertiary)'; }}>
				{@render icon('search', 16)}
			</button>
			<button onclick={toggleFullscreen} title="Toggle fullscreen (⌃⌘F)"
				class="grid place-items-center w-7 h-7 rounded-md cursor-pointer transition-colors"
				style="color: var(--text-tertiary);"
				onmouseenter={(e) => { e.currentTarget.style.background = 'var(--hover-bg)'; e.currentTarget.style.color = 'var(--text-primary)'; }}
				onmouseleave={(e) => { e.currentTarget.style.background = 'transparent'; e.currentTarget.style.color = 'var(--text-tertiary)'; }}>
				{@render icon(isFullscreen ? 'shrink' : 'expand', 16)}
			</button>
			<button onclick={toggleTheme} title="Toggle theme"
				class="grid place-items-center w-7 h-7 rounded-md cursor-pointer transition-colors"
				style="color: var(--text-tertiary);"
				onmouseenter={(e) => { e.currentTarget.style.background = 'var(--hover-bg)'; e.currentTarget.style.color = 'var(--text-primary)'; }}
				onmouseleave={(e) => { e.currentTarget.style.background = 'transparent'; e.currentTarget.style.color = 'var(--text-tertiary)'; }}>
				{@render icon(dark ? 'sun' : 'moon', 16)}
			</button>
		</header>

		<!-- Page -->
		<main class="flex-1 overflow-y-auto" style="background: var(--bg-primary);">
			{#if page === 'dashboard'}
				<Dashboard {remoteMode} {onModeChange} />
			{:else if page === 'devices'}
				<Devices {remoteMode} />
			{:else if page === 'topology'}
				<Topology {remoteMode} />
			{:else if page === 'jobs'}
				<Jobs {remoteMode} />
			{:else if page === 'submit'}
				<Submit {remoteMode} />
			{:else if page === 'monitor'}
				<Monitor {remoteMode} />
			{:else if page === 'logs'}
				<Logs />
			{:else if page === 'docs'}
				<Docs {remoteMode} />
			{:else if page === 'settings'}
				<Settings {dark} {toggleTheme} {remoteMode} />
			{/if}

			<!-- Terminal stays mounted across navigation so the SSH PTY + scrollback persist -->
			<div class="h-full" style="display: {page === 'terminal' ? 'block' : 'none'};">
				<Terminal active={page === 'terminal'} />
			</div>
		</main>
	</div>
</div>

<!-- Command palette -->
{#if paletteOpen}
	<div class="fixed inset-0 z-50 flex items-start justify-center pt-[18vh] px-4"
		style="background: rgba(0,0,0,0.35);"
		onclick={() => (paletteOpen = false)}
		onkeydown={() => {}}
		role="presentation">
		<div class="w-full max-w-[520px] rounded-xl overflow-hidden shadow-2xl"
			style="background: var(--bg-elevated); border: 1px solid var(--border);"
			onclick={(e) => e.stopPropagation()}
			onkeydown={() => {}}
			role="dialog" tabindex="-1">
			<div class="flex items-center gap-2.5 px-3.5 h-11" style="border-bottom: 1px solid var(--border-subtle);">
				<span style="color: var(--text-tertiary);">{@render icon('search', 16)}</span>
				<input
					bind:this={paletteInput}
					bind:value={query}
					onkeydown={(e) => { if (e.key === 'Enter' && filtered[0]) navigate(filtered[0].route); }}
					placeholder="Jump to a page…"
					class="flex-1 bg-transparent outline-none text-[14px]"
					style="color: var(--text-primary);"
				/>
				<span class="text-[10.5px] font-mono px-1.5 py-0.5 rounded" style="background: var(--bg-tertiary); color: var(--text-tertiary);">esc</span>
			</div>
			<div class="max-h-[320px] overflow-y-auto p-1.5">
				{#each filtered as item (item.route)}
					<button onclick={() => navigate(item.route)}
						class="flex items-center gap-3 w-full px-2.5 py-2 rounded-lg text-[13px] font-medium text-left cursor-pointer transition-colors"
						style="color: var(--text-secondary);"
						onmouseenter={(e) => { e.currentTarget.style.background = 'var(--hover-bg)'; e.currentTarget.style.color = 'var(--text-primary)'; }}
						onmouseleave={(e) => { e.currentTarget.style.background = 'transparent'; e.currentTarget.style.color = 'var(--text-secondary)'; }}>
						<span style="color: var(--text-tertiary);">{@render icon(item.icon, 16)}</span>
						{item.label}
					</button>
				{:else}
					<div class="px-3 py-6 text-center text-[13px]" style="color: var(--text-muted);">No matches</div>
				{/each}
			</div>
		</div>
	</div>
{/if}
