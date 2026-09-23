import { describe, it, expect } from 'vitest';
import { sameSession, sessionNumber } from './sessionRef';

describe('sessionNumber', () => {
  it('reads the number after the last colon', () => {
    expect(sessionNumber('a1b2:7')).toBe(7);
    expect(sessionNumber('host:with:colons:12')).toBe(12);
    expect(sessionNumber('7')).toBe(7);
  });

  it('is 0 for no session or garbage', () => {
    expect(sessionNumber('')).toBe(0);
    expect(sessionNumber('hub:')).toBe(0);
    expect(sessionNumber('hub:x')).toBe(0);
  });
});

describe('sameSession', () => {
  it('matches equal refs only', () => {
    expect(sameSession('h1:7', 'h1:7')).toBe(true);
    expect(sameSession('h2:7', 'h1:7')).toBe(false);
    expect(sameSession('h1:8', 'h1:7')).toBe(false);
  });

  // A pane saved before refs carried a hub has a bare number.
  it('matches a legacy bare number by its number', () => {
    expect(sameSession('7', 'h1:7')).toBe(true);
    expect(sameSession('8', 'h1:7')).toBe(false);
  });

  it('never matches no session', () => {
    expect(sameSession('', 'h1:7')).toBe(false);
    expect(sameSession('h1:7', '')).toBe(false);
    expect(sameSession('', '')).toBe(false);
  });
});
