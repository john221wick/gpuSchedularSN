<script>
	import { onDestroy } from 'svelte';
	import { Terminal } from '@xterm/xterm';
	import { FitAddon } from '@xterm/addon-fit';
	import '@xterm/xterm/css/xterm.css';
	import { GetNodes, StartTerminalSession, WriteTerminalInput, ResizeTerminal, StopTerminalSession } from '../lib/api.js';
	import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime.js';

	// active = true when the Terminal route is the visible page. The component
	// stays mounted across navigation so the PTY + scrollback survive.
	let { active = false } = $props();
	let booted = false;

	let nodes = $state([]);
	let selectedNode = $state('');
	let sessionID = $state('');
	let connected = $state(false);
	let connecting = $state(false);
	let error = $state('');
	let disconnected = $state(false);

	let termContainer;
	let term;
	let fitAddon;
	let resizeObserver;

	async function loadNodes() {
		try {
			const n = await GetNodes();
			nodes = (n || []).filter(nd => nd.status === 'connected');
			if (nodes.length > 0 && !selectedNode) {
				selectedNode = nodes[0].id;
			}
		} catch {}
	}

	async function connectTerminal(nodeID) {
		if (!nodeID) return;
		await cleanupSession();

		connecting = true;
		error = '';
		disconnected = false;

		try {
			const cols = term ? term.cols : 80;
			const rows = term ? term.rows : 24;
			sessionID = await StartTerminalSession(nodeID, cols, rows);
			connected = true;

			// Listen for output
			EventsOn('terminal:output:' + sessionID, (data) => {
				if (term) {
					const decoded = atob(data);
					term.write(decoded);
				}
			});

			// Listen for exit
			EventsOn('terminal:exit:' + sessionID, () => {
				disconnected = true;
				connected = false;
			});

			// Focus terminal
			if (term) term.focus();
		} catch (e) {
			error = e?.message || String(e);
			connected = false;
		} finally {
			connecting = false;
		}
	}

	async function cleanupSession() {
		if (sessionID) {
			EventsOff('terminal:output:' + sessionID);
			EventsOff('terminal:exit:' + sessionID);
			try { await StopTerminalSession(sessionID); } catch {}
			sessionID = '';
		}
		connected = false;
		disconnected = false;
	}

	async function handleNodeChange() {
		if (term) {
			term.clear();
			term.reset();
		}
		if (selectedNode) {
			await connectTerminal(selectedNode);
		}
	}

	let fitScheduled = false;
	// Fit via proposeDimensions + diff so we only resize when dims actually
	// change — prevents the FitAddon ↔ scrollbar feedback loop.
	function doFit() {
		if (!active || !fitAddon || !term) return;
		try {
			const dims = fitAddon.proposeDimensions();
			if (dims && Number.isFinite(dims.cols) && Number.isFinite(dims.rows) &&
				dims.cols > 0 && dims.rows > 0 &&
				(dims.cols !== term.cols || dims.rows !== term.rows)) {
				term.resize(dims.cols, dims.rows);
			}
		} catch {}
	}
	// rAF-debounced so a burst of ResizeObserver callbacks coalesces into one fit.
	function scheduleFit() {
		if (fitScheduled) return;
		fitScheduled = true;
		requestAnimationFrame(() => { fitScheduled = false; doFit(); });
	}

	function initTerminal() {
		term = new Terminal({
			cursorBlink: true,
			cursorStyle: 'bar',
			fontSize: 13,
			fontFamily: 'JetBrains Mono, Menlo, Monaco, Consolas, monospace',
			theme: {
				background: '#0c0c0c',
				foreground: '#cccccc',
				cursor: '#28c840',
				cursorAccent: '#0c0c0c',
				selectionBackground: 'rgba(255, 255, 255, 0.15)',
				black: '#000000',
				red: '#ef4444',
				green: '#22c55e',
				yellow: '#eab308',
				blue: '#3b82f6',
				magenta: '#a855f7',
				cyan: '#06b6d4',
				white: '#d4d4d4',
				brightBlack: '#737373',
				brightRed: '#f87171',
				brightGreen: '#4ade80',
				brightYellow: '#facc15',
				brightBlue: '#60a5fa',
				brightMagenta: '#c084fc',
				brightCyan: '#22d3ee',
				brightWhite: '#ffffff',
			},
			allowProposedApi: true,
		});

		fitAddon = new FitAddon();
		term.loadAddon(fitAddon);
		term.open(termContainer);

		// Fit after a tick so container has dimensions
		scheduleFit();

		// Send input to Go
		term.onData((data) => {
			if (sessionID && connected) {
				WriteTerminalInput(sessionID, btoa(data));
			}
		});

		// Handle resize
		term.onResize(({ cols, rows }) => {
			if (sessionID && connected) {
				ResizeTerminal(sessionID, cols, rows);
			}
		});

		// Watch container resize — debounced + diff-guarded (see scheduleFit/doFit)
		resizeObserver = new ResizeObserver(() => scheduleFit());
		resizeObserver.observe(termContainer);
	}

	async function boot() {
		await loadNodes();
		requestAnimationFrame(() => {
			if (termContainer && !term) initTerminal();
			doFit(); // size before connecting so the PTY opens at the right dimensions
			requestAnimationFrame(() => {
				if (selectedNode && !sessionID && !connecting) {
					connectTerminal(selectedNode);
				}
				term?.focus();
			});
		});
	}

	function onShow() {
		// Session persists across nav — just refit/focus and re-poll node status.
		scheduleFit();
		term?.focus();
		loadNodes();
	}

	// Lazy-init on first open; on later opens reuse the live session.
	$effect(() => {
		if (!active) return;
		if (!booted) { booted = true; boot(); }
		else onShow();
	});

	onDestroy(() => {
		cleanupSession();
		if (resizeObserver) resizeObserver.disconnect();
		if (term) term.dispose();
	});
</script>

<div class="h-full flex flex-col">
	<!-- Title bar -->
	<div class="flex items-center justify-between px-4 py-2.5 shrink-0" style="background: #1a1a1a; border-bottom: 1px solid #333;">
		<div class="flex items-center gap-3">
			<div class="flex gap-1.5">
				<div class="w-3 h-3 rounded-full" style="background: #ff5f57;"></div>
				<div class="w-3 h-3 rounded-full" style="background: #febc2e;"></div>
				<div class="w-3 h-3 rounded-full" style="background: #28c840;"></div>
			</div>
			{#if nodes.length > 1}
				<select
					bind:value={selectedNode}
					onchange={handleNodeChange}
					class="text-[12px] font-[JetBrains_Mono,monospace] outline-none cursor-pointer rounded px-2 py-0.5"
					style="background: #2a2a2a; color: #999; border: 1px solid #444;"
				>
					{#each nodes as node (node.id)}
						<option value={node.id}>{node.name}</option>
					{/each}
				</select>
			{:else if nodes.length === 1}
				<span class="text-[12px] font-[JetBrains_Mono,monospace]" style="color: #999;">
					{nodes[0].name}
				</span>
			{:else}
				<span class="text-[12px] font-[JetBrains_Mono,monospace]" style="color: #666;">
					not connected
				</span>
			{/if}
			{#if connecting}
				<span class="text-[11px] animate-pulse" style="color: #eab308;">connecting...</span>
			{:else if connected}
				<span class="text-[11px]" style="color: #22c55e;">connected</span>
			{:else if disconnected}
				<span class="text-[11px]" style="color: #ef4444;">disconnected</span>
			{/if}
		</div>
		<div class="flex items-center gap-2">
			{#if disconnected && selectedNode}
				<button
					onclick={() => connectTerminal(selectedNode)}
					class="text-[11px] px-2 py-0.5 rounded font-medium cursor-pointer"
					style="background: #2a2a2a; color: #22c55e; border: 1px solid #444;"
				>
					Reconnect
				</button>
			{/if}
		</div>
	</div>

	<!-- Error bar -->
	{#if error}
		<div class="px-4 py-2 text-[12px]" style="background: rgba(239,68,68,0.1); color: #ef4444;">
			{error}
		</div>
	{/if}

	<!-- Terminal -->
	<div
		bind:this={termContainer}
		class="flex-1 min-h-0"
		style="background: #0c0c0c;"
	></div>
</div>

<style>
	:global(.xterm) {
		padding: 8px;
		height: 100%;
	}
	:global(.xterm-viewport) {
		overflow-y: auto !important;
		scrollbar-gutter: stable;
	}
	/* Keep the scrollbar from changing content width (avoids fit/resize oscillation) */
	:global(.xterm-viewport)::-webkit-scrollbar {
		width: 10px;
	}
	:global(.xterm-viewport)::-webkit-scrollbar-thumb {
		background: #333;
		border-radius: 5px;
	}
</style>
