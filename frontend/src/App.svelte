<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import TabBar from './components/TabBar.svelte';
  import Toolbar from './components/Toolbar.svelte';
  import PaneGrid from './components/PaneGrid.svelte';
  import Footer from './components/Footer.svelte';
  import Sidebar from './components/Sidebar.svelte';
  import LaunchDialog from './components/LaunchDialog.svelte';
  import ProjectDialog from './components/ProjectDialog.svelte';
  import SettingsDialog from './components/SettingsDialog.svelte';
  import CommandPalette from './components/CommandPalette.svelte';
  import CrashDialog from './components/CrashDialog.svelte';
  import { tabStore, activeTab, allTabs } from './stores/tabs';
  import { config } from './stores/config';
  import { applyTheme, applyAccentColor } from './stores/theme';
  import type { PaneMode } from './stores/tabs';
  import { MODE_TO_INDEX, INDEX_TO_MODE, buildClaudeArgv, getClaudeName, encodeForPty } from './lib/claude';
  import { createGlobalKeyHandler } from './lib/shortcuts';
  import * as App from '../wailsjs/go/backend/App';
  import { EventsOn } from '../wailsjs/runtime/runtime';

  const MAX_PANES_PER_TAB = 10;

  let showLaunchDialog = false;
  let showProjectDialog = false;
  let showSettingsDialog = false;
  let showCommandPalette = false;
  let showSidebar = false;
  let showCrashDialog = false;
  let branch = '';
  let commitAgeMinutes = -1;

  let branchInterval: ReturnType<typeof setInterval> | null = null;
  let commitAgeInterval: ReturnType<typeof setInterval> | null = null;
  let storeUnsubscribe: (() => void) | null = null;

  async function restoreSession(): Promise<boolean> {
    try {
      const saved = await App.LoadTabs();
      if (!saved || !saved.tabs || saved.tabs.length === 0) return false;

      const claudeCmd = $config.claude_command || 'claude';

      for (const savedTab of saved.tabs) {
        const tabId = tabStore.addTab(savedTab.name, savedTab.dir);
        for (const savedPane of savedTab.panes) {
          const mode = INDEX_TO_MODE[savedPane.mode] || 'shell';
          const argv = buildClaudeArgv(mode, savedPane.model || '', claudeCmd);
          try {
            const sessionId = await App.CreateSession(argv, savedTab.dir || '', 24, 80);
            if (sessionId > 0) tabStore.addPane(tabId, sessionId, savedPane.name, mode, savedPane.model || '');
          } catch (err) {
            console.error('[restoreSession] failed to create session:', err);
          }
        }
      }

      const state = tabStore.getState();
      if (saved.active_tab >= 0 && saved.active_tab < state.tabs.length) {
        tabStore.setActiveTab(state.tabs[saved.active_tab].id);
      }
      return true;
    } catch (err) {
      console.error('[restoreSession]', err);
      return false;
    }
  }

  function saveSession() {
    const state = tabStore.getState();
    if (!state.tabs.length) return;
    const activeIdx = state.tabs.findIndex((t) => t.id === state.activeTabId);
    const tabs = state.tabs.map((tab) => ({
      name: tab.name,
      dir: tab.dir,
      focus_idx: tab.panes.findIndex((p) => p.focused),
      panes: tab.panes.map((pane) => ({
        name: pane.name,
        mode: MODE_TO_INDEX[pane.mode] ?? 0,
        model: pane.model || '',
      })),
    }));
    App.SaveTabs({ active_tab: Math.max(activeIdx, 0), tabs });
  }

  const handleGlobalKeydown = createGlobalKeyHandler({
    onNewPane: () => { showLaunchDialog = true; },
    onNewTab: () => { showProjectDialog = true; },
    onCloseTab: () => { if ($activeTab) tabStore.closeTab($activeTab.id); },
    onToggleSidebar: () => { showSidebar = !showSidebar; },
    onToggleMaximize: () => {
      const tab = $activeTab;
      if (tab?.focusedPaneId) tabStore.toggleMaximize(tab.id, tab.focusedPaneId);
    },
    onFocusPane: (idx) => {
      const tab = $activeTab;
      if (tab && idx < tab.panes.length) tabStore.focusPane(tab.id, tab.panes[idx].id);
    },
    canAddPane: () => ($activeTab?.panes.length ?? 0) < MAX_PANES_PER_TAB,
  });

  onMount(async () => {
    try {
      const cfg = await App.GetConfig();
      config.set(cfg);
      applyTheme(cfg.theme || 'dark');
      if (cfg.terminal_color) applyAccentColor(cfg.terminal_color);
    } catch { applyTheme('dark'); }

    try {
      const health = await App.CheckHealth();
      if (health.crash_detected && !health.logging_enabled) showCrashDialog = true;
    } catch {}

    const restored = await restoreSession();
    if (!restored) {
      let workDir = '';
      try { workDir = await App.GetWorkingDir(); } catch {}
      tabStore.addTab('Workspace', workDir);
    }

    EventsOn('terminal:activity', (info: any) => tabStore.updateActivity(info.id, info.activity, info.cost));
    EventsOn('terminal:exit', (id: number) => tabStore.markExited(id));
    EventsOn('terminal:error', (id: number, msg: string) => {
      console.error('[terminal:error]', id, msg);
      alert(`Terminal-Fehler (Session ${id}): ${msg}`);
    });

    let saveTimer: ReturnType<typeof setTimeout> | null = null;
    storeUnsubscribe = tabStore.subscribe(() => {
      if (saveTimer) clearTimeout(saveTimer);
      saveTimer = setTimeout(saveSession, 1000);
    });

    window.addEventListener('beforeunload', saveSession);
    updateBranch();
    updateCommitAge();
    branchInterval = setInterval(updateBranch, 10000);
    commitAgeInterval = setInterval(updateCommitAge, 30000);
    document.addEventListener('keydown', handleGlobalKeydown);
  });

  onDestroy(() => {
    if (branchInterval) clearInterval(branchInterval);
    if (commitAgeInterval) clearInterval(commitAgeInterval);
    if (storeUnsubscribe) storeUnsubscribe();
    window.removeEventListener('beforeunload', saveSession);
    document.removeEventListener('keydown', handleGlobalKeydown);
  });

  async function updateBranch() {
    const tab = $activeTab;
    if (!tab) return;
    try { branch = await App.GetGitBranch(tab.dir || '.'); } catch { branch = ''; }
  }

  $: if ($activeTab) { updateBranch(); updateCommitAge(); }

  async function updateCommitAge() {
    const tab = $activeTab;
    if (!tab) return;
    try {
      const ts = await App.GetLastCommitTime(tab.dir || '.');
      commitAgeMinutes = ts > 0 ? Math.floor((Math.floor(Date.now() / 1000) - ts) / 60) : -1;
    } catch { commitAgeMinutes = -1; }
  }

  async function handleLaunch(e: CustomEvent<{ type: PaneMode; model: string }>) {
    const { type, model } = e.detail;
    showLaunchDialog = false;
    const tab = $activeTab;
    if (!tab) return;
    if (tab.panes.length >= MAX_PANES_PER_TAB) {
      alert(`Max. ${MAX_PANES_PER_TAB} Terminals pro Tab erreicht.`);
      return;
    }
    const claudeCmd = $config.claude_command || 'claude';
    const argv = buildClaudeArgv(type, model, claudeCmd);
    const name = getClaudeName(type, model);
    try {
      const sessionId = await App.CreateSession(argv, tab.dir || '', 24, 80);
      if (sessionId > 0) tabStore.addPane(tab.id, sessionId, name, type, model);
    } catch (err) { console.error('[handleLaunch] CreateSession failed:', err); }
  }

  function handleClosePane(e: CustomEvent<{ paneId: string; sessionId: number }>) {
    const tab = $activeTab;
    if (!tab) return;
    const pane = tab.panes.find((p) => p.id === e.detail.paneId);
    if (!confirm(`"${pane?.name || 'Terminal'}" wirklich schließen?`)) return;
    App.CloseSession(e.detail.sessionId);
    tabStore.closePane(tab.id, e.detail.paneId);
  }

  function handleMaximizePane(e: CustomEvent<{ paneId: string }>) {
    const tab = $activeTab;
    if (tab) tabStore.toggleMaximize(tab.id, e.detail.paneId);
  }

  function handleFocusPane(e: CustomEvent<{ paneId: string }>) {
    const tab = $activeTab;
    if (tab) tabStore.focusPane(tab.id, e.detail.paneId);
  }

  function handleRenamePane(e: CustomEvent<{ paneId: string; name: string }>) {
    const tab = $activeTab;
    if (tab) tabStore.renamePane(tab.id, e.detail.paneId, e.detail.name);
  }

  async function handleRestartPane(e: CustomEvent<{ paneId: string; sessionId: number; mode: PaneMode; model: string; name: string }>) {
    const tab = $activeTab;
    if (!tab) return;
    const { paneId, sessionId, mode, model, name } = e.detail;
    App.CloseSession(sessionId);
    tabStore.closePane(tab.id, paneId);
    const claudeCmd = $config.claude_command || 'claude';
    const argv = buildClaudeArgv(mode, model, claudeCmd);
    try {
      const newSessionId = await App.CreateSession(argv, tab.dir || '', 24, 80);
      if (newSessionId > 0) tabStore.addPane(tab.id, newSessionId, name, mode, model);
    } catch (err) { console.error('[handleRestartPane] failed:', err); }
  }

  function handleSendCommand(e: CustomEvent<{ text: string }>) {
    const tab = $activeTab;
    if (!tab) return;
    const focusedPane = tab.panes.find((p) => p.focused);
    if (focusedPane) App.WriteToSession(focusedPane.sessionId, encodeForPty(e.detail.text + '\n'));
    showCommandPalette = false;
  }

  function handleSidebarFile(e: CustomEvent<{ path: string }>) {
    const tab = $activeTab;
    if (!tab) return;
    const focusedPane = tab.panes.find((p) => p.focused);
    if (focusedPane) {
      const pathStr = e.detail.path.includes(' ') ? `"${e.detail.path}"` : e.detail.path;
      App.WriteToSession(focusedPane.sessionId, btoa(pathStr));
    }
  }

  $: totalCost = (() => {
    let sum = 0;
    for (const tab of $allTabs) {
      for (const pane of tab.panes) {
        if (pane.cost) { const val = parseFloat(pane.cost.replace('$', '')); if (!isNaN(val)) sum += val; }
      }
    }
    return sum > 0 ? `$${sum.toFixed(2)}` : '';
  })();

  $: totalPanes = $allTabs.reduce((sum, t) => sum + t.panes.length, 0);
  $: currentPanes = $activeTab?.panes.length ?? 0;
  $: canChangeDir = currentPanes === 0;
  $: tabInfo = `Tab ${($allTabs.findIndex((t) => t.id === $activeTab?.id) ?? 0) + 1}/${$allTabs.length}  Pane ${currentPanes}/${MAX_PANES_PER_TAB}`;

  async function handleChangeDir() {
    const tab = $activeTab;
    if (!tab || tab.panes.length > 0) return;
    try {
      const dir = await App.SelectDirectory(tab.dir);
      if (dir) tabStore.setTabDir(tab.id, dir);
    } catch (err) { console.error('[handleChangeDir]', err); }
  }

  function handleProjectCreate(e: CustomEvent<{ name: string; dir: string }>) {
    tabStore.addTab(e.detail.name, e.detail.dir);
  }

  function handleCrashEnable() {
    showCrashDialog = false;
    App.EnableLogging(true);
    config.update(c => ({ ...c, logging_enabled: true }));
  }
</script>

<div class="app">
  <TabBar activeTabId={$activeTab?.id ?? ''} on:addTab={() => (showProjectDialog = true)} />
  <Toolbar
    paneCount={currentPanes}
    maxPanes={MAX_PANES_PER_TAB}
    tabDir={$activeTab?.dir ?? ''}
    {canChangeDir}
    on:newTerminal={() => (showLaunchDialog = true)}
    on:toggleSidebar={() => (showSidebar = !showSidebar)}
    on:changeDir={handleChangeDir}
    on:openSettings={() => (showSettingsDialog = true)}
    on:openCommands={() => (showCommandPalette = true)}
  />

  <div class="content">
    <Sidebar visible={showSidebar} dir={$activeTab?.dir ?? ''} on:close={() => (showSidebar = false)} on:selectFile={handleSidebarFile} />
    <PaneGrid
      panes={$activeTab?.panes ?? []}
      on:closePane={handleClosePane}
      on:maximizePane={handleMaximizePane}
      on:focusPane={handleFocusPane}
      on:renamePane={handleRenamePane}
      on:restartPane={handleRestartPane}
    />
  </div>

  <Footer {branch} {totalCost} {tabInfo} {commitAgeMinutes} />
  <LaunchDialog visible={showLaunchDialog} on:launch={handleLaunch} on:close={() => (showLaunchDialog = false)} />
  <ProjectDialog visible={showProjectDialog} on:create={handleProjectCreate} on:close={() => (showProjectDialog = false)} />
  <SettingsDialog visible={showSettingsDialog} on:close={() => (showSettingsDialog = false)} />
  <CommandPalette visible={showCommandPalette} on:send={handleSendCommand} on:close={() => (showCommandPalette = false)} />
  <CrashDialog visible={showCrashDialog} on:enable={handleCrashEnable} on:dismiss={() => (showCrashDialog = false)} />
</div>

<style>
  :global(*) { margin: 0; padding: 0; box-sizing: border-box; }
  :global(body) {
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
    background: var(--bg); color: var(--fg); overflow: hidden;
  }
  .app { display: flex; flex-direction: column; height: 100vh; overflow: hidden; }
  .content { display: flex; flex: 1; overflow: hidden; }
</style>
