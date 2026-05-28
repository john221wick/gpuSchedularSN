<script>
	let { remoteMode = false } = $props();
	let activeSection = $state('getting-started');

	const sections = [
		{ id: 'getting-started', label: 'Getting Started' },
		{ id: 'local-mode', label: 'Local Mode' },
		{ id: 'remote-mode', label: 'Remote Mode' },
		{ id: 'jobs', label: 'Running Jobs' },
		{ id: 'terminal', label: 'Terminal' },
		{ id: 'troubleshooting', label: 'Troubleshooting' },
	];
</script>

<div class="flex h-full">
	<!-- Docs sidebar -->
	<div class="w-[180px] shrink-0 p-4 overflow-y-auto" style="border-right: 1px solid var(--border);">
		<div class="text-[10px] font-semibold uppercase tracking-widest px-2 pb-2" style="color: var(--text-muted);">Docs</div>
		{#each sections as s (s.id)}
			<button
				onclick={() => activeSection = s.id}
				class="block w-full text-left px-2 py-1.5 rounded text-[12px] font-medium cursor-pointer transition-colors"
				style="color: {activeSection === s.id ? 'var(--text-primary)' : 'var(--text-tertiary)'}; background: {activeSection === s.id ? 'var(--hover-bg)' : 'transparent'};"
			>
				{s.label}
			</button>
		{/each}
	</div>

	<!-- Content -->
	<div class="flex-1 overflow-y-auto p-8 max-w-[720px]">

		{#if activeSection === 'getting-started'}
			<h1 class="text-lg font-semibold mb-4" style="color: var(--text-primary);">Getting Started</h1>
			<div class="space-y-3 text-[13px] leading-relaxed" style="color: var(--text-secondary);">
				<p>gpusched is a GPU scheduler that manages job queues and allocates GPUs for your workloads. It works in two modes:</p>
				<div class="rounded-md p-4 space-y-2" style="background: var(--bg-secondary); border: 1px solid var(--border);">
					<p><strong style="color: var(--text-primary);">Local Mode</strong> — Schedule jobs on GPUs attached to this machine.</p>
					<p><strong style="color: var(--text-primary);">Remote Mode</strong> — Connect to remote servers via SSH and schedule jobs across a cluster.</p>
				</div>
				<p>Switch between modes from the <strong style="color: var(--text-primary);">Dashboard</strong> page using the mode toggle.</p>
			</div>

		{:else if activeSection === 'local-mode'}
			<h1 class="text-lg font-semibold mb-4" style="color: var(--text-primary);">Local Mode</h1>
			<div class="space-y-4 text-[13px] leading-relaxed" style="color: var(--text-secondary);">
				<p>Local mode detects GPUs on your machine and lets you queue jobs against them.</p>

				<h2 class="text-[14px] font-semibold pt-2" style="color: var(--text-primary);">Setup</h2>
				<ol class="list-decimal pl-5 space-y-1.5">
					<li>Open the app — local GPUs are detected automatically.</li>
					<li>Optionally set a <strong>Path Variable</strong> in Settings. This is the base directory for resolving relative script paths (e.g. <code class="px-1 py-0.5 rounded text-[12px]" style="background: var(--bg-tertiary);">train.py</code> resolves to <code class="px-1 py-0.5 rounded text-[12px]" style="background: var(--bg-tertiary);">/your/path/train.py</code>).</li>
					<li>Go to <strong>Run Job</strong> and submit a command.</li>
				</ol>

				<h2 class="text-[14px] font-semibold pt-2" style="color: var(--text-primary);">Pages</h2>
				<div class="space-y-2">
					<p><strong style="color: var(--text-primary);">GPUs</strong> — View all detected GPUs, their VRAM, utilization, and allocation status.</p>
					<p><strong style="color: var(--text-primary);">Topology</strong> — See how GPUs are connected (NVLink, PCIe, etc). The scheduler uses this to pick GPUs that are close together.</p>
					<p><strong style="color: var(--text-primary);">Jobs</strong> — Monitor running, queued, and completed jobs. Kill running jobs or remove queued ones.</p>
				</div>
			</div>

		{:else if activeSection === 'remote-mode'}
			<h1 class="text-lg font-semibold mb-4" style="color: var(--text-primary);">Remote Mode</h1>
			<div class="space-y-4 text-[13px] leading-relaxed" style="color: var(--text-secondary);">
				<p>Remote mode lets you connect to GPU servers via SSH and run jobs on them.</p>

				<h2 class="text-[14px] font-semibold pt-2" style="color: var(--text-primary);">Connecting a Node</h2>
				<ol class="list-decimal pl-5 space-y-1.5">
					<li>Enable Remote Mode from the Dashboard toggle.</li>
					<li>Go to <strong>Nodes</strong> and click <strong>Connect Node</strong>.</li>
					<li>Paste your SSH command (e.g. <code class="px-1 py-0.5 rounded text-[12px]" style="background: var(--bg-tertiary);">ssh -p 20544 root@203.0.113.10</code>) or an SSH config alias (e.g. <code class="px-1 py-0.5 rounded text-[12px]" style="background: var(--bg-tertiary);">ssh host</code>).</li>
					<li>Optionally provide a key path if not using default <code class="px-1 py-0.5 rounded text-[12px]" style="background: var(--bg-tertiary);">~/.ssh/id_ed25519</code>.</li>
					<li>Check <strong>Mock mode</strong> if testing on a server without GPUs.</li>
					<li>Click Connect. The app will deploy an agent binary, detect GPUs, and set up a tunnel.</li>
				</ol>

				<h2 class="text-[14px] font-semibold pt-2" style="color: var(--text-primary);">What happens on connect</h2>
				<div class="rounded-md p-4 space-y-1.5 text-[12px] font-[JetBrains_Mono,monospace]" style="background: #0d0d0d; color: #a3a3a3; border: 1px solid var(--border);">
					<p>1. SSH connection established</p>
					<p>2. Remote GPU detection (nvidia-smi / rocm-smi)</p>
					<p>3. Agent binary cross-compiled and deployed to ~/gpuschedular/</p>
					<p>4. Agent started on remote, SSH tunnel created</p>
					<p>5. Previous jobs recovered if reconnecting</p>
				</div>

				<h2 class="text-[14px] font-semibold pt-2" style="color: var(--text-primary);">Syncing Files</h2>
				<p>On the Nodes page, click <strong>Sync Files</strong> on a node. Enter a local directory path and click Sync. Files are copied to the remote via rsync.</p>
				<p>Default remote destination: <code class="px-1 py-0.5 rounded text-[12px]" style="background: var(--bg-tertiary);">/root/gpuschedular</code></p>

				<h2 class="text-[14px] font-semibold pt-2" style="color: var(--text-primary);">Reconnecting</h2>
				<p>When you disconnect a node, the config is saved. The remote agent keeps running. Come back later and click <strong>Reconnect</strong> — jobs that ran while you were away are recovered.</p>
			</div>

		{:else if activeSection === 'jobs'}
			<h1 class="text-lg font-semibold mb-4" style="color: var(--text-primary);">Running Jobs</h1>
			<div class="space-y-4 text-[13px] leading-relaxed" style="color: var(--text-secondary);">
				<h2 class="text-[14px] font-semibold" style="color: var(--text-primary);">Submitting</h2>
				<p>Go to <strong>Run Job</strong> and enter:</p>
				<ul class="list-disc pl-5 space-y-1">
					<li><strong>Command</strong> — the command to run (e.g. <code class="px-1 py-0.5 rounded text-[12px]" style="background: var(--bg-tertiary);">python3 train.py --epochs 10</code>)</li>
					<li><strong>Number of GPUs</strong> — how many GPUs to allocate</li>
					<li><strong>Min VRAM</strong> — minimum VRAM per GPU (optional filter)</li>
					<li><strong>Priority</strong> — lower number = higher priority (default 10)</li>
				</ul>

				<h2 class="text-[14px] font-semibold pt-2" style="color: var(--text-primary);">How scheduling works</h2>
				<p>Jobs are queued by priority. The scheduler picks the best available GPUs based on:</p>
				<ul class="list-disc pl-5 space-y-1">
					<li>VRAM requirements</li>
					<li>GPU topology (prefers GPUs connected via fast links)</li>
					<li>Current utilization</li>
				</ul>
				<p>Your command runs with <code class="px-1 py-0.5 rounded text-[12px]" style="background: var(--bg-tertiary);">CUDA_VISIBLE_DEVICES</code> set to the allocated GPU IDs.</p>

				<h2 class="text-[14px] font-semibold pt-2" style="color: var(--text-primary);">Monitoring</h2>
				<p>The <strong>Jobs</strong> page shows running, queued, and completed jobs. You can:</p>
				<ul class="list-disc pl-5 space-y-1">
					<li>Kill a running job</li>
					<li>Remove a queued job</li>
					<li>Change priority of queued jobs</li>
					<li>View logs for remote jobs (click the Logs button)</li>
				</ul>
			</div>

		{:else if activeSection === 'terminal'}
			<h1 class="text-lg font-semibold mb-4" style="color: var(--text-primary);">Terminal</h1>
			<div class="space-y-4 text-[13px] leading-relaxed" style="color: var(--text-secondary);">
				<p>The Terminal page gives you shell access to connected remote nodes. Available in remote mode only.</p>

				<h2 class="text-[14px] font-semibold pt-2" style="color: var(--text-primary);">Features</h2>
				<ul class="list-disc pl-5 space-y-1">
					<li><strong>Arrow Up/Down</strong> — command history</li>
					<li><strong>Ctrl+L</strong> — clear terminal</li>
					<li><code class="px-1 py-0.5 rounded text-[12px]" style="background: var(--bg-tertiary);">cd</code> is persistent across commands</li>
					<li>Select different nodes from the dropdown (if multiple connected)</li>
				</ul>

				<div class="rounded-md p-3 text-[12px]" style="background: rgba(234,179,8,0.1); color: rgb(234,179,8); border: 1px solid rgba(234,179,8,0.2);">
					Note: Each command runs as a separate SSH session. Interactive commands (vim, top, htop) are not supported. Use it for quick checks and file operations.
				</div>
			</div>

		{:else if activeSection === 'troubleshooting'}
			<h1 class="text-lg font-semibold mb-4" style="color: var(--text-primary);">Troubleshooting</h1>
			<div class="space-y-4 text-[13px] leading-relaxed" style="color: var(--text-secondary);">
				<div class="space-y-4">
					<div>
						<p class="font-semibold" style="color: var(--text-primary);">Connection fails</p>
						<p>Check that your SSH key is correct and the server is reachable. Try connecting manually with the same SSH command first.</p>
					</div>
					<div>
						<p class="font-semibold" style="color: var(--text-primary);">No GPUs detected</p>
						<p>The remote server needs <code class="px-1 py-0.5 rounded text-[12px]" style="background: var(--bg-tertiary);">nvidia-smi</code> or <code class="px-1 py-0.5 rounded text-[12px]" style="background: var(--bg-tertiary);">rocm-smi</code> installed and working. Use Mock mode for testing without GPUs.</p>
					</div>
					<div>
						<p class="font-semibold" style="color: var(--text-primary);">Job stuck in queue</p>
						<p>Not enough free GPUs matching your requirements. Check the GPUs page for current allocation. Reduce GPU count or VRAM requirement.</p>
					</div>
					<div>
						<p class="font-semibold" style="color: var(--text-primary);">File sync not working</p>
						<p>Requires <code class="px-1 py-0.5 rounded text-[12px]" style="background: var(--bg-tertiary);">rsync</code> on the remote. The app tries to install it automatically, but if it fails, install manually.</p>
					</div>
					<div>
						<p class="font-semibold" style="color: var(--text-primary);">Check app logs</p>
						<p>Go to the <strong>Logs</strong> page to see all internal events — connections, errors, job dispatches, and file transfers.</p>
					</div>
				</div>
			</div>
		{/if}
	</div>
</div>
