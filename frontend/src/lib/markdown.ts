/**
 * Lightweight markdown renderer for chat messages.
 * Converts markdown to HTML with syntax highlighting for code blocks.
 */
import hljs from 'highlight.js';

/** Escape HTML special characters */
function escapeHtml(text: string): string {
  return text
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;');
}

/**
 * Render a markdown string to safe HTML.
 * Supports: fenced code blocks (with highlight.js), inline code,
 * bold, italic, links, headers (h1-h3), unordered/ordered lists.
 */
export function renderMarkdown(md: string): string {
  if (!md) return '';

  const lines = md.split('\n');
  const out: string[] = [];
  let i = 0;
  let inList: 'ul' | 'ol' | null = null;

  while (i < lines.length) {
    const line = lines[i];

    // Fenced code block: ```lang
    if (line.trimStart().startsWith('```')) {
      // Close any open list
      if (inList) { out.push(inList === 'ul' ? '</ul>' : '</ol>'); inList = null; }

      const lang = line.trimStart().slice(3).trim();
      const codeLines: string[] = [];
      i++;
      while (i < lines.length && !lines[i].trimStart().startsWith('```')) {
        codeLines.push(lines[i]);
        i++;
      }
      i++; // skip closing ```

      const code = codeLines.join('\n');
      let highlighted: string;
      if (lang && hljs.getLanguage(lang)) {
        highlighted = hljs.highlight(code, { language: lang }).value;
      } else {
        highlighted = escapeHtml(code);
      }
      out.push(`<pre class="md-code-block"><code class="hljs${lang ? ` language-${escapeHtml(lang)}` : ''}">${highlighted}</code></pre>`);
      continue;
    }

    // Headers
    const headerMatch = line.match(/^(#{1,3})\s+(.+)$/);
    if (headerMatch) {
      if (inList) { out.push(inList === 'ul' ? '</ul>' : '</ol>'); inList = null; }
      const level = headerMatch[1].length;
      out.push(`<h${level} class="md-h${level}">${renderInline(headerMatch[2])}</h${level}>`);
      i++;
      continue;
    }

    // Unordered list item: - or *
    const ulMatch = line.match(/^(\s*)[*-]\s+(.+)$/);
    if (ulMatch) {
      if (inList !== 'ul') {
        if (inList) out.push('</ol>');
        out.push('<ul class="md-list">');
        inList = 'ul';
      }
      out.push(`<li>${renderInline(ulMatch[2])}</li>`);
      i++;
      continue;
    }

    // Ordered list item: 1.
    const olMatch = line.match(/^(\s*)\d+\.\s+(.+)$/);
    if (olMatch) {
      if (inList !== 'ol') {
        if (inList) out.push('</ul>');
        out.push('<ol class="md-list">');
        inList = 'ol';
      }
      out.push(`<li>${renderInline(olMatch[2])}</li>`);
      i++;
      continue;
    }

    // Close list if non-list line
    if (inList) {
      out.push(inList === 'ul' ? '</ul>' : '</ol>');
      inList = null;
    }

    // Empty line → break
    if (line.trim() === '') {
      out.push('<br/>');
      i++;
      continue;
    }

    // Regular paragraph
    out.push(`<p class="md-p">${renderInline(line)}</p>`);
    i++;
  }

  // Close any trailing list
  if (inList) {
    out.push(inList === 'ul' ? '</ul>' : '</ol>');
  }

  return out.join('\n');
}

/** Render inline markdown elements (bold, italic, code, links) */
function renderInline(text: string): string {
  let html = escapeHtml(text);

  // Inline code: `code`
  html = html.replace(/`([^`]+)`/g, '<code class="md-inline-code">$1</code>');

  // Bold: **text** or __text__
  html = html.replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>');
  html = html.replace(/__(.+?)__/g, '<strong>$1</strong>');

  // Italic: *text* or _text_
  html = html.replace(/\*(.+?)\*/g, '<em>$1</em>');
  html = html.replace(/(?<!\w)_(.+?)_(?!\w)/g, '<em>$1</em>');

  // Links: [text](url)
  html = html.replace(
    /\[([^\]]+)\]\(([^)]+)\)/g,
    '<a href="$2" class="md-link" target="_blank" rel="noopener">$1</a>'
  );

  return html;
}
