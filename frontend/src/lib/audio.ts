import { writable } from 'svelte/store';

export const audioMuted = writable(false);

let ctx: AudioContext | null = null;

function getContext(): AudioContext | null {
  if (!ctx) {
    try {
      ctx = new AudioContext();
    } catch {
      return null;
    }
  }
  if (ctx.state === 'suspended') ctx.resume();
  return ctx;
}

function playTone(freq: number, duration: number, volume: number, startTime: number, ac: AudioContext) {
  const osc = ac.createOscillator();
  const gain = ac.createGain();
  osc.frequency.value = freq;
  osc.type = 'sine';
  gain.gain.value = volume;
  gain.gain.setValueAtTime(volume, startTime + duration - 0.02);
  gain.gain.linearRampToValueAtTime(0, startTime + duration);
  osc.connect(gain);
  gain.connect(ac.destination);
  osc.start(startTime);
  osc.stop(startTime + duration);
}

function playDone(volume: number) {
  const ac = getContext();
  if (!ac) return;
  const v = volume / 100 * 0.3;
  const now = ac.currentTime;
  playTone(523.25, 0.1, v, now, ac);        // C5
  playTone(659.25, 0.12, v, now + 0.12, ac); // E5
}

function playNeedsInput(volume: number) {
  const ac = getContext();
  if (!ac) return;
  const v = volume / 100 * 0.3;
  const now = ac.currentTime;
  playTone(440, 0.08, v, now, ac);        // A4
  playTone(440, 0.08, v, now + 0.15, ac); // A4
  playTone(440, 0.08, v, now + 0.30, ac); // A4
}

function playError(volume: number) {
  const ac = getContext();
  if (!ac) return;
  const v = volume / 100 * 0.3;
  const now = ac.currentTime;
  const osc = ac.createOscillator();
  const gain = ac.createGain();
  osc.type = 'sine';
  osc.frequency.setValueAtTime(329.63, now);                  // E4
  osc.frequency.linearRampToValueAtTime(261.63, now + 0.2);   // C4
  gain.gain.value = v;
  gain.gain.setValueAtTime(v, now + 0.18);
  gain.gain.linearRampToValueAtTime(0, now + 0.2);
  osc.connect(gain);
  gain.connect(ac.destination);
  osc.start(now);
  osc.stop(now + 0.2);
}

export function playCustomSound(path: string, volume: number) {
  const audio = new Audio(path);
  audio.volume = Math.min(1, Math.max(0, volume / 100));
  audio.play().catch(() => {});
}

export function playBell(event: 'done' | 'needsInput' | 'error', volume: number, customPath?: string) {
  if (customPath) {
    playCustomSound(customPath, volume);
    return;
  }
  if (event === 'done') playDone(volume);
  else if (event === 'needsInput') playNeedsInput(volume);
  else if (event === 'error') playError(volume);
}
