import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

// Mock the Wails boundary — BrowserOpenURL is the only call that must ever
// carry an external URL out of the WebView.
const { browserOpenURL } = vi.hoisted(() => ({ browserOpenURL: vi.fn() }));
vi.mock('../../wailsjs/runtime/runtime', () => ({ BrowserOpenURL: browserOpenURL }));

import { openExternal, isExternalUrl, installExternalLinkInterceptor } from './external-links';

let uninstall: (() => void) | null = null;

beforeEach(() => {
  browserOpenURL.mockClear();
  document.body.innerHTML = '';
});

afterEach(() => {
  uninstall?.();
  uninstall = null;
});

/** Build an anchor in the document and return it. */
function anchor(href: string, inner = 'link'): HTMLAnchorElement {
  const a = document.createElement('a');
  a.setAttribute('href', href);
  a.innerHTML = inner;
  document.body.appendChild(a);
  return a;
}

function clickOn(el: Element, type = 'click', button = 0): MouseEvent {
  const evt = new MouseEvent(type, { bubbles: true, cancelable: true, button });
  el.dispatchEvent(evt);
  return evt;
}

describe('isExternalUrl', () => {
  it('accepts http, https and mailto', () => {
    expect(isExternalUrl('https://example.com')).toBe(true);
    expect(isExternalUrl('http://example.com')).toBe(true);
    expect(isExternalUrl('mailto:a@b.com')).toBe(true);
  });

  it('rejects relative links and non-web schemes', () => {
    expect(isExternalUrl('/local/path')).toBe(false);
    expect(isExternalUrl('#anchor')).toBe(false);
    expect(isExternalUrl('javascript:alert(1)')).toBe(false);
    expect(isExternalUrl('file:///C:/secret.txt')).toBe(false);
    expect(isExternalUrl('')).toBe(false);
  });
});

describe('openExternal', () => {
  it('hands http(s) URLs to the OS default browser', () => {
    openExternal('https://example.com/page');
    expect(browserOpenURL).toHaveBeenCalledWith('https://example.com/page');
  });

  it('drops URLs that are not http, https or mailto', () => {
    openExternal('javascript:alert(1)');
    expect(browserOpenURL).not.toHaveBeenCalled();
  });

  it('opens the same URL only once when a single activation is delivered twice', () => {
    vi.useFakeTimers();
    try {
      openExternal('https://github.com/o/r/pull/7');
      openExternal('https://github.com/o/r/pull/7');
      expect(browserOpenURL).toHaveBeenCalledTimes(1);
    } finally {
      vi.useRealTimers();
    }
  });

  it('opens the same URL again after the duplicate window has passed', () => {
    vi.useFakeTimers();
    try {
      openExternal('https://github.com/o/r/pull/8');
      vi.advanceTimersByTime(1000);
      openExternal('https://github.com/o/r/pull/8');
      expect(browserOpenURL).toHaveBeenCalledTimes(2);
    } finally {
      vi.useRealTimers();
    }
  });

  it('does not swallow a different URL opened right after', () => {
    vi.useFakeTimers();
    try {
      openExternal('https://example.com/a');
      openExternal('https://example.com/b');
      expect(browserOpenURL).toHaveBeenCalledTimes(2);
    } finally {
      vi.useRealTimers();
    }
  });
});

describe('installExternalLinkInterceptor', () => {
  it('routes an external anchor click to the default browser instead of the WebView', () => {
    uninstall = installExternalLinkInterceptor();
    const a = anchor('https://example.com');

    const evt = clickOn(a);

    expect(browserOpenURL).toHaveBeenCalledWith('https://example.com');
    expect(evt.defaultPrevented).toBe(true);
  });

  it('intercepts clicks on elements nested inside the anchor', () => {
    uninstall = installExternalLinkInterceptor();
    const a = anchor('https://example.com/nested', '<span><b>deep</b></span>');

    clickOn(a.querySelector('b')!);

    expect(browserOpenURL).toHaveBeenCalledWith('https://example.com/nested');
  });

  it('intercepts target="_blank" anchors, which WebView2 would open in a popup window', () => {
    uninstall = installExternalLinkInterceptor();
    const a = anchor('https://example.com/blank');
    a.setAttribute('target', '_blank');

    const evt = clickOn(a);

    expect(browserOpenURL).toHaveBeenCalledWith('https://example.com/blank');
    expect(evt.defaultPrevented).toBe(true);
  });

  it('routes middle-click (auxclick) too', () => {
    uninstall = installExternalLinkInterceptor();
    const a = anchor('https://example.com/aux');

    const evt = clickOn(a, 'auxclick', 1);

    expect(browserOpenURL).toHaveBeenCalledWith('https://example.com/aux');
    expect(evt.defaultPrevented).toBe(true);
  });

  it('leaves relative links to the app itself', () => {
    uninstall = installExternalLinkInterceptor();
    const a = anchor('/some/route');

    const evt = clickOn(a);

    expect(browserOpenURL).not.toHaveBeenCalled();
    expect(evt.defaultPrevented).toBe(false);
  });

  it('ignores right-clicks so the context menu still works', () => {
    uninstall = installExternalLinkInterceptor();
    // Distinct URL per test: openExternal suppresses repeats of the same URL,
    // which would make a "not called" assertion pass for the wrong reason.
    const a = anchor('https://example.com/right-click');

    const evt = clickOn(a, 'auxclick', 2);

    expect(browserOpenURL).not.toHaveBeenCalled();
    expect(evt.defaultPrevented).toBe(false);
  });

  it('stops intercepting once uninstalled', () => {
    const stop = installExternalLinkInterceptor();
    stop();
    const a = anchor('https://example.com/uninstalled');

    clickOn(a);

    expect(browserOpenURL).not.toHaveBeenCalled();
  });
});
