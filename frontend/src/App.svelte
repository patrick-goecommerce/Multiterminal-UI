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
  import IssueDialog from './components/IssueDialog.svelte';
  import BranchConflictDialog from './components/BranchConflictDialog.svelte';
  import FilePreview from './components/FilePreview.svelte';
  import { tabStore, activeTab, allTabs } from './stores/tabs';
  import { config } from './stores/config';
  import { applyTheme, applyAccentColor } from './stores/theme';
  import type { PaneMode } from './stores/tabs';
  import { buildClaudeArgv, getClaudeName, encodeForPty } from './lib/claude';
  import { createGlobalKeyHandler } from './lib/shortcuts';
  import { sendNotification } from './lib/notifications';
  import { restoreSession, saveSession } from './lib/session';
  import { fetchBranch, fetchCommitAge, fetchConflicts, fetchIssueCount } from './lib/git-polling';
  import { buildIssuePrompt, setupIssueBranch, resolveBranchConflict } from './lib/launch';
  import type { IssueContext } from './lib/launch';
  import * as App from '../wailsjs/go/backend/App';
  import { EventsOn } from '../wailsjs/runtime/runtime';

  const MAX_PANES_PER_TAB = 10;

  let showLaunchDialog = false;
  let showProjectDialog = false;
  let showSettingsDialog = false;
  let showCommandPalette = false;
  let showSidebar = false;
  let showCrashDialog = false;
  let showIssueDialog = false;
  let previewFilePath = '';
  let editIssueData: { number: number; title: string; body: string; labels: string[]; state: string } | null = null;
  let launchIssueContext: { number: number; title: string; body: string; labels: string[] } | null = null;
  let issueCount = 0;
  let sidebarView: 'explorer' | 'source-control' | 'issues' = 'explorer';
  let branch = '';
  let commitAgeMinutes = -1;
  let updateAvailable = false;
  let latestVersion = '';
  let downloadURL = '';

  let conflictCount = 0;
  let conflictFiles: string[] = [];
  let conflictOperation = '';
  let prevConflictCount = 0;

  let showBranchConflict = false;
  let branchConflictData: {
    currentBranch: string;
    currentIssueNumber: number;
    targetIssueNumber: number;
    targetIssueTitle: string;
    dirtyWorkingTree: boolean;
  } | null = null;
  let pendingLaunch: {
    type: PaneMode;
    model: string;
    issueCtx: { number: number; title: string; body: string; labels: string[] };
    name: string;
    argv: string[];
    sessionDir: string;
  } | null = null;

  let resolvedClaudePath = 'claude';
  let claudeDetected = true;

  let branchInterval: ReturnType<typeof setInterval> | null = null;
  let commitAgeInterval: ReturnType<typeof setInterval> | null = null;
  let storeUnsubscribe: (() => void) | null = null;

  const handleGlobalKeydown = createGlobalKeyHandler({
    onNewPane: () => { showLaunchDialog = true; },
    onNewTab: () => { showProjectDialog = true; },
    onCloseTab: () => { if ($activeTab) tabStore.closeTab($activeTab.id); },
    onToggleSidebar: () => { if ($config.sidebar_pinned && showSidebar) return; showSidebar = !showSidebar; },
    onOpenIssues: () => { showSidebar = true; sidebarView = 'issues'; },
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
      if (cfg.sidebar_pinned) showSidebar = true;
    } catch { applyTheme('dark'); }

    try {
      resolvedClaudePath = (await App.GetResolvedClaudePath()) || 'claude';
      claudeDetected = await App.IsClaudeDetected();
    } catch {
      resolvedClaudePath = 'claude';
      claudeDetected = false;
    }

    try {
      const health = await App.CheckHealth();
      if (health.crash_detected && !health.logging_enabled) showCrashDialog = true;
    } catch {}

    App.CheckForUpdates().then((info) => {
      if (info.updateAvailable) {
        updateAvailable = true;
        latestVersion = info.latestVersion;
        downloadURL = info.downloadURL;
      }
    }).catch(() => {});

    const restored = await restoreSession(resolvedClaudePath);
    if (!restored) {
      let workDir = '';
      try { workDir = await App.GetWorkingDir(); } catch {}
      tabStore.addTab('Workspace', workDir);
    }

    EventsOn('terminal:activity', (info: any) => {
      tabStore.updateActivity(info.id, info.activity, info.cost);
      // Notify when an issue-linked agent finishes (only when window is focused,
      // because TerminalPane already sends a notification when unfocused)
      if (info.activity === 'done' && document.hasFocus()) {
        for (const tab of $allTabs) {
          const pane = tab.panes.find(p => p.sessionId === info.id);
          if (pane?.issueNumber) {
            sendNotification(`Agent fertig – #${pane.issueNumber}`, pane.issueTitle || pane.name);
            break;
          }
        }
      }
    });
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
    updateIssueCount();
    updateConflicts();
    branchInterval = setInterval(() => { updateBranch(); updateConflicts(); }, 10000);
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
    branch = await fetchBranch(tab.dir || '.');
  }

  $: if ($activeTab) { updateBranch(); updateCommitAge(); updateIssueCount(); updateConflicts(); }

  async function updateCommitAge() {
    const tab = $activeTab;
    if (!tab) return;
    commitAgeMinutes = await fetchCommitAge(tab.dir || '.');
  }

  async function updateConflicts() {
    const tab = $activeTab;
    const info = await fetchConflicts(tab?.dir || '');
    if (prevConflictCount === 0 && info.count > 0) {
      const opLabel = info.operation
        ? ` (${info.operation.charAt(0).toUpperCase() + info.operation.slice(1)})`
        : '';
      sendNotification(
        `Merge-Konflikte erkannt${opLabel}`,
        `${info.count} Datei${info.count > 1 ? 'en' : ''} mit Konflikten`
      );
    }
    prevConflictCount = info.count;
    conflictCount = info.count;
    conflictFiles = info.files;
    conflictOperation = info.operation;
  }

  async function handleLaunch(e: CustomEvent<{ type: PaneMode; model: string; issue?: { number: number; title: string; body: string; labels: string[] } | null }>) {
    const { type, model, issue } = e.detail;
    showLaunchDialog = false;
    const issueCtx = issue || launchIssueContext;
    launchIssueContext = null;
    const tab = $activeTab;
    if (!tab) return;
    if (tab.panes.length >= MAX_PANES_PER_TAB) {
      alert(`Max. ${MAX_PANES_PER_TAB} Terminals pro Tab erreicht.`);
      return;
    }
    const claudeCmd = resolvedClaudePath;
    const argv = buildClaudeArgv(type, model, claudeCmd);
    const baseName = getClaudeName(type, model);
    const name = issueCtx ? `${baseName} – #${issueCtx.number}` : baseName;
    try {
      let issueBranch = '';
      let worktreePath = '';
      let sessionDir = tab.dir || '';

      if (issueCtx) {
        const result = await setupIssueBranch(
          sessionDir, issueCtx,
          $config.use_worktrees === true,
          $config.auto_branch_on_issue !== false,
        );
        if (result.cancelled) return;
        if (result.conflict) {
          branchConflictData = {
            currentBranch: result.conflict.currentBranch,
            currentIssueNumber: result.conflict.currentIssueNumber,
            targetIssueNumber: issueCtx.number,
            targetIssueTitle: issueCtx.title,
            dirtyWorkingTree: result.conflict.dirtyWorkingTree,
          };
          pendingLaunch = { type, model, issueCtx, name, argv, sessionDir };
          showBranchConflict = true;
          return;
        }
        issueBranch = result.issueBranch;
        worktreePath = result.worktreePath;
        sessionDir = result.sessionDir;
      }

      const sessionId = await App.CreateSession(argv, sessionDir, 24, 80);
      if (sessionId > 0) {
        tabStore.addPane(tab.id, sessionId, name, type, model, issueCtx?.number, issueCtx?.title, issueBranch, worktreePath);
        if (issueCtx) {
          App.LinkSessionIssue(sessionId, issueCtx.number, issueCtx.title, issueBranch, sessionDir);
          setTimeout(() => {
            const prompt = buildIssuePrompt(issueCtx);
            App.WriteToSession(sessionId, encodeForPty(prompt + '\n'));
          }, 1500);
        }
      }
    } catch (err) { console.error('[handleLaunch] CreateSession failed:', err); }
  }

  async function handleBranchConflictChoice(e: CustomEvent<{ action: 'switch' | 'stay' | 'worktree' }>) {
    showBranchConflict = false;
    const launch = pendingLaunch;
    pendingLaunch = null;
    branchConflictData = null;
    if (!launch) return;

    const tab = $activeTab;
    if (!tab) return;
    const { type, model, issueCtx, name, argv } = launch;

    try {
      const resolved = await resolveBranchConflict(e.detail.action, launch.sessionDir, issueCtx);
      if (resolved.cancelled) return;

      const sessionId = await App.CreateSession(argv, resolved.sessionDir, 24, 80);
      if (sessionId > 0) {
        tabStore.addPane(tab.id, sessionId, name, type, model, issueCtx.number, issueCtx.title, resolved.issueBranch, resolved.worktreePath);
        App.LinkSessionIssue(sessionId, issueCtx.number, issueCtx.title, resolved.issueBranch, resolved.sessionDir);
        setTimeout(() => {
          const prompt = buildIssuePrompt(issueCtx);
          App.WriteToSession(sessionId, encodeForPty(prompt + '\n'));
        }, 1500);
      }
    } catch (err) { console.error('[handleBranchConflictChoice] failed:', err); }
  }

  function handleLaunchForIssue(e: CustomEvent<{ number: number; title: string; body: string; labels: string[] }>) {
    launchIssueContext = e.detail;
    showLaunchDialog = true;
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
    const claudeCmd = resolvedClaudePath;
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

  function handleNavigateFile(e: CustomEvent<{ path: string }>) {
    showSidebar = true;
    sidebarView = 'explorer';
  }

  async function handleTogglePin() {
    const pinned = !$config.sidebar_pinned;
    config.update(c => ({ ...c, sidebar_pinned: pinned }));
    try { await App.SaveConfig({ ...$config, sidebar_pinned: pinned }); } catch {}
    if (!pinned && !showSidebar) return;
    if (pinned) showSidebar = true;
  }

  function handleSidebarFile(e: CustomEvent<{ path: string }>) {
    previewFilePath = e.detail.path;
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

  // Build a map of issue number -> activity/cost for all panes with linked issues
  $: paneIssues = (() => {
    const map: Record<number, { activity: string; cost: string }> = {};
    for (const tab of $allTabs) {
      for (const pane of tab.panes) {
        if (pane.issueNumber && pane.running) {
          map[pane.issueNumber] = { activity: pane.activity, cost: pane.cost };
        }
      }
    }
    return map;
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

  async function updateIssueCount() {
    const tab = $activeTab;
    issueCount = await fetchIssueCount(tab?.dir || '');
  }

  function handleCreateIssue() {
    editIssueData = null;
    showIssueDialog = true;
  }

  function handleEditIssue(e: CustomEvent<any>) {
    editIssueData = {
      number: e.detail.number,
      title: e.detail.title,
      body: e.detail.body || '',
      labels: e.detail.labels || [],
      state: e.detail.state || 'OPEN',
    };
    showIssueDialog = true;
  }

  function handleIssueSaved() {
    showIssueDialog = false;
    editIssueData = null;
    updateIssueCount();
  }

  async function handleIssueAction(e: CustomEvent<{ paneId: string; sessionId: number; issueNumber: number; action: string }>) {
    const { sessionId, issueNumber, action } = e.detail;
    const tab = $activeTab;
    if (!tab) return;
    const dir = tab.dir || '';

    if (action === 'commit') {
      const msg = `Closes #${issueNumber}`;
      App.WriteToSession(sessionId, encodeForPty(`git add -A && git commit -m '${msg}' && git push\n`));
    } else if (action === 'pr') {
      App.WriteToSession(sessionId, encodeForPty(`gh pr create --title "Closes #${issueNumber}" --body "Resolves #${issueNumber}" --fill\n`));
    } else if (action === 'closeIssue') {
      try {
        await App.UpdateIssue(dir, issueNumber, '', '', 'closed');
        updateIssueCount();
      } catch (err) { console.error('[handleIssueAction] close failed:', err); }
    }
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
    on:toggleSidebar={() => { if ($config.sidebar_pinned && showSidebar) return; showSidebar = !showSidebar; }}
    on:changeDir={handleChangeDir}
    on:openSettings={() => (showSettingsDialog = true)}
    on:openCommands={() => (showCommandPalette = true)}
  />

  <div class="content">
    <Sidebar visible={showSidebar} dir={$activeTab?.dir ?? ''} {issueCount} {paneIssues} {conflictFiles} {conflictOperation} initialView={sidebarView} pinned={$config.sidebar_pinned} on:close={() => { if (!$config.sidebar_pinned) showSidebar = false; }} on:togglePin={handleTogglePin} on:selectFile={handleSidebarFile} on:createIssue={handleCreateIssue} on:editIssue={handleEditIssue} on:launchForIssue={handleLaunchForIssue} />
    <div class="tab-layers">
      {#each $allTabs as tab (tab.id)}
        <div class="tab-layer" class:active={tab.id === $activeTab?.id}>
          <PaneGrid
            tabId={tab.id}
            panes={tab.panes}
            active={tab.id === $activeTab?.id}
            on:closePane={handleClosePane}
            on:maximizePane={handleMaximizePane}
            on:focusPane={handleFocusPane}
            on:renamePane={handleRenamePane}
            on:restartPane={handleRestartPane}
            on:issueAction={handleIssueAction}
            on:navigateFile={handleNavigateFile}
            on:splitPane={() => (showLaunchDialog = true)}
          />
        </div>
      {/each}
    </div>
  </div>

  <Footer {branch} {totalCost} {tabInfo} {commitAgeMinutes} {conflictCount} {conflictOperation} {updateAvailable} {latestVersion} {downloadURL} />
  <LaunchDialog visible={showLaunchDialog} issueContext={launchIssueContext} {claudeDetected} on:launch={handleLaunch} on:openSettings={() => { showLaunchDialog = false; showSettingsDialog = true; }} on:close={() => { showLaunchDialog = false; launchIssueContext = null; }} />
  <ProjectDialog visible={showProjectDialog} on:create={handleProjectCreate} on:close={() => (showProjectDialog = false)} />
  <SettingsDialog visible={showSettingsDialog} on:close={() => (showSettingsDialog = false)} on:saved={async () => { try { resolvedClaudePath = (await App.GetResolvedClaudePath()) || 'claude'; claudeDetected = await App.IsClaudeDetected(); } catch {} }} />
  <CommandPalette visible={showCommandPalette} on:send={handleSendCommand} on:close={() => (showCommandPalette = false)} />
  <CrashDialog visible={showCrashDialog} on:enable={handleCrashEnable} on:dismiss={() => (showCrashDialog = false)} />
  <IssueDialog visible={showIssueDialog} dir={$activeTab?.dir ?? ''} editIssue={editIssueData} on:saved={handleIssueSaved} on:close={() => { showIssueDialog = false; editIssueData = null; }} />
  <BranchConflictDialog
    visible={showBranchConflict}
    currentBranch={branchConflictData?.currentBranch ?? ''}
    currentIssueNumber={branchConflictData?.currentIssueNumber ?? 0}
    targetIssueNumber={branchConflictData?.targetIssueNumber ?? 0}
    targetIssueTitle={branchConflictData?.targetIssueTitle ?? ''}
    dirtyWorkingTree={branchConflictData?.dirtyWorkingTree ?? false}
    on:choose={handleBranchConflictChoice}
    on:close={() => { showBranchConflict = false; pendingLaunch = null; branchConflictData = null; }}
  />
  <FilePreview visible={!!previewFilePath} filePath={previewFilePath} on:close={() => (previewFilePath = '')} />
</div>

<style>
  :global(*) { margin: 0; padding: 0; box-sizing: border-box; }
  :global(body) {
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
    background: var(--bg); color: var(--fg); overflow: hidden;
  }
  .app { display: flex; flex-direction: column; height: 100vh; overflow: hidden; }
  .content { display: flex; flex: 1; overflow: hidden; }
  .tab-layers { position: relative; flex: 1; overflow: hidden; }
  .tab-layer {
    position: absolute; inset: 0;
    display: flex; flex-direction: column;
    visibility: hidden; pointer-events: none;
  }
  .tab-layer.active { visibility: visible; pointer-events: auto; }
</style>
