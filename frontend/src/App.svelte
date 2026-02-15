<script lang="ts">
  import { onMount } from 'svelte';
  import TabBar from './components/TabBar.svelte';
  import Toolbar from './components/Toolbar.svelte';
  import PaneGrid from './components/PaneGrid.svelte';
  import Footer from './components/Footer.svelte';
  import Sidebar from './components/Sidebar.svelte';
  import LaunchDialog from './components/LaunchDialog.svelte';
  import ProjectDialog from './components/ProjectDialog.svelte';
  import SettingsDialog from './components/SettingsDialog.svelte';
  import CommandPalette from './components/CommandPalette.svelte';
  import { tabStore, activeTab, allTabs } from './stores/tabs';
  import { config } from './stores/config';
  import { applyTheme, applyAccentColor } from './stores/theme';
  import type { PaneMode } from './stores/tabs';
  import * as App from '../wailsjs/go/backend/App';
  import { EventsOn } from '../wailsjs/runtime/runtime';

  let showLaunchDialog = false;
  let showProjectDialog = false;
  let showSettingsDialog = false;
  let showCommandPalette = false;
  let showSidebar = false;
  let branch = '';
  let commitAgeMinutes = -1;

  const modeMap: Record<string, number> = { shell: 0, claude: 1, 'claude-yolo': 2 };
  const modeReverse: PaneMode[] = ['shell', 'claude', 'claude-yolo'];

  async function restoreSession(): Promise<boolean> {
    try {
      const saved = await App.LoadTabs();
      if (!saved || !saved.tabs || saved.tabs.length === 0) return false;

      const cfg = $config;
      const claudeCmd = cfg.claude_command || 'claude';

      for (const savedTab of saved.tabs) {
        const tabId = tabStore.addTab(savedTab.name, savedTab.dir);

        for (const savedPane of savedTab.panes) {
          const mode = modeReverse[savedPane.mode] || 'shell';
          let argv: string[] = [];
          let name = savedPane.name;

          if (mode === 'claude') {
            argv = savedPane.model
              ? [claudeCmd, '--model', savedPane.model]
              : [claudeCmd];
          } else if (mode === 'claude-yolo') {
            argv = savedPane.model
              ? [claudeCmd, '--dangerously-skip-permissions', '--model', savedPane.model]
              : [claudeCmd, '--dangerously-skip-permissions'];
          }

          try {
            const sessionId = await App.CreateSession(argv, savedTab.dir || '', 24, 80);
            if (sessionId > 0) {
              tabStore.addPane(tabId, sessionId, name, mode, savedPane.model || '');
            }
          } catch (err) {
            console.error('[restoreSession] failed to create session:', err);
          }
        }
      }

      // Activate saved tab
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
        mode: modeMap[pane.mode] ?? 0,
        model: pane.model || '',
      })),
    }));

    const payload = { active_tab: Math.max(activeIdx, 0), tabs };
    console.log('[saveSession]', JSON.stringify(payload));
    App.SaveTabs(payload);
  }

  onMount(async () => {
    // Load config from Go backend
    try {
      const cfg = await App.GetConfig();
      config.set(cfg);
      applyTheme(cfg.theme || 'dark');
      if (cfg.terminal_color) {
        applyAccentColor(cfg.terminal_color);
      }
    } catch {
      applyTheme('dark');
    }

    // Try restoring previous session, otherwise create default tab
    const restored = await restoreSession();
    if (!restored) {
      let workDir = '';
      try {
        workDir = await App.GetWorkingDir();
      } catch {}
      tabStore.addTab('Workspace', workDir);
    }

    // Listen for activity updates
    EventsOn('terminal:activity', (info: any) => {
      tabStore.updateActivity(info.id, info.activity, info.cost);
    });

    // Listen for session exits
    EventsOn('terminal:exit', (id: number) => {
      tabStore.markExited(id);
    });

    // Listen for session errors
    EventsOn('terminal:error', (id: number, msg: string) => {
      console.error('[terminal:error]', id, msg);
      alert(`Terminal-Fehler (Session ${id}): ${msg}`);
    });

    // Auto-save session periodically (debounced via store subscription)
    let saveTimer: ReturnType<typeof setTimeout> | null = null;
    tabStore.subscribe(() => {
      if (saveTimer) clearTimeout(saveTimer);
      saveTimer = setTimeout(saveSession, 1000);
    });

    // Also try beforeunload as fallback
    window.addEventListener('beforeunload', saveSession);

    // Update git branch and commit age periodically
    updateBranch();
    updateCommitAge();
    setInterval(updateBranch, 10000);
    setInterval(updateCommitAge, 30000);

    // Keyboard shortcuts
    document.addEventListener('keydown', handleGlobalKeydown);
  });

  async function updateBranch() {
    const tab = $activeTab;
    if (!tab) return;
    try {
      branch = await App.GetGitBranch(tab.dir || '.');
    } catch {
      branch = '';
    }
  }

  // Refresh branch + commit age instantly when switching tabs
  $: if ($activeTab) {
    updateBranch();
    updateCommitAge();
  }

  async function updateCommitAge() {
    const tab = $activeTab;
    if (!tab) return;
    try {
      const ts = await App.GetLastCommitTime(tab.dir || '.');
      if (ts > 0) {
        const nowSec = Math.floor(Date.now() / 1000);
        commitAgeMinutes = Math.floor((nowSec - ts) / 60);
      } else {
        commitAgeMinutes = -1;
      }
    } catch {
      commitAgeMinutes = -1;
    }
  }

  async function handleLaunch(e: CustomEvent<{ type: PaneMode; model: string }>) {
    const { type, model } = e.detail;
    showLaunchDialog = false;

    const tab = $activeTab;
    if (!tab) {
      console.error('[handleLaunch] no active tab');
      return;
    }

    if (tab.panes.length >= MAX_PANES_PER_TAB) {
      alert(`Max. ${MAX_PANES_PER_TAB} Terminals pro Tab erreicht.`);
      return;
    }

    const claudeCmd = $config.claude_command || 'claude';
    let argv: string[];
    let name: string;

    switch (type) {
      case 'claude':
        argv = model ? [claudeCmd, '--model', model] : [claudeCmd];
        name = `Claude ${model ? `(${model})` : ''}`.trim();
        break;
      case 'claude-yolo':
        argv = model
          ? [claudeCmd, '--dangerously-skip-permissions', '--model', model]
          : [claudeCmd, '--dangerously-skip-permissions'];
        name = `YOLO ${model ? `(${model})` : ''}`.trim();
        break;
      default:
        argv = [];
        name = 'Shell';
        break;
    }

    try {
      console.log('[handleLaunch] creating session:', { type, model, argv, dir: tab.dir });
      const sessionId = await App.CreateSession(argv, tab.dir || '', 24, 80);
      console.log('[handleLaunch] session created:', sessionId);
      if (sessionId > 0) {
        tabStore.addPane(tab.id, sessionId, name, type, model);
      } else {
        console.error('[handleLaunch] CreateSession returned invalid id:', sessionId);
      }
    } catch (err) {
      console.error('[handleLaunch] CreateSession failed:', err);
    }
  }

  function handleClosePane(e: CustomEvent<{ paneId: string; sessionId: number }>) {
    const tab = $activeTab;
    if (!tab) return;
    const pane = tab.panes.find((p) => p.id === e.detail.paneId);
    const name = pane?.name || 'Terminal';
    if (!confirm(`"${name}" wirklich schließen?`)) return;
    App.CloseSession(e.detail.sessionId);
    tabStore.closePane(tab.id, e.detail.paneId);
  }

  function handleMaximizePane(e: CustomEvent<{ paneId: string }>) {
    const tab = $activeTab;
    if (!tab) return;
    tabStore.toggleMaximize(tab.id, e.detail.paneId);
  }

  function handleFocusPane(e: CustomEvent<{ paneId: string }>) {
    const tab = $activeTab;
    if (!tab) return;
    tabStore.focusPane(tab.id, e.detail.paneId);
  }

  function handleRenamePane(e: CustomEvent<{ paneId: string; name: string }>) {
    const tab = $activeTab;
    if (!tab) return;
    tabStore.renamePane(tab.id, e.detail.paneId, e.detail.name);
  }

  function handleSendCommand(e: CustomEvent<{ text: string }>) {
    const tab = $activeTab;
    if (!tab) return;
    const focusedPane = tab.panes.find((p) => p.focused);
    if (focusedPane) {
      const encoder = new TextEncoder();
      const bytes = encoder.encode(e.detail.text + '\n');
      let binary = '';
      for (let i = 0; i < bytes.length; i++) {
        binary += String.fromCharCode(bytes[i]);
      }
      App.WriteToSession(focusedPane.sessionId, btoa(binary));
    }
    showCommandPalette = false;
  }

  function handleSidebarFile(e: CustomEvent<{ path: string }>) {
    const tab = $activeTab;
    if (!tab) return;
    const focusedPane = tab.panes.find((p) => p.focused);
    if (focusedPane) {
      const pathStr = e.detail.path.includes(' ') ? `"${e.detail.path}"` : e.detail.path;
      const b64 = btoa(pathStr);
      App.WriteToSession(focusedPane.sessionId, b64);
    }
  }

  function handleGlobalKeydown(e: KeyboardEvent) {
    if (e.ctrlKey && e.key === 'n') {
      e.preventDefault();
      if ($activeTab && $activeTab.panes.length >= MAX_PANES_PER_TAB) return;
      showLaunchDialog = true;
      return;
    }
    if (e.ctrlKey && e.key === 't') {
      e.preventDefault();
      showProjectDialog = true;
      return;
    }
    if (e.ctrlKey && e.key === 'w') {
      e.preventDefault();
      if ($activeTab) tabStore.closeTab($activeTab.id);
      return;
    }
    if (e.ctrlKey && e.key === 'b') {
      e.preventDefault();
      showSidebar = !showSidebar;
      return;
    }
    if (e.ctrlKey && e.key === 'z') {
      e.preventDefault();
      const tab = $activeTab;
      if (tab?.focusedPaneId) {
        tabStore.toggleMaximize(tab.id, tab.focusedPaneId);
      }
      return;
    }
  }

  $: totalCost = (() => {
    let sum = 0;
    for (const tab of $allTabs) {
      for (const pane of tab.panes) {
        if (pane.cost) {
          const val = parseFloat(pane.cost.replace('$', ''));
          if (!isNaN(val)) sum += val;
        }
      }
    }
    return sum > 0 ? `$${sum.toFixed(2)}` : '';
  })();

  const MAX_PANES_PER_TAB = 10;

  $: totalPanes = $allTabs.reduce((sum, t) => sum + t.panes.length, 0);
  $: currentPanes = $activeTab?.panes.length ?? 0;
  $: canChangeDir = currentPanes === 0;
  $: tabInfo = `Tab ${($allTabs.findIndex((t) => t.id === $activeTab?.id) ?? 0) + 1}/${$allTabs.length}  Pane ${currentPanes}/${MAX_PANES_PER_TAB}`;

  async function handleChangeDir() {
    const tab = $activeTab;
    if (!tab || tab.panes.length > 0) return;
    try {
      const dir = await App.SelectDirectory(tab.dir);
      if (dir) {
        tabStore.setTabDir(tab.id, dir);
      }
    } catch (err) {
      console.error('[handleChangeDir]', err);
    }
  }

  function handleProjectCreate(e: CustomEvent<{ name: string; dir: string }>) {
    const { name, dir } = e.detail;
    tabStore.addTab(name, dir);
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
    <Sidebar
      visible={showSidebar}
      dir={$activeTab?.dir ?? ''}
      on:close={() => (showSidebar = false)}
      on:selectFile={handleSidebarFile}
    />
    <PaneGrid
      panes={$activeTab?.panes ?? []}
      on:closePane={handleClosePane}
      on:maximizePane={handleMaximizePane}
      on:focusPane={handleFocusPane}
      on:renamePane={handleRenamePane}
    />
  </div>

  <Footer
    {branch}
    {totalCost}
    {tabInfo}
    {commitAgeMinutes}
  />

  <LaunchDialog
    visible={showLaunchDialog}
    on:launch={handleLaunch}
    on:close={() => (showLaunchDialog = false)}
  />

  <ProjectDialog
    visible={showProjectDialog}
    on:create={handleProjectCreate}
    on:close={() => (showProjectDialog = false)}
  />

  <SettingsDialog
    visible={showSettingsDialog}
    on:close={() => (showSettingsDialog = false)}
  />

  <CommandPalette
    visible={showCommandPalette}
    on:send={handleSendCommand}
    on:close={() => (showCommandPalette = false)}
  />
</div>

<style>
  :global(*) {
    margin: 0;
    padding: 0;
    box-sizing: border-box;
  }

  :global(body) {
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
    background: var(--bg);
    color: var(--fg);
    overflow: hidden;
  }

  .app {
    display: flex;
    flex-direction: column;
    height: 100vh;
    overflow: hidden;
  }

  .content {
    display: flex;
    flex: 1;
    overflow: hidden;
  }
</style>
