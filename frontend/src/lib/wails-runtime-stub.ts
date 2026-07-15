// Stub for /wails/runtime.js used during Vite build.
// At runtime inside Wails v3, the actual /wails/runtime.js is served by the
// Go application and provides all real implementations.
// This stub provides no-op shims so that `npm run build` succeeds without
// a live Wails backend.

// WailsEvent type (mirrored from runtime type declarations)
export interface WailsEvent {
  name: string;
  data: any;
  sender?: string;
}

type EventCallback = (event: WailsEvent) => void;
const _listeners = new Map<string, EventCallback[]>();

export const Events = {
  On(eventName: string, callback: EventCallback): () => void {
    const list = _listeners.get(eventName) ?? [];
    list.push(callback);
    _listeners.set(eventName, list);
    return () => {
      const l = _listeners.get(eventName) ?? [];
      _listeners.set(eventName, l.filter(cb => cb !== callback));
    };
  },
  OnMultiple(eventName: string, callback: EventCallback, _maxCallbacks: number): () => void {
    return Events.On(eventName, callback);
  },
  Once(eventName: string, callback: EventCallback): () => void {
    return Events.On(eventName, callback);
  },
  Off(...eventNames: string[]): void {
    eventNames.forEach(n => _listeners.delete(n));
  },
  OffAll(): void {
    _listeners.clear();
  },
  Emit(eventName: string, data?: any): void {
    const list = _listeners.get(eventName) ?? [];
    const evt: WailsEvent = { name: eventName, data: data ?? null };
    list.forEach(cb => cb(evt));
  },
};

export const Browser = {
  OpenURL(url: string): void {
    window.open(url, '_blank');
  },
};

export const Clipboard = {
  async Text(): Promise<string> {
    try { return await navigator.clipboard.readText(); } catch { return ''; }
  },
  async SetText(text: string): Promise<void> {
    try { await navigator.clipboard.writeText(text); } catch {}
  },
};

export const Window = {
  Get(_name: string): any { return Window; },
  async Center(): Promise<void> {},
  async Close(): Promise<void> {},
  async Focus(): Promise<void> {},
  async Fullscreen(): Promise<void> {},
  async Hide(): Promise<void> {},
  async Maximise(): Promise<void> {},
  async Minimise(): Promise<void> {},
  async Name(): Promise<string> { return ''; },
  async Reload(): Promise<void> { window.location.reload(); },
  async Restore(): Promise<void> {},
  async SetTitle(title: string): Promise<void> { document.title = title; },
  async Show(): Promise<void> {},
  async Size(): Promise<{width: number; height: number}> {
    return { width: window.innerWidth, height: window.innerHeight };
  },
  async Height(): Promise<number> { return window.innerHeight; },
  async Width(): Promise<number> { return window.innerWidth; },
  async ToggleMaximise(): Promise<void> {},
  async UnMaximise(): Promise<void> {},
  async UnMinimise(): Promise<void> {},
  async UnFullscreen(): Promise<void> {},
  async SetSize(_w: number, _h: number): Promise<void> {},
  async SetMinSize(_w: number, _h: number): Promise<void> {},
  async SetMaxSize(_w: number, _h: number): Promise<void> {},
  async SetAlwaysOnTop(_b: boolean): Promise<void> {},
  async SetPosition(_x: number, _y: number): Promise<void> {},
  async SetRelativePosition(_x: number, _y: number): Promise<void> {},
  async Position(): Promise<{x: number; y: number}> { return {x: 0, y: 0}; },
  async RelativePosition(): Promise<{x: number; y: number}> { return {x: 0, y: 0}; },
  async OpenDevTools(): Promise<void> {},
  async SetBackgroundColour(_r: number, _g: number, _b: number, _a: number): Promise<void> {},
  async SetZoom(_zoom: number): Promise<void> {},
  async GetZoom(): Promise<number> { return 1; },
  async ZoomIn(): Promise<void> {},
  async ZoomOut(): Promise<void> {},
  async ZoomReset(): Promise<void> {},
  async IsMaximised(): Promise<boolean> { return false; },
  async IsMinimised(): Promise<boolean> { return false; },
  async IsFullscreen(): Promise<boolean> { return false; },
  async IsFocused(): Promise<boolean> { return document.hasFocus(); },
  async GetScreen(): Promise<any> { return null; },
  async Print(): Promise<void> { window.print(); },
};

export const Application = {
  async Hide(): Promise<void> {},
  async Show(): Promise<void> {},
  async Quit(): Promise<void> {},
};

export const Screens = {
  async GetAll(): Promise<any[]> { return []; },
  async GetCurrent(): Promise<any> { return null; },
  async GetPrimary(): Promise<any> { return null; },
};

export const Dialogs = {
  async Info(_opts: any): Promise<void> {},
  async Warning(_opts: any): Promise<void> {},
  async Error(_opts: any): Promise<void> {},
  async Question(_opts: any): Promise<string> { return ''; },
  async OpenFile(_opts: any): Promise<string[]> { return []; },
  async SaveFile(_opts: any): Promise<string> { return ''; },
};

export const System = {
  async IsDarkMode(): Promise<boolean> {
    return window.matchMedia('(prefers-color-scheme: dark)').matches;
  },
  async Environment(): Promise<any> { return {}; },
  async Capabilities(): Promise<any> { return {}; },
};

export const Flags = {
  GetFlag(_name: string): any { return undefined; },
};

export const Call = {
  ByID(_id: number, ..._args: any[]): Promise<any> {
    return Promise.reject(new Error('Wails runtime not available (build stub)'));
  },
  ByName(_name: string, ..._args: any[]): Promise<any> {
    return Promise.reject(new Error('Wails runtime not available (build stub)'));
  },
};

export const Create = {
  Any: (v: any) => v,
  Array: (fn: any) => (v: any) => v,
  Map: (_kfn: any, vfn: any) => (v: any) => v,
  Nullable: (fn: any) => (v: any) => v,
  Struct: (_shape: any) => (v: any) => v,
  ByteSlice: (v: any) => v,
  Events: {} as any,
};

export const WML = {
  Enable(): void {},
  Reload(): void {},
};

export const CancelError = class CancelError extends Error {};
export const CancellablePromise = Promise;
export const CancelledRejectionError = class CancelledRejectionError extends Error {};

export const clientId = 'stub';
export function getTransport() { return null; }
export function setTransport(_t: any) {}
export function loadOptionalScript(_url: string): Promise<void> { return Promise.resolve(); }
export const objectNames = {};
export const IOS = {};
