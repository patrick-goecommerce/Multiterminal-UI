import { describe, it, expect } from 'vitest';
import { renderMarkdown } from './markdown';

describe('renderMarkdown link scheme allow-list', () => {
  it('renders http(s) links as clickable anchors', () => {
    const html = renderMarkdown('[click me](https://example.com)');
    expect(html).toContain('<a href="https://example.com"');
    expect(html).toContain('click me');
  });

  it('renders mailto links as clickable anchors', () => {
    const html = renderMarkdown('[mail](mailto:a@b.com)');
    expect(html).toContain('<a href="mailto:a@b.com"');
  });

  it('does not linkify javascript: URIs (XSS via {@html}-rendered chat content)', () => {
    const html = renderMarkdown('[click me](javascript:alert(1))');
    expect(html).not.toContain('<a href');
    expect(html).not.toContain('javascript:');
    expect(html).toContain('click me');
  });

  it('does not linkify data: URIs', () => {
    const html = renderMarkdown('[x](data:text/html,<script>alert(1)</script>)');
    expect(html).not.toContain('<a href');
  });

  it('does not linkify vbscript: URIs', () => {
    const html = renderMarkdown('[x](vbscript:msgbox(1))');
    expect(html).not.toContain('<a href');
  });
});
