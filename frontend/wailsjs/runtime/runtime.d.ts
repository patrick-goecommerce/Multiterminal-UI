// Wails v3 runtime type declarations
// Provides both v3 native exports and v2-compatible helper functions.

// --- Wails v3 WailsEvent ---
export interface WailsEvent {
  name: string;
  data: any;
  sender?: string;
}

// --- Wails v3 Events namespace ---
export declare namespace Events {
  function On(eventName: string, callback: (event: WailsEvent) => void): () => void;
  function OnMultiple(eventName: string, callback: (event: WailsEvent) => void, maxCallbacks: number): () => void;
  function Once(eventName: string, callback: (event: WailsEvent) => void): () => void;
  function Off(...eventNames: string[]): void;
  function OffAll(): void;
  function Emit(eventName: string, data?: any): void;
}

// --- Wails v3 Browser namespace ---
export declare namespace Browser {
  function OpenURL(url: string): void;
}

// --- Wails v3 Clipboard namespace ---
export declare namespace Clipboard {
  function Text(): Promise<string>;
  function SetText(text: string): Promise<void>;
}

// --- Wails v3 Window ---
export declare namespace Window {
  function Get(name: string): WindowInstance;
  function Center(): Promise<void>;
  function Close(): Promise<void>;
  function Focus(): Promise<void>;
  function Fullscreen(): Promise<void>;
  function Hide(): Promise<void>;
  function Maximise(): Promise<void>;
  function Minimise(): Promise<void>;
  function Name(): Promise<string>;
  function Reload(): Promise<void>;
  function Restore(): Promise<void>;
  function SetTitle(title: string): Promise<void>;
  function Show(): Promise<void>;
  function Size(): Promise<{width: number; height: number}>;
  function Height(): Promise<number>;
  function Width(): Promise<number>;
  function ToggleMaximise(): Promise<void>;
  function UnMaximise(): Promise<void>;
  function UnMinimise(): Promise<void>;
  function UnFullscreen(): Promise<void>;
  function SetSize(width: number, height: number): Promise<void>;
  function SetMinSize(width: number, height: number): Promise<void>;
  function SetMaxSize(width: number, height: number): Promise<void>;
  function SetAlwaysOnTop(alwaysOnTop: boolean): Promise<void>;
  function SetPosition(x: number, y: number): Promise<void>;
  function SetRelativePosition(x: number, y: number): Promise<void>;
  function Position(): Promise<{x: number; y: number}>;
  function RelativePosition(): Promise<{x: number; y: number}>;
  function OpenDevTools(): Promise<void>;
  function SetBackgroundColour(r: number, g: number, b: number, a: number): Promise<void>;
  function SetZoom(zoom: number): Promise<void>;
  function GetZoom(): Promise<number>;
  function ZoomIn(): Promise<void>;
  function ZoomOut(): Promise<void>;
  function ZoomReset(): Promise<void>;
  function IsMaximised(): Promise<boolean>;
  function IsMinimised(): Promise<boolean>;
  function IsFullscreen(): Promise<boolean>;
  function IsFocused(): Promise<boolean>;
  function GetScreen(): Promise<any>;
  function Print(): Promise<void>;
}

export interface WindowInstance {
  Center(): Promise<void>;
  Close(): Promise<void>;
  Focus(): Promise<void>;
  Hide(): Promise<void>;
  Show(): Promise<void>;
  SetTitle(title: string): Promise<void>;
  SetSize(width: number, height: number): Promise<void>;
  Maximise(): Promise<void>;
  UnMaximise(): Promise<void>;
  Minimise(): Promise<void>;
  UnMinimise(): Promise<void>;
  ToggleMaximise(): Promise<void>;
}

// --- Wails v3 Application namespace ---
export declare namespace Application {
  function Hide(): Promise<void>;
  function Show(): Promise<void>;
  function Quit(): Promise<void>;
}

// --- Wails v3 Screens namespace ---
export declare namespace Screens {
  function GetAll(): Promise<any[]>;
  function GetCurrent(): Promise<any>;
  function GetPrimary(): Promise<any>;
}

// --- Wails v3 Dialogs namespace ---
export declare namespace Dialogs {
  function Info(options: any): Promise<void>;
  function Warning(options: any): Promise<void>;
  function Error(options: any): Promise<void>;
  function Question(options: any): Promise<string>;
  function OpenFile(options: any): Promise<string[]>;
  function SaveFile(options: any): Promise<string>;
}

// --- Wails v3 System namespace ---
export declare namespace System {
  function IsDarkMode(): Promise<boolean>;
  function Environment(): Promise<any>;
  function Capabilities(): Promise<any>;
}

// --- Wails v3 Flags namespace ---
export declare namespace Flags {
  function GetFlag(name: string): any;
}

// --- Wails v2 compatibility helpers ---
// These are kept for backward compatibility. New code should use the namespace APIs above.

/** @deprecated Use Events.On */
export function EventsOn(eventName: string, callback: (event: WailsEvent) => void): () => void;

/** @deprecated Use Events.OnMultiple */
export function EventsOnMultiple(eventName: string, callback: (event: WailsEvent) => void, maxCallbacks: number): () => void;

/** @deprecated Use Events.Once */
export function EventsOnce(eventName: string, callback: (event: WailsEvent) => void): () => void;

/** @deprecated Use Events.Off */
export function EventsOff(...eventNames: string[]): void;

/** @deprecated Use Events.OffAll */
export function EventsOffAll(): void;

/** @deprecated Use Events.Emit */
export function EventsEmit(eventName: string, data?: any): void;

/** @deprecated Use Browser.OpenURL */
export function BrowserOpenURL(url: string): void;

/** @deprecated Use Clipboard.Text */
export function ClipboardGetText(): Promise<string>;

/** @deprecated Use Clipboard.SetText */
export function ClipboardSetText(text: string): Promise<void>;
