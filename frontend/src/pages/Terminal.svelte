<script>
	import { onMount, onDestroy } from 'svelte';
	import { Terminal } from '@xterm/xterm';
	import { FitAddon } from '@xterm/addon-fit';
	import '@xterm/xterm/css/xterm.css';
	import { GetNodes, StartTerminalSession, WriteTerminalInput, ResizeTerminal, StopTerminalSession } from '../lib/api.js';
	import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime.js';

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
		requestAnimationFrame(() => {
			fitAddon.fit();
		});

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

		// Watch container resize
		resizeObserver = new ResizeObserver(() => {
			if (fitAddon) {
				try { fitAddon.fit(); } catch {}
			}
		});
		resizeObserver.observe(termContainer);
	}

	onMount(async () => {
		await loadNodes();
		// Small delay to ensure container is rendered
		requestAnimationFrame(() => {
			if (termContainer) {
				initTerminal();
				if (selectedNode) {
					connectTerminal(selectedNode);
				}
			}
		});
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
	}
</style>
