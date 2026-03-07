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
  import DashboardView from './components/DashboardView.svelte';
  import SetupDialog from './components/SetupDialog.svelte';
  import LeftNav from './components/LeftNav.svelte';
  import SkillPicker from './components/SkillPicker.svelte';
  import { get } from 'svelte/store';
  import { tabStore, activeTab, allTabs } from './stores/tabs';
  import { workspace } from './stores/workspace';
  import { config } from './stores/config';
  import { applyTheme, applyAccentColor } from './stores/theme';
  import { initI18n, setLanguage, t, type Language } from './stores/i18n';
  import type { PaneMode } from './stores/tabs';
  import { buildClaudeArgv, getClaudeName, encodeForPty } from './lib/claude';
  import { getWindowId, isMainWindow, getInitialTabs } from './lib/window';
  import { createGlobalKeyHandler } from './lib/shortcuts';
  import { sendNotification } from './lib/notifications';
  import { restoreSession, saveSession } from './lib/session';
  import { startKeepAliveLoop } from './lib/keepalive';
  import { fetchBranch, fetchCommitAge, fetchConflicts, fetchIssueCount } from './lib/git-polling';
  import { checkForNewCommit } from './lib/background-agents';
  import { buildIssuePrompt, setupIssueBranch, resolveBranchConflict } from './lib/launch';
  import type { IssueContext } from './lib/launch';
  import * as App from '../wailsjs/go/backend/App';
  import { EventsOn } from '../wailsjs/runtime/runtime';

  const MAX_PANES_PER_TAB = 10;

  const _windowId = getWindowId();
  const _isMain = isMainWindow();
  // TODO: use _initialTabs to populate secondary window tabs (pending implementation)
  const _initialTabs = getInitialTabs();

  let showLaunchDialog = false;
  let showProjectDialog = false;
  let showSettingsDialog = false;
  let showCommandPalette = false;
  let showSidebar = false;
  let showCrashDialog = false;
  let showSetupDialog = false;
  let showIssueDialog = false;
  let showDashboard = false;
  let showSkillPicker = false;
  let skillPickerDir = '';
  let previewFilePath = '';
  let editIssueData: { number: number; title: string; body: string; labels: string[]; state: string } | null = null;
  let launchIssueContext: { number: number; title: string; body: string; labels: string[] } | null = null;
  let issueCount = 0;
  let sidebarView: 'explorer' | 'source-control' | 'issues' = 'explorer';
  let branch = '';
  let allWorktrees: any[] = [];
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
  let resolvedCodexPath = 'codex';
  let codexDetected = false;
  let resolvedGeminiPath = 'gemini';
  let geminiDetected = false;

  let branchInterval: ReturnType<typeof setInterval> | null = null;
  let commitAgeInterval: ReturnType<typeof setInterval> | null = null;
  let storeUnsubscribe: (() => void) | null = null;
  let keepAliveCleanup: (() => void) | null = null;

  const handleGlobalKeydown = createGlobalKeyHandler({
    onNewPane: () => { showLaunchDialog = true; },
    onNewTab: () => { showProjectDialog = true; },
    onCloseTab: () => { if ($activeTab) tabStore.closeTab($activeTab.id); },
    onToggleSidebar: () => workspace.toggleSidebar(),
    onOpenIssues: () => workspace.openSidebar('issues'),
    onToggleMaximize: () => {
      const tab = $activeTab;
      if (tab?.focusedPaneId) tabStore.toggleMaximize(tab.id, tab.focusedPaneId);
    },
    onFocusPane: (idx) => {
      const tab = $activeTab;
      if (tab && idx < tab.panes.length) tabStore.focusPane(tab.id, tab.panes[idx].id);
    },
    canAddPane: () => ($activeTab?.panes.length ?? 0) < MAX_PANES_PER_TAB,
    onToggleDashboard: () => {
      if ($workspace.activeView === 'dashboard') workspace.setView('terminals');
      else workspace.setView('dashboard');
    },
  });

  onMount(async () => {
    // Secondary window: load config (theme), restore detached tab, set up merge-on-close
    if (!_isMain) {
      // Apply theme from config
      try {
        const cfg = await App.GetConfig();
        config.set(cfg);
        applyTheme(cfg.theme || 'dark');
        if (cfg.terminal_color) applyAccentColor(cfg.terminal_color);
      } catch { applyTheme('dark'); }

      // Restore the tab that was detached into this window
      try {
        const stateJSON = await App.GetDetachedTabState(_windowId);
        if (stateJSON) {
          const tab = JSON.parse(stateJSON);
          tabStore.importTab(tab);
        }
      } catch (e) {
        console.error('[secondary] GetDetachedTabState failed', e);
      }

      // Push tab state to backend on every change so WindowClosing hook can
      // emit window:tabs-merged reliably (no IPC race on close).
      let saveTabsTimer: ReturnType<typeof setTimeout> | null = null;
      allTabs.subscribe(tabs => {
        if (saveTabsTimer) clearTimeout(saveTabsTimer);
        saveTabsTimer = setTimeout(() => {
          App.SaveWindowTabs(_windowId, JSON.stringify({ tabs })).catch(() => {});
        }, 300);
      });
      return; // skip rest of onMount for secondary windows
    }

    try {
      const cfg = await App.GetConfig();
      config.set(cfg);
      applyTheme(cfg.theme || 'dark');
      if (cfg.terminal_color) applyAccentColor(cfg.terminal_color);
      if (cfg.sidebar_pinned) { workspace.setSidebarPinned(true); workspace.openSidebar('explorer'); }
      await initI18n((cfg.language || 'de') as Language);
      if (!cfg.setup_done) showSetupDialog = true;
    } catch { applyTheme('dark'); await initI18n('de'); }

    try {
      resolvedClaudePath = (await App.GetResolvedClaudePath()) || 'claude';
      claudeDetected = await App.IsClaudeDetected();
    } catch {
      resolvedClaudePath = 'claude';
      claudeDetected = false;
    }

    try {
      resolvedCodexPath = (await App.GetResolvedCodexPath()) || 'codex';
      codexDetected = await App.IsCodexDetected();
    } catch {
      resolvedCodexPath = 'codex';
      codexDetected = false;
    }

    try {
      resolvedGeminiPath = (await App.GetResolvedGeminiPath()) || 'gemini';
      geminiDetected = await App.IsGeminiDetected();
    } catch {
      resolvedGeminiPath = 'gemini';
      geminiDetected = false;
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

    const restored = await restoreSession(resolvedClaudePath, resolvedCodexPath, resolvedGeminiPath);
    if (!restored) {
      let workDir = '';
      try { workDir = await App.GetWorkingDir(); } catch {}
      tabStore.addTab('Workspace', workDir);
    }

    // Start keep-alive loop (auto-start + periodic ping).
    // NOTE: restoreSession() calls tabStore.addPane() with running=true before
    // CreateSession resolves, so findFirstClaudePane() in startKeepAliveLoop
    // correctly sees restored panes immediately.
    if ($config.keep_alive) {
      keepAliveCleanup = await startKeepAliveLoop($config.keep_alive, resolvedClaudePath);
    }

    // Listen for tabs merging back from secondary windows
    EventsOn('window:tabs-merged', (event: any) => {
      try {
        const incoming = JSON.parse(event.data?.tabState ?? '{}');
        if (Array.isArray(incoming?.tabs)) {
          for (const tab of incoming.tabs) {
            tabStore.importTab(tab);
          }
        }
      } catch (e) {
        console.error('[window:tabs-merged] parse error', e);
      }
    });

    // Wails v3: event handlers receive a WailsEvent object; payload is in event.data
    EventsOn('terminal:activity', (event: any) => {
      const info = event.data; // ActivityInfo { id, activity, cost }
      tabStore.updateActivity(info.id, info.activity, info.cost);
      // Notify when an issue-linked agent finishes (only when window is focused,
      // because TerminalPane already sends a notification when unfocused)
      if (info.activity === 'done' && document.hasFocus()) {
        for (const tab of $allTabs) {
          const pane = tab.panes.find((p: any) => p.sessionId === info.id);
          if (pane?.issueNumber) {
            sendNotification($t('app.agentDone', { number: pane.issueNumber }), pane.issueTitle || pane.name);
            break;
          }
        }
      }
    });
    EventsOn('terminal:exit', (event: any) => {
      const id: number = event.data.id;
      tabStore.markExited(id);
    });
    EventsOn('terminal:error', (event: any) => {
      const id: number = event.data.id;
      const msg: string = event.data.message;
      console.error('[terminal:error]', id, msg);
      alert($t('app.terminalError', { id: String(id), msg }));
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
    if (keepAliveCleanup) keepAliveCleanup();
    window.removeEventListener('beforeunload', saveSession);
    document.removeEventListener('keydown', handleGlobalKeydown);
  });

  async function updateBranch() {
    const tab = $activeTab;
    if (!tab) return;
    branch = await fetchBranch(tab.dir || '.');
  }

  async function loadWorktrees() {
    const tab = $activeTab;
    if (!tab?.dir) return;
    try { allWorktrees = await App.ListAllWorktrees(tab.dir); } catch {}
  }

  $: if ($activeTab) { updateBranch(); updateCommitAge(); updateIssueCount(); updateConflicts(); loadWorktrees(); checkProjectInit($activeTab.dir); }

  async function updateCommitAge() {
    const tab = $activeTab;
    if (!tab) return;
    const dir = tab.dir || '.';
    commitAgeMinutes = await fetchCommitAge(dir);
    const bg = $config.background_agents;
    if (bg?.review_enabled || bg?.test_enabled) {
      checkForNewCommit(dir, bg, { claude: resolvedClaudePath, codex: resolvedCodexPath, gemini: resolvedGeminiPath }, tab.id);
    }
  }

  async function updateConflicts() {
    const tab = $activeTab;
    const info = await fetchConflicts(tab?.dir || '');
    if (prevConflictCount === 0 && info.count > 0) {
      const opLabel = info.operation
        ? ` (${info.operation.charAt(0).toUpperCase() + info.operation.slice(1)})`
        : '';
      sendNotification(
        $t('app.mergeConflicts', { op: opLabel }),
        $t('app.conflictFiles', { count: info.count })
      );
    }
    prevConflictCount = info.count;
    conflictCount = info.count;
    conflictFiles = info.files;
    conflictOperation = info.operation;
  }

  async function handleOpenWorktreePane(e: CustomEvent<{ worktree: any }>) {
    const tab = $activeTab;
    if (!tab) return;
    const wt = e.detail.worktree;
    if (tab.panes.length >= MAX_PANES_PER_TAB) {
      alert(`Max. ${MAX_PANES_PER_TAB} Terminals pro Tab erreicht.`);
      return;
    }
    const claudeCmd = resolvedClaudePath;
    const argv = buildClaudeArgv('claude', '', claudeCmd);
    const name = `Claude – ⎇ ${wt.branch}`;
    try {
      const sessionId = await App.CreateSession(argv, wt.path, 24, 80, 'claude');
      if (sessionId > 0) {
        tabStore.addPane(tab.id, sessionId, name, 'claude', '', null, '', wt.branch, wt.path, wt.branch);
      }
    } catch (err) { console.error('[handleOpenWorktreePane] failed:', err); }
  }

  async function handleLaunch(e: CustomEvent<{ type: PaneMode; model: string; issue?: { number: number; title: string; body: string; labels: string[] } | null }>) {
    const { type, model, issue } = e.detail;
    showLaunchDialog = false;
    const issueCtx = issue || launchIssueContext;
    launchIssueContext = null;
    const tab = $activeTab;
    if (!tab) return;
    if (tab.panes.length >= MAX_PANES_PER_TAB) {
      alert($t('app.maxPanes', { max: MAX_PANES_PER_TAB }));
      return;
    }
    const argv = buildClaudeArgv(type, model, resolvedClaudePath, resolvedCodexPath, resolvedGeminiPath);
    const baseName = getClaudeName(type, model);
    const name = issueCtx ? `${baseName} – #${issueCtx.number}` : baseName;
    try {
      let issueBranch = '';
      let worktreePath = '';
      let sessionDir = tab.dir || '';

      if (issueCtx) {
        const result = await setupIssueBranch(
          sessionDir, issueCtx,
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

      const sessionId = await App.CreateSession(argv, sessionDir, 24, 80, type);
      if (sessionId > 0) {
        let paneBranch = issueBranch;
        if (!paneBranch) {
          try { paneBranch = await App.GetGitBranch(sessionDir); } catch {}
        }
        tabStore.addPane(tab.id, sessionId, name, type, model,
          issueCtx?.number, issueCtx?.title, issueBranch, worktreePath, paneBranch);
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

      const sessionId = await App.CreateSession(argv, resolved.sessionDir, 24, 80, type);
      if (sessionId > 0) {
        let paneBranch = resolved.issueBranch;
        if (!paneBranch) {
          try { paneBranch = await App.GetGitBranch(resolved.sessionDir); } catch {}
        }
        tabStore.addPane(tab.id, sessionId, name, type, model,
          issueCtx.number, issueCtx.title, resolved.issueBranch, resolved.worktreePath, paneBranch);
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

  function handleDashboardNavigate(e: CustomEvent<{ tabId: string; paneId: string }>) {
    const { tabId, paneId } = e.detail;
    showDashboard = false;
    workspace.setView('terminals');
    tabStore.setActiveTab(tabId);
    tabStore.focusPane(tabId, paneId);
  }

  async function checkProjectInit(dir: string) {
    if (!dir) return;
    try {
      const initialized = await App.IsProjectInitialized(dir);
      if (!initialized) {
        skillPickerDir = dir;
        showSkillPicker = true;
      }
    } catch {}
  }

  async function handleSkillPickerDone(e: CustomEvent<{ skillIds: string[] }>) {
    showSkillPicker = false;
    const dir = skillPickerDir;
    skillPickerDir = '';
    if (dir) {
      try { await App.InitProject(dir, e.detail.skillIds); } catch (err) { console.error('[InitProject]', err); }
    }
  }

  function handleSkillPickerSkip() {
    showSkillPicker = false;
    const dir = skillPickerDir;
    skillPickerDir = '';
    if (dir) {
      App.InitProject(dir, []).catch(() => {});
    }
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
    const argv = buildClaudeArgv(mode, model, resolvedClaudePath, resolvedCodexPath, resolvedGeminiPath);
    try {
      const newSessionId = await App.CreateSession(argv, tab.dir || '', 24, 80, mode);
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
    const rel = e.detail.path;
    const dir = $activeTab?.dir ?? '';
    // Resolve relative path against working directory
    const fullPath = rel.match(/^[A-Z]:|^\//) ? rel : (dir ? dir.replace(/\\/g, '/').replace(/\/$/, '') + '/' + rel.replace(/\\/g, '/') : rel);
    previewFilePath = fullPath;
  }

  async function handleTogglePin() {
    const pinned = !$config.sidebar_pinned;
    config.update(c => ({ ...c, sidebar_pinned: pinned }));
    workspace.setSidebarPinned(pinned);
    try { await App.SaveConfig({ ...$config, sidebar_pinned: pinned }); } catch {}
    if (pinned) workspace.openSidebar($workspace.sidebarView || 'explorer');
  }

  function handleSidebarFile(e: CustomEvent<{ path: string }>) {
    previewFilePath = e.detail.path;
  }

  let _prevActiveTabId = '';
  $: {
    const id = $activeTab?.id ?? '';
    if (id && id !== _prevActiveTabId) {
      if (_prevActiveTabId !== '' && $workspace.activeView === 'dashboard') workspace.setView('terminals');
      _prevActiveTabId = id;
    }
  }

  // Sync showDashboard with workspace store for backward compatibility
  $: showDashboard = $workspace.activeView === 'dashboard';

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

  async function handleSetupFinish(e: CustomEvent<{ language: Language; claudeEnabled: boolean; codexEnabled: boolean; geminiEnabled: boolean }>) {
    const { language, claudeEnabled, codexEnabled, geminiEnabled } = e.detail;
    showSetupDialog = false;
    await setLanguage(language);
    const updated = {
      ...$config,
      language,
      setup_done: true,
      claude_enabled: claudeEnabled,
      codex_enabled: codexEnabled,
      gemini_enabled: geminiEnabled,
    };
    config.set(updated);
    try { await App.SaveConfig(updated); } catch {}
  }

  async function handleSetupLangChange(e: CustomEvent<{ lang: Language }>) {
    await setLanguage(e.detail.lang);
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

  async function handleCommitPush(e: CustomEvent<{ paneId: string; sessionId: number }>) {
    const { sessionId } = e.detail;
    const tab = $activeTab;
    if (!tab) return;
    const dir = tab.dir || '';
    try {
      const suggestion = await App.GenerateCommitSuggestion(dir, []);
      const type = suggestion.type || 'chore';
      const scope = suggestion.scope ? `(${suggestion.scope})` : '';
      const desc = suggestion.description || 'update';
      const msg = `${type}${scope}: ${desc}`;
      App.WriteToSession(sessionId, encodeForPty(`git add -A && git commit -m '${msg.replace(/'/g, "'\\''")}' && git push\n`));
    } catch (err) {
      console.error('[handleCommitPush] failed:', err);
      App.WriteToSession(sessionId, encodeForPty(`git add -A && git commit -m 'chore: update' && git push\n`));
    }
  }

  async function handleIssueAction(e: CustomEvent<{ paneId: string; sessionId: number; issueNumber: number; action: string }>) {
    const { sessionId, issueNumber, action } = e.detail;
    const tab = $activeTab;
    if (!tab) return;
    const dir = tab.dir || '';

    if (action === 'commit') {
      try {
        const suggestion = await App.GenerateCommitSuggestion(dir, []);
        const type = suggestion.type || 'chore';
        const scope = suggestion.scope ? `(${suggestion.scope})` : '';
        const desc = suggestion.description || 'update';
        const msg = `${type}${scope}: ${desc} (#${issueNumber})`;
        App.WriteToSession(sessionId, encodeForPty(`git add -A && git commit -m '${msg.replace(/'/g, "'\\''")}' && git push\n`));
      } catch {
        App.WriteToSession(sessionId, encodeForPty(`git add -A && git commit -m 'chore: update (#${issueNumber})' && git push\n`));
      }
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
  <TabBar
    activeTabId={$activeTab?.id ?? ''}
    isDashboard={$workspace.activeView === 'dashboard'}
    on:addTab={() => (showProjectDialog = true)}
    on:showDashboard={() => workspace.setView('dashboard')}
    on:closeDashboard={() => workspace.setView('terminals')}
  />
  <Toolbar
    paneCount={currentPanes}
    maxPanes={MAX_PANES_PER_TAB}
    tabDir={$activeTab?.dir ?? ''}
    {canChangeDir}
    on:newTerminal={() => (showLaunchDialog = true)}
    on:toggleSidebar={() => workspace.toggleSidebar()}
    on:changeDir={handleChangeDir}
    on:openSettings={() => (showSettingsDialog = true)}
    on:openCommands={() => (showCommandPalette = true)}
  />

  <div class="content">
    <LeftNav {issueCount} queueCount={0} chatUnread={0} />
    <Sidebar visible={$workspace.activeView === 'terminals' && $workspace.sidebarView !== null} dir={$activeTab?.dir ?? ''} {issueCount} {paneIssues} {conflictFiles} {conflictOperation} initialView={$workspace.sidebarView || 'explorer'} pinned={$config.sidebar_pinned} on:close={() => workspace.closeSidebar()} on:togglePin={handleTogglePin} on:selectFile={handleSidebarFile} on:createIssue={handleCreateIssue} on:editIssue={handleEditIssue} on:launchForIssue={handleLaunchForIssue} />
    {#if $workspace.activeView === 'dashboard'}
      <DashboardView on:navigate={handleDashboardNavigate} />
    {:else if $workspace.activeView === 'terminals'}
      <div class="tab-layers">
        {#each $allTabs as tab (tab.id)}
          <div class="tab-layer" class:active={tab.id === $activeTab?.id}>
            <PaneGrid
              tabId={tab.id}
              panes={tab.panes}
              active={tab.id === $activeTab?.id}
              worktrees={allWorktrees}
              tabDir={$activeTab?.dir || ''}
              on:closePane={handleClosePane}
              on:maximizePane={handleMaximizePane}
              on:focusPane={handleFocusPane}
              on:renamePane={handleRenamePane}
              on:restartPane={handleRestartPane}
              on:issueAction={handleIssueAction}
              on:commitPush={handleCommitPush}
              on:navigateFile={handleNavigateFile}
              on:splitPane={() => (showLaunchDialog = true)}
              on:openWorktreePane={handleOpenWorktreePane}
              on:worktreeListChanged={loadWorktrees}
            />
          </div>
        {/each}
        {#if previewFilePath}
          <FilePreview filePath={previewFilePath} dir={$activeTab?.dir ?? ''} on:close={() => (previewFilePath = '')} />
        {/if}
      </div>
    {:else if $workspace.activeView === 'kanban'}
      <div class="placeholder-view">
        <div class="placeholder-icon">&#9635;</div>
        <h3>Kanban Board</h3>
        <p>Wird in Sprint 2 implementiert</p>
      </div>
    {:else if $workspace.activeView === 'chat'}
      <div class="placeholder-view">
        <div class="placeholder-icon">&#128172;</div>
        <h3>Chat</h3>
        <p>Wird in Sprint 3 implementiert</p>
      </div>
    {:else if $workspace.activeView === 'queue'}
      <div class="placeholder-view">
        <div class="placeholder-icon">&#8801;</div>
        <h3>Queue-Übersicht</h3>
        <p>Wird in Sprint 3 implementiert</p>
      </div>
    {/if}
  </div>

  <Footer {branch} {totalCost} {tabInfo} {commitAgeMinutes} {conflictCount} {conflictOperation} {updateAvailable} {latestVersion} {downloadURL} />
  <LaunchDialog visible={showLaunchDialog} issueContext={launchIssueContext} {claudeDetected} {codexDetected} {geminiDetected} on:launch={handleLaunch} on:openSettings={() => { showLaunchDialog = false; showSettingsDialog = true; }} on:close={() => { showLaunchDialog = false; launchIssueContext = null; }} />
  <ProjectDialog visible={showProjectDialog} on:create={handleProjectCreate} on:close={() => (showProjectDialog = false)} />
  <SettingsDialog visible={showSettingsDialog} on:close={() => (showSettingsDialog = false)} on:saved={async () => { try { resolvedClaudePath = (await App.GetResolvedClaudePath()) || 'claude'; claudeDetected = await App.IsClaudeDetected(); } catch {} try { resolvedCodexPath = (await App.GetResolvedCodexPath()) || 'codex'; codexDetected = await App.IsCodexDetected(); } catch {} try { resolvedGeminiPath = (await App.GetResolvedGeminiPath()) || 'gemini'; geminiDetected = await App.IsGeminiDetected(); } catch {} }} />
  <CommandPalette visible={showCommandPalette} on:send={handleSendCommand} on:close={() => (showCommandPalette = false)} />
  <SetupDialog visible={showSetupDialog} {claudeDetected} {codexDetected} {geminiDetected} on:finish={handleSetupFinish} on:langChange={handleSetupLangChange} on:close={() => { showSetupDialog = false; }} />
  <CrashDialog visible={showCrashDialog} on:enable={handleCrashEnable} on:dismiss={() => (showCrashDialog = false)} />
  <IssueDialog visible={showIssueDialog} dir={$activeTab?.dir ?? ''} editIssue={editIssueData} on:saved={handleIssueSaved} on:close={() => { showIssueDialog = false; editIssueData = null; }} />
  <SkillPicker visible={showSkillPicker} dir={skillPickerDir} on:done={handleSkillPickerDone} on:skip={handleSkillPickerSkip} on:close={() => { showSkillPicker = false; skillPickerDir = ''; }} />
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
  .placeholder-view {
    flex: 1; display: flex; flex-direction: column;
    align-items: center; justify-content: center;
    color: var(--fg-muted, #a6adc8); gap: 0.5rem;
  }
  .placeholder-icon { font-size: 3rem; opacity: 0.3; }
  .placeholder-view h3 { color: var(--fg, #cdd6f4); font-size: 1.2rem; }
  .placeholder-view p { font-size: 0.85rem; }
</style>
