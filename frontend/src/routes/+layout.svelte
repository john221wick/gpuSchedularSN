<script>
	import './layout.css';
	let { children } = $props();

	const nav = [
		{ href: '/', label: 'Dashboard', icon: 'grid' },
		{ href: '/devices', label: 'GPUs', icon: 'cpu' },
		{ href: '/topology', label: 'Topology', icon: 'network' },
		{ href: '/jobs', label: 'Jobs', icon: 'layers' },
		{ href: '/submit', label: 'Run Job', icon: 'play' }
	];

	const icons = {
		grid: 'M4 5a1 1 0 011-1h4a1 1 0 011 1v4a1 1 0 01-1 1H5a1 1 0 01-1-1V5zm10 0a1 1 0 011-1h4a1 1 0 011 1v4a1 1 0 01-1 1h-4a1 1 0 01-1-1V5zM4 15a1 1 0 011-1h4a1 1 0 011 1v4a1 1 0 01-1 1H5a1 1 0 01-1-1v-4zm10 0a1 1 0 011-1h4a1 1 0 011 1v4a1 1 0 01-1 1h-4a1 1 0 01-1-1v-4z',
		cpu: 'M9 3v2m6-2v2M9 19v2m6-2v2M3 9h2m-2 6h2m14-6h2m-2 6h2M7 7h10v10H7V7z',
		network: 'M12 4v1m6 11h2m-6 0h-2v4m0-11v3m0 0h.01M12 12h4.01M16 20h2M4 16h2m10 0h2M4 12h2m0-4h12a2 2 0 012 2v4a2 2 0 01-2 2H6a2 2 0 01-2-2v-4a2 2 0 012-2z',
		layers: 'M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10',
		play: 'M14.752 11.168l-3.197-2.132A1 1 0 0010 9.87v4.263a1 1 0 001.555.832l3.197-2.132a1 1 0 000-1.664z'
	};
</script>

<div class="flex h-screen overflow-hidden">
	<!-- Sidebar -->
	<aside class="flex flex-col w-[220px] bg-[#16162a] border-r border-[#2a2a4a] shrink-0">
		<!-- Logo -->
		<div class="flex items-center gap-2.5 px-5 h-[56px] border-b border-[#2a2a4a]">
			<div class="w-7 h-7 rounded-lg bg-[#4f6ef7] flex items-center justify-center">
				<svg class="w-4 h-4 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
					<path d={icons.cpu} />
				</svg>
			</div>
			<span class="text-[15px] font-semibold text-white tracking-tight">GPU Scheduler</span>
		</div>

		<!-- Nav -->
		<nav class="flex-1 py-3 px-3 space-y-0.5">
			{#each nav as item}
				<a
					href={item.href}
					class="flex items-center gap-3 px-3 py-2 rounded-md text-[13px] font-medium transition-colors
						text-[#8888aa] hover:text-white hover:bg-[#1e1e3a]
						aria-[current=page]:text-white aria-[current=page]:bg-[#1e1e3a]"
				>
					<svg class="w-[18px] h-[18px] shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
						<path stroke-linecap="round" stroke-linejoin="round" d={icons[item.icon]} />
					</svg>
					{item.label}
				</a>
			{/each}
		</nav>

		<!-- Footer -->
		<div class="px-5 py-3 border-t border-[#2a2a4a] text-[11px] text-[#555577]">
			gpusched v0.1.0
		</div>
	</aside>

	<!-- Main -->
	<main class="flex-1 overflow-y-auto bg-[#1a1a2e]">
		{@render children()}
	</main>
</div>
