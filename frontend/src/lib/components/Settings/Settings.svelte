<script lang="ts">
	import { onMount } from 'svelte';
	import { api } from '$lib/api/client';
	import {
		Settings as SettingsIcon,
		X,
		Speaker,
		Sliders,
		Info,
		Loader2,
		Volume2,
		Save,
		HardDrive,
		FolderOpen,
		ChevronRight,
		RefreshCw
	} from 'lucide-svelte';

	interface SpeakerInfo {
		ip: string;
		name: string;
		model: string;
		firmware: string;
		macPrimary: string;
	}

	interface SpeakerSettings {
		maxVolume: number;
		volume: number;
		muted: boolean;
		source: string;
		poweredOn: boolean;
	}

	interface EQSettings {
		profileName: string;
		bassExtension: string;
		deskMode: boolean;
		deskModeSetting: number;
		wallMode: boolean;
		wallModeSetting: number;
		trebleAmount: number;
		balance: number;
		phaseCorrection: boolean;
		isExpertMode: boolean;
	}

	interface SubwooferSettings {
		enabled: boolean;
		count: number;
		gain: number;
		polarity: string;
		preset: string;
		lowPassFreq: number;
		stereo: boolean;
		highPassMode: boolean;
		highPassFreq: number;
	}

	interface UPnPSettings {
		defaultServer: string;
		defaultServerPath: string;
		browseContainer: string;
		indexContainer: string;
	}

	interface UPnPServer {
		name: string;
		path: string;
		icon: string;
	}

	let open = $state(false);
	let activeTab = $state<'speaker' | 'eq' | 'media' | 'about'>('speaker');
	let loading = $state(false);
	let saving = $state(false);
	let error = $state<string | null>(null);

	// Data
	let speakerInfo = $state<SpeakerInfo | null>(null);
	let speakerSettings = $state<SpeakerSettings | null>(null);
	let eqSettings = $state<EQSettings | null>(null);
	let subwooferSettings = $state<SubwooferSettings | null>(null);
	let appVersion = $state<string>('');

	// UPnP/Media settings
	let upnpSettings = $state<UPnPSettings | null>(null);
	let upnpServers = $state<UPnPServer[]>([]);
	let browseContainers = $state<string[]>([]);
	let indexContainers = $state<string[]>([]);
	let loadingServers = $state(false);
	let loadingBrowseContainers = $state(false);
	let loadingIndexContainers = $state(false);
	
	// Editable UPnP settings
	let selectedServer = $state<string>('');
	let selectedServerPath = $state<string>('');
	let browseContainerPath = $state<string>('');
	let indexContainerPath = $state<string>('');
	let upnpChanged = $state(false);

	// Editable settings
	let maxVolumeInput = $state(100);
	let maxVolumeChanged = $state(false);

	async function loadSpeakerSettings() {
		try {
			loading = true;
			error = null;
			const response = await api.getSpeakerSettings();
			speakerInfo = response.speaker;
			speakerSettings = response.settings;
			maxVolumeInput = response.settings.maxVolume;
			maxVolumeChanged = false;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load settings';
		} finally {
			loading = false;
		}
	}

	async function loadEQSettings() {
		try {
			loading = true;
			error = null;
			const response = await api.getEQSettings();
			eqSettings = response.eq;
			subwooferSettings = response.subwoofer;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load EQ settings';
		} finally {
			loading = false;
		}
	}

	async function loadAppSettings() {
		try {
			const response = await api.getAppSettings();
			appVersion = response.version;
		} catch (e) {
			console.error('Failed to load app settings:', e);
		}
	}

	async function saveMaxVolume() {
		if (!maxVolumeChanged) return;

		try {
			saving = true;
			await api.updateSpeakerSettings({ maxVolume: maxVolumeInput });
			maxVolumeChanged = false;
			// Reload to confirm
			await loadSpeakerSettings();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to save settings';
		} finally {
			saving = false;
		}
	}

	function handleMaxVolumeChange(e: Event) {
		const value = parseInt((e.target as HTMLInputElement).value);
		if (!isNaN(value) && value >= 0 && value <= 100) {
			maxVolumeInput = value;
			maxVolumeChanged = speakerSettings ? value !== speakerSettings.maxVolume : false;
		}
	}

	// UPnP/Media Settings functions
	interface ContainerItem {
		name: string;
		path: string;
	}

	let browseContainerItems = $state<ContainerItem[]>([]);
	let indexContainerItems = $state<ContainerItem[]>([]);
	let browseCurrentPath = $state<string[]>([]);
	let indexCurrentPath = $state<string[]>([]);

	async function loadUPnPSettings() {
		try {
			loading = true;
			error = null;
			const settings = await api.getUPnPSettings();
			upnpSettings = settings;
			selectedServer = settings.defaultServer || '';
			selectedServerPath = settings.defaultServerPath || '';
			browseContainerPath = settings.browseContainer || '';
			indexContainerPath = settings.indexContainer || '';
			upnpChanged = false;

			// Parse current paths for breadcrumbs
			browseCurrentPath = browseContainerPath ? browseContainerPath.split('/') : [];
			indexCurrentPath = indexContainerPath ? indexContainerPath.split('/') : [];
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load UPnP settings';
		} finally {
			loading = false;
		}
	}

	async function loadUPnPServers() {
		try {
			loadingServers = true;
			const response = await api.getUPnPServers();
			upnpServers = response.servers || [];
		} catch (e) {
			console.error('Failed to load UPnP servers:', e);
			upnpServers = [];
		} finally {
			loadingServers = false;
		}
	}

	async function loadBrowseContainers(path: string = '') {
		if (!selectedServerPath) return;
		try {
			loadingBrowseContainers = true;
			const response = await api.getUPnPContainers(selectedServerPath, path || undefined);
			browseContainerItems = (response.containers || []).map((name) => ({
				name,
				path: path ? `${path}/${name}` : name
			}));
		} catch (e) {
			console.error('Failed to load browse containers:', e);
			browseContainerItems = [];
		} finally {
			loadingBrowseContainers = false;
		}
	}

	async function loadIndexContainers(path: string = '') {
		if (!selectedServerPath) return;
		try {
			loadingIndexContainers = true;
			const response = await api.getUPnPContainers(selectedServerPath, path || undefined);
			indexContainerItems = (response.containers || []).map((name) => ({
				name,
				path: path ? `${path}/${name}` : name
			}));
		} catch (e) {
			console.error('Failed to load index containers:', e);
			indexContainerItems = [];
		} finally {
			loadingIndexContainers = false;
		}
	}

	async function saveUPnPSettings() {
		try {
			saving = true;
			await api.updateUPnPSettings({
				defaultServer: selectedServer,
				defaultServerPath: selectedServerPath,
				browseContainer: browseContainerPath,
				indexContainer: indexContainerPath
			});
			upnpChanged = false;
			await loadUPnPSettings();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to save UPnP settings';
		} finally {
			saving = false;
		}
	}

	function handleServerChange(e: Event) {
		const select = e.target as HTMLSelectElement;
		const server = upnpServers.find((s) => s.path === select.value);
		if (server) {
			selectedServer = server.name;
			selectedServerPath = server.path;
			// Reset containers when server changes
			browseContainerPath = '';
			indexContainerPath = '';
			browseCurrentPath = [];
			indexCurrentPath = [];
			browseContainerItems = [];
			indexContainerItems = [];
			upnpChanged = true;
			// Load root containers for new server
			loadBrowseContainers('');
			loadIndexContainers('');
		} else {
			selectedServer = '';
			selectedServerPath = '';
		}
	}

	function navigateBrowseContainer(containerPath: string) {
		browseContainerPath = containerPath;
		browseCurrentPath = containerPath ? containerPath.split('/') : [];
		upnpChanged = true;
		loadBrowseContainers(containerPath);
	}

	function navigateIndexContainer(containerPath: string) {
		indexContainerPath = containerPath;
		indexCurrentPath = containerPath ? containerPath.split('/') : [];
		upnpChanged = true;
		loadIndexContainers(containerPath);
	}

	function selectBrowseContainer(container: ContainerItem) {
		browseContainerPath = container.path;
		browseCurrentPath = container.path.split('/');
		upnpChanged = true;
		loadBrowseContainers(container.path);
	}

	function selectIndexContainer(container: ContainerItem) {
		indexContainerPath = container.path;
		indexCurrentPath = container.path.split('/');
		upnpChanged = true;
		loadIndexContainers(container.path);
	}

	function setBrowseContainerHere() {
		upnpChanged = true;
	}

	function setIndexContainerHere() {
		upnpChanged = true;
	}

	function clearBrowseContainer() {
		browseContainerPath = '';
		browseCurrentPath = [];
		browseContainerItems = [];
		upnpChanged = true;
		if (selectedServerPath) {
			loadBrowseContainers('');
		}
	}

	function clearIndexContainer() {
		indexContainerPath = '';
		indexCurrentPath = [];
		indexContainerItems = [];
		upnpChanged = true;
		if (selectedServerPath) {
			loadIndexContainers('');
		}
	}

	function openSettings() {
		open = true;
		loadSpeakerSettings();
		loadAppSettings();
	}

	function closeSettings() {
		open = false;
	}

	function switchTab(tab: 'speaker' | 'eq' | 'media' | 'about') {
		activeTab = tab;
		error = null;
		if (tab === 'speaker' && !speakerInfo) {
			loadSpeakerSettings();
		} else if (tab === 'eq' && !eqSettings) {
			loadEQSettings();
		} else if (tab === 'media' && !upnpSettings) {
			loadUPnPSettings();
			loadUPnPServers();
		}
	}

	function formatBoolean(value: boolean): string {
		return value ? 'On' : 'Off';
	}
</script>

<!-- Settings Button -->
<button
	class="rounded-full p-2 text-zinc-400 transition-colors hover:bg-zinc-800 hover:text-zinc-200"
	onclick={openSettings}
	title="Settings"
>
	<SettingsIcon class="h-5 w-5" />
</button>

<!-- Modal Backdrop -->
{#if open}
	<button
		class="fixed inset-0 z-50 cursor-default bg-black/60 backdrop-blur-sm"
		onclick={closeSettings}
		aria-label="Close settings"
	></button>

	<!-- Modal Content -->
	<div
		class="fixed left-1/2 top-1/2 z-50 flex h-[80vh] w-full max-w-2xl -translate-x-1/2 -translate-y-1/2 flex-col rounded-lg border border-zinc-700 bg-zinc-900 shadow-xl"
	>
		<!-- Header -->
		<div class="flex items-center justify-between border-b border-zinc-700 px-5 py-4">
			<div class="flex items-center gap-2">
				<SettingsIcon class="h-5 w-5 text-zinc-400" />
				<h2 class="text-lg font-semibold text-white">Settings</h2>
			</div>
			<button
				class="rounded p-1.5 text-zinc-400 transition-colors hover:bg-zinc-800 hover:text-white"
				onclick={closeSettings}
			>
				<X class="h-5 w-5" />
			</button>
		</div>

		<!-- Tabs -->
		<div class="flex border-b border-zinc-800">
			<button
				class="flex items-center gap-2 px-5 py-3 text-sm font-medium transition-colors"
				class:border-b-2={activeTab === 'speaker'}
				class:border-green-500={activeTab === 'speaker'}
				class:text-white={activeTab === 'speaker'}
				class:text-zinc-400={activeTab !== 'speaker'}
				onclick={() => switchTab('speaker')}
			>
				<Speaker class="h-4 w-4" />
				Speaker
			</button>
			<button
				class="flex items-center gap-2 px-5 py-3 text-sm font-medium transition-colors"
				class:border-b-2={activeTab === 'eq'}
				class:border-green-500={activeTab === 'eq'}
				class:text-white={activeTab === 'eq'}
				class:text-zinc-400={activeTab !== 'eq'}
				onclick={() => switchTab('eq')}
			>
				<Sliders class="h-4 w-4" />
				EQ & DSP
			</button>
			<button
				class="flex items-center gap-2 px-5 py-3 text-sm font-medium transition-colors"
				class:border-b-2={activeTab === 'media'}
				class:border-green-500={activeTab === 'media'}
				class:text-white={activeTab === 'media'}
				class:text-zinc-400={activeTab !== 'media'}
				onclick={() => switchTab('media')}
			>
				<HardDrive class="h-4 w-4" />
				Media
			</button>
			<button
				class="flex items-center gap-2 px-5 py-3 text-sm font-medium transition-colors"
				class:border-b-2={activeTab === 'about'}
				class:border-green-500={activeTab === 'about'}
				class:text-white={activeTab === 'about'}
				class:text-zinc-400={activeTab !== 'about'}
				onclick={() => switchTab('about')}
			>
				<Info class="h-4 w-4" />
				About
			</button>
		</div>

		<!-- Content -->
		<div class="flex-1 overflow-y-auto p-5">
			{#if error}
				<div class="mb-4 rounded bg-red-900/30 px-4 py-3 text-sm text-red-400">
					{error}
				</div>
			{/if}

			{#if loading}
				<div class="flex items-center justify-center py-12">
					<Loader2 class="h-8 w-8 animate-spin text-zinc-500" />
				</div>
			{:else if activeTab === 'speaker'}
				<!-- Speaker Settings Tab -->
				{#if speakerInfo}
					<div class="space-y-6">
						<!-- Speaker Info -->
						<div>
							<h3 class="mb-3 text-sm font-medium text-zinc-300">Speaker Information</h3>
							<div class="rounded-lg border border-zinc-800 bg-zinc-800/50 p-4">
								<dl class="grid grid-cols-2 gap-3 text-sm">
									<div>
										<dt class="text-zinc-500">Name</dt>
										<dd class="text-zinc-200">{speakerInfo.name}</dd>
									</div>
									<div>
										<dt class="text-zinc-500">Model</dt>
										<dd class="text-zinc-200">{speakerInfo.model}</dd>
									</div>
									<div>
										<dt class="text-zinc-500">IP Address</dt>
										<dd class="font-mono text-zinc-200">{speakerInfo.ip}</dd>
									</div>
									<div>
										<dt class="text-zinc-500">MAC Address</dt>
										<dd class="font-mono text-zinc-200">{speakerInfo.macPrimary}</dd>
									</div>
									<div class="col-span-2">
										<dt class="text-zinc-500">Firmware</dt>
										<dd class="text-zinc-200">{speakerInfo.firmware}</dd>
									</div>
								</dl>
							</div>
						</div>

						<!-- Current State -->
						{#if speakerSettings}
							<div>
								<h3 class="mb-3 text-sm font-medium text-zinc-300">Current State</h3>
								<div class="rounded-lg border border-zinc-800 bg-zinc-800/50 p-4">
									<dl class="grid grid-cols-2 gap-3 text-sm">
										<div>
											<dt class="text-zinc-500">Power</dt>
											<dd class="text-zinc-200">
												<span
													class="inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 text-xs {speakerSettings.poweredOn ? 'bg-green-900/50 text-green-400' : 'bg-zinc-700 text-zinc-400'}"
												>
													{speakerSettings.poweredOn ? 'On' : 'Standby'}
												</span>
											</dd>
										</div>
										<div>
											<dt class="text-zinc-500">Source</dt>
											<dd class="text-zinc-200 capitalize">{speakerSettings.source}</dd>
										</div>
										<div>
											<dt class="text-zinc-500">Volume</dt>
											<dd class="text-zinc-200">{speakerSettings.volume}%</dd>
										</div>
										<div>
											<dt class="text-zinc-500">Muted</dt>
											<dd class="text-zinc-200">{formatBoolean(speakerSettings.muted)}</dd>
										</div>
									</dl>
								</div>
							</div>
						{/if}

						<!-- Max Volume Setting -->
						<div>
							<h3 class="mb-3 text-sm font-medium text-zinc-300">Volume Limit</h3>
							<div class="rounded-lg border border-zinc-800 bg-zinc-800/50 p-4">
								<div class="flex items-center gap-4">
									<Volume2 class="h-5 w-5 text-zinc-400" />
									<div class="flex-1">
										<label for="maxVolume" class="mb-1 block text-sm text-zinc-400">
											Maximum Volume
										</label>
										<div class="flex items-center gap-3">
											<input
												id="maxVolume"
												type="range"
												min="0"
												max="100"
												value={maxVolumeInput}
												oninput={handleMaxVolumeChange}
												class="h-2 flex-1 cursor-pointer appearance-none rounded-lg bg-zinc-700 accent-green-500"
											/>
											<span class="w-12 text-right text-sm text-zinc-200">{maxVolumeInput}%</span>
										</div>
									</div>
								</div>
								{#if maxVolumeChanged}
									<div class="mt-3 flex justify-end">
										<button
											class="flex items-center gap-1.5 rounded bg-green-600 px-3 py-1.5 text-sm font-medium text-white transition-colors hover:bg-green-500 disabled:opacity-50"
											onclick={saveMaxVolume}
											disabled={saving}
										>
											{#if saving}
												<Loader2 class="h-4 w-4 animate-spin" />
											{:else}
												<Save class="h-4 w-4" />
											{/if}
											Save
										</button>
									</div>
								{/if}
							</div>
						</div>
					</div>
				{:else}
					<div class="py-8 text-center text-sm text-zinc-500">
						No speaker connected
					</div>
				{/if}
			{:else if activeTab === 'eq'}
				<!-- EQ & DSP Tab -->
				{#if eqSettings}
					<div class="space-y-6">
						<!-- EQ Profile -->
						<div>
							<h3 class="mb-3 text-sm font-medium text-zinc-300">Equalizer Profile</h3>
							<div class="rounded-lg border border-zinc-800 bg-zinc-800/50 p-4">
								<p class="mb-3 text-xs text-zinc-500">
									EQ settings are read-only. Use the KEF Connect app to modify these settings.
								</p>
								<dl class="grid grid-cols-2 gap-3 text-sm">
									<div>
										<dt class="text-zinc-500">Profile</dt>
										<dd class="text-zinc-200">{eqSettings.profileName || 'Default'}</dd>
									</div>
									<div>
										<dt class="text-zinc-500">Bass Extension</dt>
										<dd class="capitalize text-zinc-200">{eqSettings.bassExtension}</dd>
									</div>
									<div>
										<dt class="text-zinc-500">Treble</dt>
										<dd class="text-zinc-200">{eqSettings.trebleAmount > 0 ? '+' : ''}{eqSettings.trebleAmount} dB</dd>
									</div>
									<div>
										<dt class="text-zinc-500">Balance</dt>
										<dd class="text-zinc-200">
											{eqSettings.balance === 0 ? 'Center' : eqSettings.balance > 0 ? `R +${eqSettings.balance}` : `L ${eqSettings.balance}`}
										</dd>
									</div>
									<div>
										<dt class="text-zinc-500">Desk Mode</dt>
										<dd class="text-zinc-200">
											{formatBoolean(eqSettings.deskMode)}
											{#if eqSettings.deskMode}
												({eqSettings.deskModeSetting} dB)
											{/if}
										</dd>
									</div>
									<div>
										<dt class="text-zinc-500">Wall Mode</dt>
										<dd class="text-zinc-200">
											{formatBoolean(eqSettings.wallMode)}
											{#if eqSettings.wallMode}
												({eqSettings.wallModeSetting} dB)
											{/if}
										</dd>
									</div>
									<div>
										<dt class="text-zinc-500">Phase Correction</dt>
										<dd class="text-zinc-200">{formatBoolean(eqSettings.phaseCorrection)}</dd>
									</div>
									<div>
										<dt class="text-zinc-500">Expert Mode</dt>
										<dd class="text-zinc-200">{formatBoolean(eqSettings.isExpertMode)}</dd>
									</div>
								</dl>
							</div>
						</div>

						<!-- Subwoofer Settings -->
						{#if subwooferSettings}
							<div>
								<h3 class="mb-3 text-sm font-medium text-zinc-300">Subwoofer</h3>
								<div class="rounded-lg border border-zinc-800 bg-zinc-800/50 p-4">
									<dl class="grid grid-cols-2 gap-3 text-sm">
										<div>
											<dt class="text-zinc-500">Enabled</dt>
											<dd class="text-zinc-200">{formatBoolean(subwooferSettings.enabled)}</dd>
										</div>
										<div>
											<dt class="text-zinc-500">Count</dt>
											<dd class="text-zinc-200">{subwooferSettings.count}</dd>
										</div>
										<div>
											<dt class="text-zinc-500">Gain</dt>
											<dd class="text-zinc-200">{subwooferSettings.gain > 0 ? '+' : ''}{subwooferSettings.gain} dB</dd>
										</div>
										<div>
											<dt class="text-zinc-500">Low Pass</dt>
											<dd class="text-zinc-200">{subwooferSettings.lowPassFreq} Hz</dd>
										</div>
										<div>
											<dt class="text-zinc-500">High Pass Mode</dt>
											<dd class="text-zinc-200">
												{formatBoolean(subwooferSettings.highPassMode)}
												{#if subwooferSettings.highPassMode}
													({subwooferSettings.highPassFreq} Hz)
												{/if}
											</dd>
										</div>
										<div>
											<dt class="text-zinc-500">Polarity</dt>
											<dd class="capitalize text-zinc-200">{subwooferSettings.polarity}</dd>
										</div>
									</dl>
								</div>
							</div>
						{/if}
					</div>
				{:else}
					<div class="py-8 text-center text-sm text-zinc-500">
						No EQ data available
					</div>
				{/if}
			{:else if activeTab === 'media'}
				<!-- Media/UPnP Settings Tab -->
				<div class="space-y-6">
					<!-- Default Server -->
					<div>
						<h3 class="mb-3 text-sm font-medium text-zinc-300">Default Media Server</h3>
						<div class="rounded-lg border border-zinc-800 bg-zinc-800/50 p-4">
							<div class="flex items-center gap-3">
								<HardDrive class="h-5 w-5 shrink-0 text-zinc-400" />
								<div class="flex-1">
									<label for="upnpServer" class="mb-1 block text-sm text-zinc-400">
										UPnP/DLNA Server
									</label>
									<div class="flex items-center gap-2">
										<select
											id="upnpServer"
											class="flex-1 rounded bg-zinc-700 px-3 py-2 text-sm text-zinc-200 focus:outline-none focus:ring-2 focus:ring-green-500"
											value={selectedServerPath}
											onchange={handleServerChange}
											disabled={loadingServers}
										>
											<option value="">Select a server...</option>
											{#each upnpServers as server}
												<option value={server.path}>{server.name}</option>
											{/each}
										</select>
										<button
											class="rounded p-2 text-zinc-400 transition-colors hover:bg-zinc-700 hover:text-white"
											onclick={() => loadUPnPServers()}
											disabled={loadingServers}
											title="Refresh servers"
										>
											{#if loadingServers}
												<Loader2 class="h-4 w-4 animate-spin" />
											{:else}
												<RefreshCw class="h-4 w-4" />
											{/if}
										</button>
									</div>
								</div>
							</div>
							{#if !selectedServerPath}
								<p class="mt-2 text-xs text-zinc-500">
									Select a media server to configure browse and index containers.
								</p>
							{/if}
						</div>
					</div>

					{#if selectedServerPath}
						<!-- Browse Container -->
						<div>
							<h3 class="mb-3 text-sm font-medium text-zinc-300">Browse Container</h3>
							<div class="rounded-lg border border-zinc-800 bg-zinc-800/50 p-4">
								<p class="mb-3 text-xs text-zinc-500">
									Set the starting point for browsing. Parent folders above this path will be hidden.
								</p>
								
								<!-- Current path display -->
								<div class="mb-3 flex items-center gap-2">
									<FolderOpen class="h-4 w-4 shrink-0 text-zinc-400" />
									<div class="flex flex-wrap items-center gap-1 text-sm">
										<button
											class="rounded px-1.5 py-0.5 text-zinc-300 hover:bg-zinc-700"
											onclick={() => navigateBrowseContainer('')}
										>
											Root
										</button>
										{#each browseCurrentPath as segment, i}
											<ChevronRight class="h-3 w-3 text-zinc-600" />
											<button
												class="rounded px-1.5 py-0.5 text-zinc-300 hover:bg-zinc-700"
												onclick={() => navigateBrowseContainer(browseCurrentPath.slice(0, i + 1).join('/'))}
											>
												{segment}
											</button>
										{/each}
									</div>
									{#if browseContainerPath}
										<button
											class="ml-auto text-xs text-zinc-500 hover:text-zinc-300"
											onclick={clearBrowseContainer}
										>
											Clear
										</button>
									{/if}
								</div>

								<!-- Container list -->
								<div class="max-h-40 overflow-y-auto rounded border border-zinc-700 bg-zinc-800">
									{#if loadingBrowseContainers}
										<div class="flex items-center justify-center py-4">
											<Loader2 class="h-5 w-5 animate-spin text-zinc-500" />
										</div>
									{:else if browseContainerItems.length === 0}
										<div class="px-3 py-4 text-center text-xs text-zinc-500">
											No containers found
										</div>
									{:else}
										{#each browseContainerItems as container}
											<button
												class="flex w-full items-center gap-2 px-3 py-2 text-left text-sm text-zinc-300 hover:bg-zinc-700"
												onclick={() => selectBrowseContainer(container)}
											>
												<FolderOpen class="h-4 w-4 shrink-0 text-zinc-500" />
												{container.name}
												<ChevronRight class="ml-auto h-4 w-4 text-zinc-600" />
											</button>
										{/each}
									{/if}
								</div>

								{#if browseContainerPath}
									<div class="mt-2 flex items-center gap-2 rounded bg-green-900/20 px-3 py-2 text-xs text-green-400">
										<span>Browse root set to:</span>
										<span class="font-medium">{browseContainerPath}</span>
									</div>
								{/if}
							</div>
						</div>

						<!-- Index Container -->
						<div>
							<h3 class="mb-3 text-sm font-medium text-zinc-300">Index Container</h3>
							<div class="rounded-lg border border-zinc-800 bg-zinc-800/50 p-4">
								<p class="mb-3 text-xs text-zinc-500">
									Set the scope for search indexing. Only content within this path will be searchable.
								</p>
								<p class="mb-3 rounded bg-amber-900/20 px-3 py-2 text-xs text-amber-400">
									<strong>Tip:</strong> Use a "By Folder" path for indexing (e.g., "Music/By Folder") to avoid 
									media server reorganization that can break playback.
								</p>
								
								<!-- Current path display -->
								<div class="mb-3 flex items-center gap-2">
									<FolderOpen class="h-4 w-4 shrink-0 text-zinc-400" />
									<div class="flex flex-wrap items-center gap-1 text-sm">
										<button
											class="rounded px-1.5 py-0.5 text-zinc-300 hover:bg-zinc-700"
											onclick={() => navigateIndexContainer('')}
										>
											Root
										</button>
										{#each indexCurrentPath as segment, i}
											<ChevronRight class="h-3 w-3 text-zinc-600" />
											<button
												class="rounded px-1.5 py-0.5 text-zinc-300 hover:bg-zinc-700"
												onclick={() => navigateIndexContainer(indexCurrentPath.slice(0, i + 1).join('/'))}
											>
												{segment}
											</button>
										{/each}
									</div>
									{#if indexContainerPath}
										<button
											class="ml-auto text-xs text-zinc-500 hover:text-zinc-300"
											onclick={clearIndexContainer}
										>
											Clear
										</button>
									{/if}
								</div>

								<!-- Container list -->
								<div class="max-h-40 overflow-y-auto rounded border border-zinc-700 bg-zinc-800">
									{#if loadingIndexContainers}
										<div class="flex items-center justify-center py-4">
											<Loader2 class="h-5 w-5 animate-spin text-zinc-500" />
										</div>
									{:else if indexContainerItems.length === 0}
										<div class="px-3 py-4 text-center text-xs text-zinc-500">
											No containers found
										</div>
									{:else}
										{#each indexContainerItems as container}
											<button
												class="flex w-full items-center gap-2 px-3 py-2 text-left text-sm text-zinc-300 hover:bg-zinc-700"
												onclick={() => selectIndexContainer(container)}
											>
												<FolderOpen class="h-4 w-4 shrink-0 text-zinc-500" />
												{container.name}
												<ChevronRight class="ml-auto h-4 w-4 text-zinc-600" />
											</button>
										{/each}
									{/if}
								</div>

								{#if indexContainerPath}
									<div class="mt-2 flex items-center gap-2 rounded bg-green-900/20 px-3 py-2 text-xs text-green-400">
										<span>Index scope set to:</span>
										<span class="font-medium">{indexContainerPath}</span>
									</div>
								{/if}
							</div>
						</div>
					{/if}

					<!-- Save Button -->
					{#if upnpChanged}
						<div class="flex justify-end">
							<button
								class="flex items-center gap-1.5 rounded bg-green-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-green-500 disabled:opacity-50"
								onclick={saveUPnPSettings}
								disabled={saving}
							>
								{#if saving}
									<Loader2 class="h-4 w-4 animate-spin" />
								{:else}
									<Save class="h-4 w-4" />
								{/if}
								Save Media Settings
							</button>
						</div>
					{/if}
				</div>
			{:else if activeTab === 'about'}
				<!-- About Tab -->
				<div class="space-y-6">
					<div class="text-center">
						<div class="mb-4 inline-flex h-16 w-16 items-center justify-center rounded-full bg-zinc-800">
							<Speaker class="h-8 w-8 text-green-500" />
						</div>
						<h3 class="text-xl font-semibold text-white">kefw2ui</h3>
						<p class="text-sm text-zinc-400">Web interface for KEF W2 speakers</p>
						{#if appVersion}
							<p class="mt-1 text-xs text-zinc-500">Version {appVersion}</p>
						{/if}
					</div>

					<div class="rounded-lg border border-zinc-800 bg-zinc-800/50 p-4">
						<h4 class="mb-2 text-sm font-medium text-zinc-300">Supported Speakers</h4>
						<ul class="space-y-1 text-sm text-zinc-400">
							<li>KEF LS50 Wireless II</li>
							<li>KEF LSX II / LSX II LT</li>
							<li>KEF LS60 Wireless</li>
						</ul>
					</div>

					<div class="rounded-lg border border-zinc-800 bg-zinc-800/50 p-4">
						<h4 class="mb-2 text-sm font-medium text-zinc-300">Links</h4>
						<div class="space-y-2 text-sm">
							<a
								href="https://github.com/hilli/go-kef-w2"
								target="_blank"
								rel="noopener noreferrer"
								class="block text-green-400 hover:text-green-300"
							>
								GitHub Repository
							</a>
						</div>
					</div>
				</div>
			{/if}
		</div>
	</div>
{/if}
