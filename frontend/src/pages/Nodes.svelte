<script>
	import { onMount } from 'svelte';
	import { GetNodes, GetSavedNodes, ConnectNode, DisconnectNode, ReconnectNode, RemoveNode, SetNodePaths, SyncFilesToNode } from '../lib/api.js';

	let nodes = $state([]);
	let savedNodes = $state([]);
	let showConnect = $state(false);
	let reconnecting = $state({});
	let sshCommand = $state('');
	let keyPath = $state('');
	let mockMode = $state(false);
	let connecting = $state(false);
	let connectError = $state('');
	let connectSuccess = $state('');

	// Per-node config state
	let editingNode = $state(null);
	let editRemoteDir = $state('');

	// Disconnect confirm state
	let confirmDisconnect = $state(null); // nodeId to confirm

	// File sync state
	let syncNodeId = $state(null);
	let syncLocalPath = $state('');
	let syncing = $state(false);
	let syncError = $state('');
	let syncedFiles = $state([]); // [{localPath, remotePath, nodeId}]

	let interval;

	async function refresh() {
		try {
			nodes = await GetNodes() || [];
			savedNodes = await GetSavedNodes() || [];
		} catch (e) {
			console.error('Nodes refresh failed:', e);
		}
	}

	async function handleConnect() {
		if (!sshCommand.trim()) return;
		connecting = true;
		connectError = '';
		connectSuccess = '';

		try {
			const node = await ConnectNode(sshCommand.trim(), keyPath.trim(), mockMode);
			const gpuInfo = node.gpuName ? ` — ${node.gpuName}` : '';
			connectSuccess = `Connected to ${node.name} (${node.numGPUs} GPUs${gpuInfo})`;
			sshCommand = '';
			keyPath = '';
			mockMode = false;
			showConnect = false;
			await refresh();
		} catch (e) {
			connectError = e?.message || String(e);
		} finally {
			connecting = false;
		}
	}

	function requestDisconnect(nodeId) {
		confirmDisconnect = nodeId;
	}

	async function handleReconnect(nodeId) {
		reconnecting = { ...reconnecting, [nodeId]: true };
		try {
			await ReconnectNode(nodeId);
			await refresh();
		} catch (e) {
			console.error('Reconnect failed:', e);
		} finally {
			reconnecting = { ...reconnecting, [nodeId]: false };
		}
	}

	async function handleRemove(nodeId) {
		try {
			await RemoveNode(nodeId);
			await refresh();
		} catch (e) {
			console.error('Remove failed:', e);
		}
	}

	async function doDisconnect() {
		const nodeId = confirmDisconnect;
		confirmDisconnect = null;
		try {
			await DisconnectNode(nodeId);
			syncedFiles = syncedFiles.filter(f => f.nodeId !== nodeId);
			await refresh();
		} catch (e) {
			console.error('Disconnect failed:', e);
		}
	}

	function startEditDest(node) {
		editingNode = node.id;
		editRemoteDir = node.remoteDir || '/root/gpuschedular';
	}

	async function saveDest(nodeId) {
		try {
			await SetNodePaths(nodeId, '', editRemoteDir.trim() || '/root/gpuschedular');
			await refresh();
		} catch (e) {
			console.error('Set paths failed:', e);
		}
		editingNode = null;
	}

	function toggleSyncPanel(nodeId) {
		if (syncNodeId === nodeId) {
			syncNodeId = null;
		} else {
			syncNodeId = nodeId;
			syncLocalPath = '';
			syncError = '';
		}
	}

	async function handleSync(nodeId) {
		if (!syncLocalPath.trim()) return;
		syncing = true;
		syncError = '';

		try {
			const remotePath = await SyncFilesToNode(nodeId, syncLocalPath.trim());
			syncedFiles = [...syncedFiles, {
				localPath: syncLocalPath.trim(),
				remotePath,
				nodeId,
				time: new Date().toLocaleTimeString()
			}];
			syncLocalPath = '';
		} catch (e) {
			syncError = e?.message || String(e);
		} finally {
			syncing = false;
		}
	}

	function handleSyncKeydown(e, nodeId) {
		if (e.key === 'Enter' && !syncing) handleSync(nodeId);
	}

	function handleKeydown(e) {
		if (e.key === 'Enter' && !connecting) handleConnect();
		if (e.key === 'Escape') showConnect = false;
	}

	onMount(() => {
		refresh();
		interval = setInterval(refresh, 5000);
		return () => clearInterval(interval);
	});
</script>

<div class="p-8 space-y-6 max-w-[1000px]">
	<div class="flex items-center justify-between">
		<div>
			<h1 class="text-lg font-semibold" style="color: var(--text-primary);">Nodes</h1>
			<p class="text-[13px] mt-0.5" style="color: var(--text-tertiary);">
				{nodes.length} node{nodes.length !== 1 ? 's' : ''} connected
			</p>
		</div>
		<button
			onclick={() => { showConnect = !showConnect; connectError = ''; connectSuccess = ''; }}
			class="px-3.5 py-1.5 rounded-md text-[13px] font-medium transition-colors cursor-pointer"
			style="background: var(--accent); color: var(--accent-text);"
		>
			Connect Node
		</button>
	</div>

	{#if connectSuccess}
		<div class="rounded-md px-4 py-3 text-[13px]" style="background: rgba(34,197,94,0.1); color: rgb(34,197,94);">
			{connectSuccess}
		</div>
	{/if}

	<!-- Connect dialog -->
	{#if showConnect}
		<div class="rounded-lg p-5 space-y-4" style="background: var(--bg-secondary); border: 1px solid var(--border);">
			<h2 class="text-[14px] font-semibold" style="color: var(--text-primary);">Connect via SSH</h2>
			<p class="text-[12px]" style="color: var(--text-tertiary);">
				Paste the SSH command from your cloud provider, or use an SSH config alias (e.g. "ssh host")
			</p>

			<div class="space-y-3">
				<div>
					<label class="block text-[11px] font-medium uppercase tracking-wider mb-1.5" style="color: var(--text-tertiary);">
						SSH Command
					</label>
					<input
						type="text"
						bind:value={sshCommand}
						onkeydown={handleKeydown}
						placeholder="ssh -p 20544 root@203.0.113.10  or  ssh host"
						autocomplete="off" autocorrect="off" autocapitalize="off" spellcheck="false" data-form-type="other"
						class="w-full px-3 py-2 rounded-md text-[13px] font-[JetBrains_Mono,monospace] outline-none"
						style="background: var(--input-bg); color: var(--text-primary); border: 1px solid var(--border);"
						disabled={connecting}
					/>
				</div>

				<div>
					<label class="block text-[11px] font-medium uppercase tracking-wider mb-1.5" style="color: var(--text-tertiary);">
						SSH Key Path (optional)
					</label>
					<input
						type="text"
						bind:value={keyPath}
						onkeydown={handleKeydown}
						placeholder="~/.ssh/id_ed25519"
						autocomplete="off" autocorrect="off" autocapitalize="off" spellcheck="false" data-form-type="other"
						class="w-full px-3 py-2 rounded-md text-[13px] font-[JetBrains_Mono,monospace] outline-none"
						style="background: var(--input-bg); color: var(--text-primary); border: 1px solid var(--border);"
						disabled={connecting}
					/>
				</div>

				<div class="flex items-center gap-2">
					<input
						type="checkbox"
						id="mockMode"
						bind:checked={mockMode}
						disabled={connecting}
						class="w-3.5 h-3.5 rounded cursor-pointer"
					/>
					<label for="mockMode" class="text-[12px] cursor-pointer" style="color: var(--text-secondary);">
						Mock mode (test without real GPUs)
					</label>
				</div>

				{#if connectError}
					<div class="rounded-md px-3 py-2 text-[12px]" style="background: rgba(239,68,68,0.1); color: rgb(239,68,68);">
						{connectError}
					</div>
				{/if}

				<div class="flex gap-2">
					<button
						onclick={handleConnect}
						disabled={connecting || !sshCommand.trim()}
						class="px-3.5 py-1.5 rounded-md text-[13px] font-medium transition-colors disabled:opacity-50 cursor-pointer"
						style="background: var(--accent); color: var(--accent-text);"
					>
						{connecting ? 'Connecting...' : 'Connect'}
					</button>
					<button
						onclick={() => showConnect = false}
						disabled={connecting}
						class="px-3.5 py-1.5 rounded-md text-[13px] font-medium transition-colors cursor-pointer"
						style="background: var(--bg-tertiary); color: var(--text-secondary);"
					>
						Cancel
					</button>
				</div>
			</div>
		</div>
	{/if}

	<!-- Node list -->
	{#if nodes.length > 0}
		<div class="space-y-3">
			{#each nodes as node (node.id)}
				<div class="rounded-lg p-4" style="background: var(--bg-secondary); border: 1px solid var(--border);">
					<!-- Header row -->
					<div class="flex items-center justify-between">
						<div class="flex items-center gap-3">
							<div class="w-2 h-2 rounded-full" style="background: {node.status === 'connected' ? 'rgb(34,197,94)' : 'rgb(239,68,68)'};"></div>
							<div>
								<span class="text-[13px] font-medium" style="color: var(--text-primary);">{node.name}</span>
								<span class="text-[11px] font-[JetBrains_Mono,monospace] ml-2" style="color: var(--text-muted);">{node.id}</span>
							</div>
							<span class="text-[11px] px-2 py-0.5 rounded font-medium"
								style="background: {node.status === 'connected' ? 'rgba(34,197,94,0.1)' : 'rgba(239,68,68,0.1)'}; color: {node.status === 'connected' ? 'rgb(34,197,94)' : 'rgb(239,68,68)'};">
								{node.status}
							</span>
							<span class="text-[12px] font-[JetBrains_Mono,monospace]" style="color: var(--text-secondary);">{node.numGPUs} GPUs</span>
						</div>
						<div class="flex items-center gap-2">
							<button
								onclick={() => toggleSyncPanel(node.id)}
								class="text-[11px] px-2 py-1 rounded transition-colors font-medium cursor-pointer"
								style="background: {syncNodeId === node.id ? 'var(--accent)' : 'var(--bg-tertiary)'}; color: {syncNodeId === node.id ? 'var(--accent-text)' : 'var(--text-secondary)'};"
							>
								Sync Files
							</button>
							{#if node.id !== 'local'}
								<button
									onclick={() => requestDisconnect(node.id)}
									class="text-[11px] px-2 py-1 rounded bg-red-500/10 text-red-500 hover:bg-red-500/20 transition-colors font-medium cursor-pointer"
								>
									Disconnect
								</button>
							{/if}
						</div>
					</div>

					<!-- GPU / OS info -->
					{#if node.gpuName || node.os}
						<div class="flex flex-wrap gap-x-4 gap-y-0.5 mt-1.5 text-[11px]" style="color: var(--text-muted);">
							{#if node.gpuName}<span>{node.gpuName}</span>{/if}
							{#if node.os}<span>{node.os}</span>{/if}
							{#if node.arch}<span>{node.arch}</span>{/if}
						</div>
					{/if}

					<!-- Remote destination display -->
					<div class="mt-2 flex items-center gap-2 text-[11px] font-[JetBrains_Mono,monospace]">
						<span style="color: var(--text-tertiary);">dest:</span>
						{#if editingNode === node.id}
							<input
								type="text"
								bind:value={editRemoteDir}
								onkeydown={(e) => { if (e.key === 'Enter') saveDest(node.id); if (e.key === 'Escape') editingNode = null; }}
								autocomplete="off" autocorrect="off" autocapitalize="off" spellcheck="false" data-form-type="other"
								class="px-2 py-0.5 rounded text-[11px] font-[JetBrains_Mono,monospace] outline-none w-64"
								style="background: var(--input-bg); color: var(--text-primary); border: 1px solid var(--accent);"
							/>
							<button onclick={() => saveDest(node.id)} class="text-[10px] px-1.5 py-0.5 rounded cursor-pointer" style="background: var(--accent); color: var(--accent-text);">Save</button>
							<button onclick={() => editingNode = null} class="text-[10px] px-1.5 py-0.5 rounded cursor-pointer" style="background: var(--bg-tertiary); color: var(--text-secondary);">Cancel</button>
						{:else}
							<span style="color: var(--text-secondary);">{node.remoteDir || '/root/gpuschedular'}</span>
							<button onclick={() => startEditDest(node)} class="text-[10px] px-1 py-0.5 rounded cursor-pointer" style="color: var(--text-muted);">edit</button>
						{/if}
					</div>

					<!-- Sync Files Panel -->
					{#if syncNodeId === node.id}
						<div class="mt-3 pt-3 space-y-3" style="border-top: 1px solid var(--border);">
							<div>
								<label class="block text-[11px] font-medium uppercase tracking-wider mb-1.5" style="color: var(--text-tertiary);">
									Local Path to Sync
								</label>
								<div class="flex gap-2">
									<input
										type="text"
										bind:value={syncLocalPath}
										onkeydown={(e) => handleSyncKeydown(e, node.id)}
										placeholder="/Users/you/myproject"
										autocomplete="off" autocorrect="off" autocapitalize="off" spellcheck="false" data-form-type="other"
										class="flex-1 px-2.5 py-1.5 rounded-md text-[12px] font-[JetBrains_Mono,monospace] outline-none"
										style="background: var(--input-bg); color: var(--text-primary); border: 1px solid var(--border);"
										disabled={syncing}
									/>
									<button
										onclick={() => handleSync(node.id)}
										disabled={syncing || !syncLocalPath.trim()}
										class="px-3 py-1.5 rounded-md text-[12px] font-medium transition-colors disabled:opacity-50 cursor-pointer"
										style="background: var(--accent); color: var(--accent-text);"
									>
										{syncing ? 'Syncing...' : 'Sync'}
									</button>
								</div>
								<p class="text-[10px] mt-1" style="color: var(--text-muted);">
									Files will be copied to <span class="font-[JetBrains_Mono,monospace]">{node.remoteDir || '/root/gpuschedular'}/</span> on remote
								</p>
							</div>

							{#if syncError}
								<div class="rounded-md px-3 py-2 text-[12px]" style="background: rgba(239,68,68,0.1); color: rgb(239,68,68);">
									{syncError}
								</div>
							{/if}

							<!-- Synced files list -->
							{#if syncedFiles.some(f => f.nodeId === node.id)}
								<div>
									<p class="text-[11px] font-medium mb-1.5" style="color: var(--text-tertiary);">Synced Files</p>
									<div class="space-y-1">
										{#each syncedFiles.filter(f => f.nodeId === node.id) as sf}
											<div class="flex items-center gap-2 px-2.5 py-1.5 rounded text-[11px] font-[JetBrains_Mono,monospace]" style="background: rgba(34,197,94,0.05); border: 1px solid rgba(34,197,94,0.15);">
												<span style="color: rgb(34,197,94);">✓</span>
												<span style="color: var(--text-secondary);">{sf.localPath}</span>
												<span style="color: var(--text-muted);">→</span>
												<span style="color: var(--text-secondary);">{sf.remotePath}</span>
												<span class="ml-auto text-[10px]" style="color: var(--text-muted);">{sf.time}</span>
											</div>
										{/each}
									</div>
								</div>
							{/if}
						</div>
					{/if}
				</div>
			{/each}
		</div>
	{:else if savedNodes.length === 0}
		<div class="rounded-lg flex flex-col items-center justify-center py-12" style="background: var(--bg-secondary); border: 1px solid var(--border);">
			<p class="text-[13px]" style="color: var(--text-tertiary);">No nodes connected</p>
			<p class="text-[12px] mt-1" style="color: var(--text-muted);">Click "Connect Node" to add a remote GPU server</p>
		</div>
	{/if}

	<!-- Saved/disconnected nodes -->
	{#if savedNodes.length > 0}
		<div class="space-y-2 mt-4">
			<h2 class="text-[12px] font-medium uppercase tracking-wider" style="color: var(--text-muted);">Saved Nodes</h2>
			{#each savedNodes as sn (sn.id)}
				<div class="rounded-lg p-4 flex items-center justify-between" style="background: var(--bg-secondary); border: 1px solid var(--border); opacity: 0.7;">
					<div class="flex items-center gap-3">
						<div class="w-2 h-2 rounded-full" style="background: #555;"></div>
						<div>
							<span class="text-[13px] font-medium" style="color: var(--text-secondary);">{sn.id}</span>
							<span class="text-[11px] font-[JetBrains_Mono,monospace] ml-2" style="color: var(--text-muted);">{sn.sshCommand}</span>
							{#if sn.mockMode}
								<span class="text-[10px] ml-2 px-1.5 py-0.5 rounded" style="background: rgba(234,179,8,0.1); color: rgb(234,179,8);">mock</span>
							{/if}
						</div>
					</div>
					<div class="flex items-center gap-2">
						<button
							onclick={() => handleReconnect(sn.id)}
							disabled={reconnecting[sn.id]}
							class="text-[11px] px-3 py-1 rounded font-medium cursor-pointer transition-colors disabled:opacity-50"
							style="background: var(--accent); color: var(--accent-text);"
						>
							{reconnecting[sn.id] ? 'Connecting...' : 'Reconnect'}
						</button>
						<button
							onclick={() => handleRemove(sn.id)}
							class="text-[11px] px-2 py-1 rounded font-medium cursor-pointer transition-colors"
							style="background: rgba(239,68,68,0.1); color: rgb(239,68,68);"
						>
							Remove
						</button>
					</div>
				</div>
			{/each}
		</div>
	{/if}
</div>

<!-- Disconnect confirmation modal -->
{#if confirmDisconnect}
	<!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
	<div
		class="fixed inset-0 flex items-center justify-center z-50"
		style="background: rgba(0,0,0,0.5);"
		onclick={() => confirmDisconnect = null}
	>
		<!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
		<div
			class="rounded-lg p-6 space-y-4 w-[360px] shadow-xl"
			style="background: var(--bg-secondary); border: 1px solid var(--border);"
			onclick={(e) => e.stopPropagation()}
		>
			<h3 class="text-[14px] font-semibold" style="color: var(--text-primary);">Disconnect Node</h3>
			<p class="text-[13px]" style="color: var(--text-tertiary);">
				Are you sure you want to disconnect <span class="font-[JetBrains_Mono,monospace] text-[12px]" style="color: var(--text-secondary);">{confirmDisconnect}</span>?
			</p>
			<p class="text-[12px]" style="color: var(--text-muted);">
				The remote agent will keep running. You can reconnect later.
			</p>
			<div class="flex justify-end gap-2 pt-1">
				<button
					onclick={() => confirmDisconnect = null}
					class="px-3 py-1.5 rounded-md text-[13px] font-medium transition-colors cursor-pointer"
					style="background: var(--bg-tertiary); color: var(--text-secondary);"
				>
					Cancel
				</button>
				<button
					onclick={doDisconnect}
					class="px-3 py-1.5 rounded-md text-[13px] font-medium transition-colors cursor-pointer bg-red-500/10 text-red-500 hover:bg-red-500/20"
				>
					Disconnect
				</button>
			</div>
		</div>
	</div>
{/if}
