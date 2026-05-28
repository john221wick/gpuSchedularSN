<script>
	import { onMount } from 'svelte';
	import { GetRemoteMode } from './lib/api.js';
	import Dashboard from './pages/Dashboard.svelte';
	import Devices from './pages/Devices.svelte';
	import Topology from './pages/Topology.svelte';
	import Jobs from './pages/Jobs.svelte';
	import Submit from './pages/Submit.svelte';
	import Settings from './pages/Settings.svelte';
	import Nodes from './pages/Nodes.svelte';
	import Logs from './pages/Logs.svelte';
	import Terminal from './pages/Terminal.svelte';
	import Docs from './pages/Docs.svelte';

	let page = $state('dashboard');
	let dark = $state(true);
	let remoteMode = $state(false);

	const routes = {
		'': 'dashboard',
		'dashboard': 'dashboard',
		'devices': 'devices',
		'topology': 'topology',
		'jobs': 'jobs',
		'submit': 'submit',
		'settings': 'settings',
		'nodes': 'nodes',
		'terminal': 'terminal',
		'logs': 'logs',
		'docs': 'docs'
	};

	function handleHash() {
		const hash = window.location.hash.replace('#/', '').replace('#', '');
		page = routes[hash] || 'dashboard';
	}

	onMount(async () => {
		const saved = localStorage.getItem('theme');
		dark = saved ? saved === 'dark' : true;
		applyTheme();

		try { remoteMode = await GetRemoteMode(); } catch {}

		handleHash();
		window.addEventListener('hashchange', handleHash);
		return () => window.removeEventListener('hashchange', handleHash);
	});

	function onModeChange(mode) {
		remoteMode = mode;
	}

	function applyTheme() {
		document.documentElement.classList.toggle('dark', dark);
		localStorage.setItem('theme', dark ? 'dark' : 'light');
	}

	function toggleTheme() {
		dark = !dark;
		applyTheme();
	}

	function navigate(route) {
		window.location.hash = '#/' + route;
	}

	const sections = $derived(remoteMode ? [
		{ label: null, items: [
			{ route: 'dashboard', label: 'Dashboard' },
		]},
		{ label: 'Cluster', items: [
			{ route: 'devices', label: 'GPUs' },
			{ route: 'topology', label: 'Topology' },
			{ route: 'nodes', label: 'Nodes' },
		]},
		{ label: 'Jobs', items: [
			{ route: 'jobs', label: 'Jobs' },
			{ route: 'submit', label: 'Run Job' },
		]},
		{ label: 'Tools', items: [
			{ route: 'terminal', label: 'Terminal' },
			{ route: 'logs', label: 'Logs' },
			{ route: 'docs', label: 'Docs' },
		]},
	] : [
		{ label: null, items: [
			{ route: 'dashboard', label: 'Dashboard' },
		]},
		{ label: 'Hardware', items: [
			{ route: 'devices', label: 'GPUs' },
			{ route: 'topology', label: 'Topology' },
		]},
		{ label: 'Jobs', items: [
			{ route: 'jobs', label: 'Jobs' },
			{ route: 'submit', label: 'Run Job' },
		]},
		{ label: 'System', items: [
			{ route: 'logs', label: 'Logs' },
			{ route: 'docs', label: 'Docs' },
		]},
	]);
</script>

<div class="flex h-screen overflow-hidden">
	<!-- Sidebar -->
	<aside class="flex flex-col w-[200px] shrink-0" style="background: var(--bg-sidebar); border-right: 1px solid var(--border);">
		<div class="px-5 h-[56px] flex items-center">
			<span class="text-[13px] font-semibold tracking-tight" style="color: var(--text-primary);">gpusched</span>
		</div>

		<nav class="flex-1 px-3 py-1 overflow-y-auto">
			{#each sections as section, si}
				{#if section.label}
					<div class="text-[10px] font-semibold uppercase tracking-widest px-3 pt-4 pb-1.5" style="color: var(--text-muted);">
						{section.label}
					</div>
				{/if}
				{#each section.items as item (item.route)}
					<button
						onclick={() => navigate(item.route)}
						class="block w-full text-left px-3 py-1.5 rounded-md text-[13px] font-medium transition-colors cursor-pointer"
						style="color: {page === item.route ? 'var(--text-primary)' : 'var(--text-tertiary)'}; background: {page === item.route ? 'var(--hover-bg)' : 'transparent'};"
						onmouseenter={(e) => { if (page !== item.route) { e.currentTarget.style.color = 'var(--text-primary)'; e.currentTarget.style.background = 'var(--hover-bg)'; }}}
						onmouseleave={(e) => { if (page !== item.route) { e.currentTarget.style.color = 'var(--text-tertiary)'; e.currentTarget.style.background = 'transparent'; }}}
					>
						{item.label}
					</button>
				{/each}
			{/each}
		</nav>

		<div class="px-4 py-3 space-y-3" style="border-top: 1px solid var(--border);">
			<button
				onclick={() => navigate('settings')}
				class="flex items-center gap-2 text-[12px] font-medium cursor-pointer w-full rounded-md px-2 py-1.5 transition-colors"
				style="color: {page === 'settings' ? 'var(--text-primary)' : 'var(--text-secondary)'}; background: {page === 'settings' ? 'var(--hover-bg)' : 'var(--bg-tertiary)'};"
			>
				<svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
					<path stroke-linecap="round" stroke-linejoin="round" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
					<path stroke-linecap="round" stroke-linejoin="round" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
				</svg>
				Settings
			</button>
			<div class="text-[11px] font-mono px-2" style="color: var(--text-muted);">v0.1.0</div>
		</div>
	</aside>

	<!-- Main -->
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
		{:else if page === 'nodes'}
			<Nodes />
		{:else if page === 'terminal'}
			<Terminal />
		{:else if page === 'logs'}
			<Logs />
		{:else if page === 'docs'}
			<Docs {remoteMode} />
		{:else if page === 'settings'}
			<Settings {dark} {toggleTheme} {remoteMode} />
		{/if}
	</main>
</div>
