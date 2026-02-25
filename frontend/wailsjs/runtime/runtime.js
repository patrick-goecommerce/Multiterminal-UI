// Wails v3 runtime shim
// Re-exports from the Wails v3 runtime served at /wails/runtime.js
// This file keeps backwards compatibility with imports from wailsjs/runtime/runtime

export {
  Events,
  Window,
  Application,
  Browser,
  Clipboard,
  Dialogs,
  Screens,
  System,
  Call,
  Create,
  Flags,
  WML,
  CancelError,
  CancellablePromise,
  CancelledRejectionError,
  clientId,
  getTransport,
  setTransport,
  loadOptionalScript
} from '/wails/runtime.js';

// --- Wails v2 compatibility helpers ---
// These map old v2 function names to their v3 equivalents.

import { Events as _Events, Browser as _Browser, Clipboard as _Clipboard } from '/wails/runtime.js';

/** @deprecated Use Events.On from /wails/runtime.js */
export function EventsOn(eventName, callback) {
  return _Events.On(eventName, callback);
}

/** @deprecated Use Events.OnMultiple from /wails/runtime.js */
export function EventsOnMultiple(eventName, callback, maxCallbacks) {
  return _Events.OnMultiple(eventName, callback, maxCallbacks);
}

/** @deprecated Use Events.Once from /wails/runtime.js */
export function EventsOnce(eventName, callback) {
  return _Events.Once(eventName, callback);
}

/** @deprecated Use Events.Off from /wails/runtime.js */
export function EventsOff(...eventNames) {
  return _Events.Off(...eventNames);
}

/** @deprecated Use Events.OffAll from /wails/runtime.js */
export function EventsOffAll() {
  return _Events.OffAll();
}

/** @deprecated Use Events.Emit from /wails/runtime.js */
export function EventsEmit(eventName, data) {
  return _Events.Emit(eventName, data);
}

/** @deprecated Use Browser.OpenURL from /wails/runtime.js */
export function BrowserOpenURL(url) {
  return _Browser.OpenURL(url);
}

/** @deprecated Use Clipboard.Text from /wails/runtime.js */
export function ClipboardGetText() {
  return _Clipboard.Text();
}

/** @deprecated Use Clipboard.SetText from /wails/runtime.js */
export function ClipboardSetText(text) {
  return _Clipboard.SetText(text);
}
