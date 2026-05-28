<script>
	import { onMount, tick } from 'svelte';
	import { GetNodes, RunTerminalCommand } from '../lib/api.js';

	let nodes = $state([]);
	let selectedNode = $state('');
	let command = $state('');
	let history = $state([]);
	let running = $state(false);
	let inputEl;
	let cmdHistory = $state([]);
	let cmdHistoryIdx = $state(-1);
	let cwd = $state('~');

	async function loadNodes() {
		try {
			const n = await GetNodes();
			nodes = (n || []).filter(nd => nd.status === 'connected');
			if (nodes.length > 0 && !selectedNode) {
				selectedNode = nodes[0].id;
			}
		} catch {}
	}

	function getPrompt() {
		const node = nodes.find(n => n.id === selectedNode);
		const host = node?.name || 'remote';
		return `root@${host}`;
	}

	async function runCommand() {
		const cmd = command.trim();
		if (!cmd || !selectedNode || running) return;

		cmdHistory = [...cmdHistory, cmd];
		cmdHistoryIdx = -1;

		const prompt = getPrompt();
		history = [...history, { type: 'cmd', text: cmd, prompt, cwd }];
		command = '';

		// Handle client-side commands
		if (cmd === 'clear') {
			history = [];
			return;
		}

		running = true;
		await tick();
		scrollBottom();

		try {
			// Wrap command: cd to cwd first, set TERM, then run, then print cwd
			const wrapped = `cd ${cwd} 2>/dev/null; TERM=xterm ${cmd}; echo "___CWD___$(pwd)"`;
			const result = await RunTerminalCommand(selectedNode, wrapped);

			let output = result.output || '';
			// Extract new cwd from output
			const cwdMatch = output.match(/___CWD___(.*)/);
			if (cwdMatch) {
				cwd = cwdMatch[1].trim();
				output = output.replace(/___CWD___.*\n?/, '');
			}

			if (output.trim()) {
				history = [...history, { type: 'out', text: output.trimEnd() }];
			}
			if (result.error) {
				// Filter out cd errors since we handle cwd ourselves
				const errText = result.error.replace(/Process exited with status \d+/, '').trim();
				if (errText) {
					history = [...history, { type: 'err', text: errText }];
				}
			}
		} catch (e) {
			history = [...history, { type: 'err', text: e?.message || String(e) }];
		} finally {
			running = false;
			await tick();
			scrollBottom();
			inputEl?.focus();
		}
	}

	function scrollBottom() {
		const el = document.getElementById('term-scroll');
		if (el) el.scrollTop = el.scrollHeight;
	}

	function handleKeydown(e) {
		if (e.key === 'Enter' && !running) {
			e.preventDefault();
			runCommand();
		} else if (e.key === 'ArrowUp') {
			e.preventDefault();
			if (cmdHistory.length > 0) {
				if (cmdHistoryIdx === -1) cmdHistoryIdx = cmdHistory.length - 1;
				else if (cmdHistoryIdx > 0) cmdHistoryIdx--;
				command = cmdHistory[cmdHistoryIdx];
			}
		} else if (e.key === 'ArrowDown') {
			e.preventDefault();
			if (cmdHistoryIdx >= 0) {
				cmdHistoryIdx++;
				if (cmdHistoryIdx >= cmdHistory.length) {
					cmdHistoryIdx = -1;
					command = '';
				} else {
					command = cmdHistory[cmdHistoryIdx];
				}
			}
		} else if (e.key === 'l' && e.ctrlKey) {
			e.preventDefault();
			history = [];
		}
	}

	function focusInput() {
		inputEl?.focus();
	}

	onMount(() => {
		loadNodes();
		setTimeout(() => inputEl?.focus(), 100);
	});
</script>

<div class="h-full flex flex-col">
	<!-- Title bar -->
	<div class="flex items-center justify-between px-5 py-3 shrink-0" style="background: #1a1a1a; border-bottom: 1px solid #333;">
		<div class="flex items-center gap-3">
			<div class="flex gap-1.5">
				<div class="w-3 h-3 rounded-full" style="background: #ff5f57;"></div>
				<div class="w-3 h-3 rounded-full" style="background: #febc2e;"></div>
				<div class="w-3 h-3 rounded-full" style="background: #28c840;"></div>
			</div>
			{#if nodes.length > 1}
				<select
					bind:value={selectedNode}
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
		</div>
		<span class="text-[11px] font-[JetBrains_Mono,monospace]" style="color: #555;">
			{history.filter(h => h.type === 'cmd').length} commands
		</span>
	</div>

	<!-- Terminal body -->
	<!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
	<div
		id="term-scroll"
		class="flex-1 overflow-auto p-4 font-[JetBrains_Mono,monospace] text-[13px] leading-[1.7] min-h-0"
		style="background: #0c0c0c; color: #cccccc;"
		onclick={focusInput}
	>
		{#if nodes.length === 0}
			<div style="color: #555;">
				<pre>No nodes connected. Go to Nodes page to connect a remote server.</pre>
			</div>
		{:else}
			{#if history.length === 0}
				<div style="color: #555;">
					<pre style="color: #28c840;">Welcome to gpusched remote terminal</pre>
					<pre style="color: #555;">Type commands below. Arrow up/down for history. Ctrl+L to clear.</pre>
					<pre style="color: #555;"> </pre>
				</div>
			{/if}

			{#each history as entry}
				{#if entry.type === 'cmd'}
					<div>
						<span style="color: #28c840;">{entry.prompt}</span><span style="color: #555;">:</span><span style="color: #3b82f6;">{entry.cwd || '~'}</span><span style="color: #555;">$ </span><span style="color: #e2e8f0;">{entry.text}</span>
					</div>
				{:else if entry.type === 'err'}
					<pre class="whitespace-pre-wrap" style="color: #ef4444; margin: 0;">{entry.text}</pre>
				{:else}
					<pre class="whitespace-pre-wrap" style="color: #b0b0b0; margin: 0;">{entry.text}</pre>
				{/if}
			{/each}

			{#if running}
				<div class="animate-pulse" style="color: #555;">running...</div>
			{/if}

			<!-- Input line -->
			{#if !running && nodes.length > 0}
				<div class="flex items-center">
					<span style="color: #28c840;">{getPrompt()}</span><span style="color: #555;">:</span><span style="color: #3b82f6;">{cwd}</span><span style="color: #555;">$ </span>
					<input
						bind:this={inputEl}
						type="text"
						bind:value={command}
						onkeydown={handleKeydown}
						autocomplete="off" autocorrect="off" autocapitalize="off" spellcheck="false" data-form-type="other"
						class="flex-1 bg-transparent text-[13px] font-[JetBrains_Mono,monospace]"
						style="color: #e2e8f0; caret-color: #28c840; outline: none; border: none; box-shadow: none; -webkit-appearance: none;"
					/>
				</div>
			{/if}
		{/if}
	</div>
</div>
