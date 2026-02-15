<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import * as App from '../../wailsjs/go/backend/App';
  import { EventsOn } from '../../wailsjs/runtime/runtime';

  export let sessionId: number;
  export let visible: boolean = false;

  interface Item { id: number; prompt: string; status: string; }

  let items: Item[] = [];
  let promptText = '';
  let cleanupFn: (() => void) | null = null;

  async function loadQueue() {
    try {
      items = await App.GetQueue(sessionId);
    } catch (err) {
      console.error('[QueuePanel] GetQueue failed:', err);
    }
  }

  async function addItem() {
    const text = promptText.trim();
    if (!text) return;
    try {
      await App.AddToQueue(sessionId, text);
      promptText = '';
      await loadQueue();
    } catch (err) {
      console.error('[QueuePanel] AddToQueue failed:', err);
    }
  }

  async function removeItem(itemId: number) {
    try {
      await App.RemoveFromQueue(sessionId, itemId);
      await loadQueue();
    } catch (err) {
      console.error('[QueuePanel] RemoveFromQueue failed:', err);
    }
  }

  async function clearDone() {
    try {
      await App.ClearDoneFromQueue(sessionId);
      await loadQueue();
    } catch (err) {
      console.error('[QueuePanel] ClearDoneFromQueue failed:', err);
    }
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      addItem();
    }
    // Stop all key events from bubbling to the terminal
    e.stopPropagation();
  }

  onMount(() => {
    loadQueue();
    cleanupFn = EventsOn('queue:update', (sid: number) => {
      if (sid === sessionId) loadQueue();
    });
  });

  onDestroy(() => {
    if (cleanupFn) cleanupFn();
  });

  $: pendingCount = items.filter(i => i.status === 'pending').length;
  $: doneCount = items.filter(i => i.status === 'done').length;
  $: sentItem = items.find(i => i.status === 'sent');

  export function getPendingCount(): number {
    return items.filter(i => i.status !== 'done').length;
  }
</script>

{#if visible}
  <!-- svelte-ignore a11y-click-events-have-key-events -->
  <div class="queue-panel" on:click|stopPropagation>
    <div class="queue-header">
      <span class="queue-title">Pipeline Queue</span>
      {#if doneCount > 0}
        <button class="clear-btn" on:click={clearDone}>Clear done ({doneCount})</button>
      {/if}
    </div>

    <div class="queue-input">
      <textarea
        bind:value={promptText}
        on:keydown={handleKeydown}
        placeholder="Prompt eingeben... (Enter = Add)"
        rows="2"
      ></textarea>
      <button class="add-btn" on:click={addItem} disabled={!promptText.trim()}>+</button>
    </div>

    {#if items.length === 0}
      <div class="queue-empty">Queue ist leer. Prompts werden nacheinander abgearbeitet.</div>
    {:else}
      <div class="queue-list">
        {#each items as item (item.id)}
          <div class="queue-item" class:item-sent={item.status === 'sent'} class:item-done={item.status === 'done'}>
            <span class="item-status">
              {#if item.status === 'pending'}&#9679;
              {:else if item.status === 'sent'}&#9654;
              {:else}&#10003;{/if}
            </span>
            <span class="item-prompt" title={item.prompt}>
              {item.prompt.length > 80 ? item.prompt.slice(0, 80) + '...' : item.prompt}
            </span>
            {#if item.status !== 'sent'}
              <button class="item-remove" on:click={() => removeItem(item.id)} title="Entfernen">&times;</button>
            {/if}
          </div>
        {/each}
      </div>
    {/if}

    {#if pendingCount > 0 && sentItem}
      <div class="queue-footer">{pendingCount} wartend</div>
    {/if}
  </div>
{/if}

<style>
  .queue-panel {
    position: absolute;
    top: 30px;
    right: 0;
    width: 360px;
    max-height: 400px;
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 8px;
    box-shadow: 0 8px 24px rgba(0, 0, 0, 0.4);
    z-index: 50;
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }

  .queue-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 8px 12px;
    border-bottom: 1px solid var(--border);
  }

  .queue-title { font-size: 12px; font-weight: 600; color: var(--fg); }

  .clear-btn {
    font-size: 11px;
    background: none;
    border: 1px solid var(--border);
    border-radius: 4px;
    color: var(--fg-muted);
    cursor: pointer;
    padding: 2px 8px;
  }
  .clear-btn:hover { color: var(--fg); border-color: var(--fg-muted); }

  .queue-input {
    display: flex;
    gap: 6px;
    padding: 8px 12px;
    border-bottom: 1px solid var(--border);
  }

  .queue-input textarea {
    flex: 1;
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    border-radius: 4px;
    color: var(--fg);
    font-size: 12px;
    padding: 6px 8px;
    resize: none;
    font-family: inherit;
  }
  .queue-input textarea:focus { outline: none; border-color: var(--accent); }

  .add-btn {
    background: var(--accent);
    color: var(--bg);
    border: none;
    border-radius: 4px;
    width: 32px;
    font-size: 16px;
    font-weight: 700;
    cursor: pointer;
    align-self: stretch;
  }
  .add-btn:disabled { opacity: 0.4; cursor: default; }
  .add-btn:not(:disabled):hover { filter: brightness(1.2); }

  .queue-empty {
    padding: 16px 12px;
    text-align: center;
    color: var(--fg-muted);
    font-size: 12px;
  }

  .queue-list { overflow-y: auto; max-height: 240px; }

  .queue-item {
    display: flex;
    align-items: flex-start;
    gap: 8px;
    padding: 6px 12px;
    border-bottom: 1px solid var(--border);
    font-size: 12px;
  }
  .queue-item:last-child { border-bottom: none; }

  .item-sent { background: rgba(57, 255, 20, 0.06); }
  .item-done { opacity: 0.45; }

  .item-status {
    flex-shrink: 0;
    width: 14px;
    text-align: center;
    font-size: 10px;
    line-height: 18px;
    color: var(--fg-muted);
  }
  .item-sent .item-status { color: var(--accent); }
  .item-done .item-status { color: #22c55e; }

  .item-prompt {
    flex: 1;
    color: var(--fg);
    word-break: break-word;
    line-height: 18px;
  }

  .item-remove {
    background: none;
    border: none;
    color: var(--fg-muted);
    cursor: pointer;
    font-size: 14px;
    padding: 0 2px;
    line-height: 1;
    flex-shrink: 0;
  }
  .item-remove:hover { color: var(--error); }

  .queue-footer {
    padding: 6px 12px;
    font-size: 11px;
    color: var(--fg-muted);
    text-align: center;
    border-top: 1px solid var(--border);
  }
</style>
