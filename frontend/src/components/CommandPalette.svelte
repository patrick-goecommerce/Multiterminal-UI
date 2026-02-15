<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import { config } from '../stores/config';
  import type { CommandEntry } from '../stores/config';
  import * as App from '../../wailsjs/go/backend/App';

  export let visible: boolean = false;

  const dispatch = createEventDispatcher();

  let editingIndex = -1;
  let editName = '';
  let editText = '';
  let adding = false;
  let newName = '';
  let newText = '';

  function sendCommand(cmd: CommandEntry) {
    dispatch('send', { text: cmd.text });
  }

  function startEdit(index: number) {
    const cmd = $config.commands[index];
    editingIndex = index;
    editName = cmd.name;
    editText = cmd.text;
  }

  function saveEdit() {
    if (!editName.trim() || !editText.trim()) return;
    const commands = [...$config.commands];
    commands[editingIndex] = { name: editName.trim(), text: editText.trim() };
    updateCommands(commands);
    editingIndex = -1;
  }

  function cancelEdit() {
    editingIndex = -1;
  }

  function deleteCommand(index: number) {
    const commands = $config.commands.filter((_, i) => i !== index);
    updateCommands(commands);
    if (editingIndex === index) editingIndex = -1;
  }

  function startAdd() {
    adding = true;
    newName = '';
    newText = '';
  }

  function saveAdd() {
    if (!newName.trim() || !newText.trim()) return;
    const commands = [...$config.commands, { name: newName.trim(), text: newText.trim() }];
    updateCommands(commands);
    adding = false;
  }

  function cancelAdd() {
    adding = false;
  }

  function updateCommands(commands: CommandEntry[]) {
    config.update(c => ({ ...c, commands }));
    // Persist to disk via backend
    const cfg = { ...$config, commands };
    App.SaveConfig(cfg).catch(err => console.error('[CommandPalette] SaveConfig failed:', err));
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') dispatch('close');
    e.stopPropagation();
  }

  function close() {
    dispatch('close');
  }
</script>

{#if visible}
  <!-- svelte-ignore a11y-click-events-have-key-events -->
  <div class="overlay" on:click={close} on:keydown={handleKeydown}>
    <!-- svelte-ignore a11y-click-events-have-key-events -->
    <div class="palette" on:click|stopPropagation>
      <div class="palette-header">
        <h3>Befehlspalette</h3>
        <button class="close-btn" on:click={close}>&times;</button>
      </div>

      <div class="command-list">
        {#each $config.commands as cmd, i (i)}
          {#if editingIndex === i}
            <div class="command-edit">
              <input class="edit-input" bind:value={editName} placeholder="Name" on:keydown={handleKeydown} />
              <textarea class="edit-textarea" bind:value={editText} placeholder="Befehl / Text" rows="2" on:keydown={handleKeydown}></textarea>
              <div class="edit-actions">
                <button class="btn-save" on:click={saveEdit}>Speichern</button>
                <button class="btn-cancel" on:click={cancelEdit}>Abbrechen</button>
              </div>
            </div>
          {:else}
            <div class="command-item">
              <button class="command-trigger" on:click={() => sendCommand(cmd)} title={cmd.text}>
                <span class="cmd-name">{cmd.name}</span>
                <span class="cmd-preview">{cmd.text.length > 60 ? cmd.text.slice(0, 60) + '...' : cmd.text}</span>
              </button>
              <div class="command-actions">
                <button class="action-btn" on:click={() => startEdit(i)} title="Bearbeiten">&#9998;</button>
                <button class="action-btn delete" on:click={() => deleteCommand(i)} title="Löschen">&times;</button>
              </div>
            </div>
          {/if}
        {/each}

        {#if adding}
          <div class="command-edit">
            <input class="edit-input" bind:value={newName} placeholder="Name (z.B. Run Tests)" on:keydown={handleKeydown} />
            <textarea class="edit-textarea" bind:value={newText} placeholder="Befehl / Text" rows="2" on:keydown={handleKeydown}></textarea>
            <div class="edit-actions">
              <button class="btn-save" on:click={saveAdd} disabled={!newName.trim() || !newText.trim()}>Hinzufügen</button>
              <button class="btn-cancel" on:click={cancelAdd}>Abbrechen</button>
            </div>
          </div>
        {:else}
          <button class="add-btn" on:click={startAdd}>+ Neuen Befehl anlegen</button>
        {/if}
      </div>
    </div>
  </div>
{/if}

<style>
  .overlay {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.4);
    display: flex;
    align-items: flex-start;
    justify-content: center;
    padding-top: 80px;
    z-index: 100;
  }

  .palette {
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 12px;
    width: 480px;
    max-height: 500px;
    box-shadow: 0 8px 32px rgba(0, 0, 0, 0.5);
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }

  .palette-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 14px 16px;
    border-bottom: 1px solid var(--border);
  }

  h3 { margin: 0; font-size: 16px; color: var(--fg); font-weight: 600; }

  .close-btn {
    background: none;
    border: none;
    color: var(--fg-muted);
    font-size: 20px;
    cursor: pointer;
    padding: 0 4px;
    line-height: 1;
  }
  .close-btn:hover { color: var(--fg); }

  .command-list {
    overflow-y: auto;
    padding: 8px 0;
  }

  .command-item {
    display: flex;
    align-items: center;
    padding: 0 8px;
  }

  .command-trigger {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 2px;
    padding: 10px 12px;
    background: none;
    border: none;
    border-radius: 8px;
    color: var(--fg);
    cursor: pointer;
    text-align: left;
    transition: background 0.12s;
  }
  .command-trigger:hover { background: var(--bg-tertiary); }

  .cmd-name { font-size: 13px; font-weight: 500; }
  .cmd-preview { font-size: 11px; color: var(--fg-muted); font-family: monospace; }

  .command-actions {
    display: flex;
    gap: 2px;
    flex-shrink: 0;
    opacity: 0;
    transition: opacity 0.15s;
  }
  .command-item:hover .command-actions { opacity: 1; }

  .action-btn {
    background: none;
    border: none;
    color: var(--fg-muted);
    cursor: pointer;
    padding: 4px 6px;
    font-size: 14px;
    border-radius: 4px;
  }
  .action-btn:hover { background: var(--bg-tertiary); color: var(--fg); }
  .action-btn.delete:hover { color: var(--error); }

  .command-edit {
    padding: 8px 16px;
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .edit-input, .edit-textarea {
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    border-radius: 6px;
    color: var(--fg);
    font-size: 13px;
    padding: 8px 10px;
    font-family: inherit;
  }
  .edit-textarea { resize: none; font-family: monospace; font-size: 12px; }
  .edit-input:focus, .edit-textarea:focus { outline: none; border-color: var(--accent); }

  .edit-actions {
    display: flex;
    gap: 8px;
    justify-content: flex-end;
  }

  .btn-save {
    background: var(--accent);
    color: var(--bg);
    border: none;
    border-radius: 6px;
    padding: 6px 16px;
    font-size: 12px;
    font-weight: 600;
    cursor: pointer;
  }
  .btn-save:disabled { opacity: 0.4; cursor: default; }
  .btn-save:not(:disabled):hover { filter: brightness(1.2); }

  .btn-cancel {
    background: var(--bg-tertiary);
    color: var(--fg-muted);
    border: 1px solid var(--border);
    border-radius: 6px;
    padding: 6px 16px;
    font-size: 12px;
    cursor: pointer;
  }
  .btn-cancel:hover { color: var(--fg); }

  .add-btn {
    display: block;
    width: calc(100% - 32px);
    margin: 4px 16px 8px;
    padding: 10px;
    background: none;
    border: 1px dashed var(--border);
    border-radius: 8px;
    color: var(--fg-muted);
    font-size: 13px;
    cursor: pointer;
    transition: all 0.15s;
  }
  .add-btn:hover { border-color: var(--accent); color: var(--accent); }
</style>
