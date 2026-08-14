import { describe, it, expect } from 'vitest';
import { formatDuration } from './duration';

const NOW = 1_800_000_000_000; // ms
const sec = (agoSeconds: number) => Math.floor(NOW / 1000) - agoSeconds;

describe('formatDuration', () => {
  it('returns empty string for an unknown timestamp', () => {
    expect(formatDuration(0, NOW)).toBe('');
  });

  it('collapses anything under a minute', () => {
    expect(formatDuration(sec(0), NOW)).toBe('gerade eben');
    expect(formatDuration(sec(59), NOW)).toBe('gerade eben');
  });

  it('shows whole minutes below an hour', () => {
    expect(formatDuration(sec(60), NOW)).toBe('1 Min');
    expect(formatDuration(sec(12 * 60), NOW)).toBe('12 Min');
    expect(formatDuration(sec(59 * 60 + 59), NOW)).toBe('59 Min');
  });

  it('shows hours and minutes below a day', () => {
    expect(formatDuration(sec(60 * 60), NOW)).toBe('1 Std 0');
    expect(formatDuration(sec(3 * 60 * 60 + 20 * 60), NOW)).toBe('3 Std 20');
    expect(formatDuration(sec(23 * 60 * 60 + 59 * 60), NOW)).toBe('23 Std 59');
  });

  it('switches to whole days after 24 hours', () => {
    expect(formatDuration(sec(24 * 60 * 60), NOW)).toBe('1 Tag');
    expect(formatDuration(sec(3 * 24 * 60 * 60), NOW)).toBe('3 Tage');
  });

  it('never renders a negative duration when clocks disagree', () => {
    expect(formatDuration(sec(-30), NOW)).toBe('gerade eben');
  });
});
