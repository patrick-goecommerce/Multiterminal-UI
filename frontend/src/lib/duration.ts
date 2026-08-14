/**
 * Formats how long a pane has been in its current state.
 *
 * The output carries no "vor"/"seit" prefix: the state in front of it supplies
 * the tense already ("fertig · 3 Std 20" reads as "finished for 3h20"), and a
 * prefix would have to differ per state to stay correct.
 *
 * @param sinceUnix seconds since epoch; 0 means unknown
 * @param nowMs current time in milliseconds
 * @returns the formatted duration, or '' when unknown
 */
export function formatDuration(sinceUnix: number, nowMs: number): string {
  if (!sinceUnix || sinceUnix <= 0) return '';

  // Clamp: a restored timestamp from a machine whose clock ran ahead would
  // otherwise render as a negative duration.
  const seconds = Math.max(0, Math.floor(nowMs / 1000) - sinceUnix);

  if (seconds < 60) return 'gerade eben';

  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes} Min`;

  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours} Std ${minutes % 60}`;

  const days = Math.floor(hours / 24);
  return days === 1 ? '1 Tag' : `${days} Tage`;
}
