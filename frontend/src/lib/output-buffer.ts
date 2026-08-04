/**
 * Bounded FIFO for raw PTY bytes waiting to be handed to xterm.js.
 *
 * The bound is necessary because a pane in a background tab does not drain at
 * all — without it, a busy session piles up hundreds of MB that then block the
 * JS thread the moment its tab is opened.
 *
 * What the bound must never do is trim the *oldest* chunks and keep the rest.
 * That leaves a hole in the middle of a VT100 stream: escape sequences get cut
 * in half, whole runs of text vanish, and since full-screen apps only repaint
 * the regions they changed, the pane stays garbled for good (#157). So an
 * overflow drops the entire backlog and reports it, letting the caller resync
 * the terminal from a full repaint instead.
 */
export class PendingOutput {
  private chunks: Uint8Array[] = [];
  private bytes = 0;

  constructor(private readonly maxBytes: number) {}

  /** Bytes currently buffered. */
  get byteLength(): number {
    return this.bytes;
  }

  get isEmpty(): boolean {
    return this.chunks.length === 0;
  }

  /**
   * Append a chunk. Returns true when the backlog overflowed and was dropped
   * wholesale — the caller must then resynchronise the terminal rather than
   * carry on writing.
   */
  push(chunk: Uint8Array): boolean {
    this.chunks.push(chunk);
    this.bytes += chunk.length;
    if (this.bytes > this.maxBytes) {
      this.clear();
      return true;
    }
    return false;
  }

  /**
   * Hand over up to maxBytes as one contiguous buffer, or null when empty.
   * The first chunk is always taken, so a chunk larger than the limit cannot
   * stall the pane forever.
   */
  drain(maxBytes: number): Uint8Array | null {
    if (this.chunks.length === 0) return null;

    let total = 0;
    let i = 0;
    while (i < this.chunks.length) {
      if (i > 0 && total + this.chunks[i].length > maxBytes) break;
      total += this.chunks[i].length;
      i++;
    }

    const taken = this.chunks.splice(0, i);
    this.bytes -= total;
    if (taken.length === 1) return taken[0];

    const merged = new Uint8Array(total);
    let offset = 0;
    for (const chunk of taken) {
      merged.set(chunk, offset);
      offset += chunk.length;
    }
    return merged;
  }

  clear(): void {
    this.chunks = [];
    this.bytes = 0;
  }
}
