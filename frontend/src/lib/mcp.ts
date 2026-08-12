import * as App from '../../wailsjs/go/backend/App';
import { MCP_PROFILE_GLOBAL, MCP_PROFILE_NONE } from './claude';

/**
 * Resolve a pane's MCP profile to the .mcp.json path for `--mcp-config`
 * (issue #179). The two sentinels need no file and never hit the backend.
 *
 * A failure resolves to '' — which, combined with a non-sentinel profile name,
 * makes buildClaudeArgv emit `--strict-mcp-config` alone. That is deliberate:
 * a pane the user asked to keep small falls back to ZERO servers, never to the
 * full global set.
 */
export async function resolveMCPConfigPath(dir: string, profile: string): Promise<string> {
  const name = (profile || '').trim();
  if (name === MCP_PROFILE_GLOBAL || name === MCP_PROFILE_NONE) return '';
  try {
    return (await App.ResolveMCPProfile(dir || '', name)) || '';
  } catch (err) {
    console.error('[mcp] profile resolution failed, falling back to zero MCP servers:', name, err);
    return '';
  }
}
