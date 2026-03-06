import { tabStore } from '../stores/tabs';
import { buildClaudeArgv, encodeForPty } from './claude';
import * as App from '../../wailsjs/go/backend/App';

export interface BackgroundAgentsConfig {
  review_enabled?: boolean;
  review_tool: string;
  review_model: string;
  review_prompt: string;
  test_enabled?: boolean;
  test_command: string;
}

export interface CliPaths {
  claude: string;
  codex: string;
  gemini: string;
}

// Track the last known commit hash per directory
const lastKnownHash = new Map<string, string>();

// Track BG pane session IDs for reuse
const bgReviewPanes = new Map<string, number>(); // dir -> sessionId
const bgTestPanes = new Map<string, number>();    // dir -> sessionId

function findBgPane(tabId: string, type: 'review' | 'test'): number | null {
  const state = tabStore.getState();
  const tab = state.tabs.find(t => t.id === tabId);
  if (!tab) return null;
  const map = type === 'review' ? bgReviewPanes : bgTestPanes;
  const dir = tab.dir || '';
  const sessionId = map.get(dir);
  if (!sessionId) return null;
  // Check if pane still exists and is running
  for (const pane of tab.panes) {
    if (pane.sessionId === sessionId && pane.running && pane.background) {
      return sessionId;
    }
  }
  map.delete(dir);
  return null;
}

async function ensureReviewPane(
  tabId: string,
  dir: string,
  cfg: BackgroundAgentsConfig,
  cliPaths: CliPaths,
): Promise<number | null> {
  const existing = findBgPane(tabId, 'review');
  if (existing) return existing;

  const tool = (cfg.review_tool || 'claude') as import('../stores/tabs').PaneMode;
  const model = cfg.review_model || '';
  const argv = buildClaudeArgv(tool, model, cliPaths.claude, cliPaths.codex, cliPaths.gemini);
  const toolLabel = tool.charAt(0).toUpperCase() + tool.slice(1);
  try {
    const sessionId = await App.CreateSession(argv, dir, 24, 80, tool);
    if (sessionId > 0) {
      tabStore.addPane(tabId, sessionId, `Review ${toolLabel} (BG)`, tool, model,
        null, '', '', '', '', true);
      bgReviewPanes.set(dir, sessionId);
      return sessionId;
    }
  } catch (err) {
    console.error('[bg-agents] create review pane failed:', err);
  }
  return null;
}

async function ensureTestPane(
  tabId: string,
  dir: string,
): Promise<number | null> {
  const existing = findBgPane(tabId, 'test');
  if (existing) return existing;

  try {
    const sessionId = await App.CreateSession([], dir, 24, 80, 'shell');
    if (sessionId > 0) {
      tabStore.addPane(tabId, sessionId, 'Test (BG)', 'shell', '',
        null, '', '', '', '', true);
      bgTestPanes.set(dir, sessionId);
      return sessionId;
    }
  } catch (err) {
    console.error('[bg-agents] create test pane failed:', err);
  }
  return null;
}

export async function checkForNewCommit(
  dir: string,
  cfg: BackgroundAgentsConfig,
  cliPaths: CliPaths,
  tabId: string,
): Promise<void> {
  if (!dir) return;

  let currentHash: string;
  try {
    currentHash = await App.GetLastCommitHash(dir);
  } catch {
    return; // not a git repo or error
  }
  if (!currentHash) return;

  const prevHash = lastKnownHash.get(dir);
  lastKnownHash.set(dir, currentHash);

  // First run: just record the hash, don't trigger
  if (!prevHash) return;
  // No new commit
  if (prevHash === currentHash) return;

  console.log('[bg-agents] new commit detected:', currentHash.slice(0, 8));

  // Trigger review
  if (cfg.review_enabled) {
    try {
      const diff = await App.GetLastCommitDiff(dir);
      if (diff) {
        const sessionId = await ensureReviewPane(tabId, dir, cfg, cliPaths);
        if (sessionId) {
          const prompt = cfg.review_prompt.replace('{diff}', diff);
          await App.AddToQueue(sessionId, prompt);
        }
      }
    } catch (err) {
      console.error('[bg-agents] review trigger failed:', err);
    }
  }

  // Trigger test
  if (cfg.test_enabled && cfg.test_command) {
    try {
      const sessionId = await ensureTestPane(tabId, dir);
      if (sessionId) {
        await App.WriteToSession(sessionId, encodeForPty(cfg.test_command + '\n'));
      }
    } catch (err) {
      console.error('[bg-agents] test trigger failed:', err);
    }
  }
}
