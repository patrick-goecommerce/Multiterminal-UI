import { describe, it, expect } from 'vitest';
import { LINK_REGEX, LOCALHOST_REGEX, isUrl } from './links';

describe('LINK_REGEX', () => {
  const match = (text: string) => {
    LINK_REGEX.lastIndex = 0;
    const m = LINK_REGEX.exec(text);
    return m ? m[0] : null;
  };

  it('matches https URLs', () => {
    expect(match('visit https://example.com for info')).toBe('https://example.com');
  });

  it('matches http URLs with paths', () => {
    expect(match('see http://localhost:3000/api/v1')).toBe('http://localhost:3000/api/v1');
  });

  it('matches relative file paths with extension', () => {
    expect(match('error in ./src/App.svelte')).toBe('./src/App.svelte');
  });

  it('matches file paths with line number', () => {
    expect(match('at src/utils/parse.ts:42')).toBe('src/utils/parse.ts:42');
  });

  it('matches file paths with line:col', () => {
    expect(match('error src/index.ts:10:5')).toBe('src/index.ts:10:5');
  });

  it('matches Windows paths', () => {
    expect(match('file C:\\Users\\foo\\bar.ts')).toBe('C:\\Users\\foo\\bar.ts');
  });

  it('matches parent-relative paths', () => {
    expect(match('see ../lib/helper.go:99')).toBe('../lib/helper.go:99');
  });

  it('matches absolute Unix paths', () => {
    expect(match('at /usr/local/lib/foo.go:10')).toBe('/usr/local/lib/foo.go:10');
  });

  it('does not include trailing period in URL', () => {
    expect(match('Visit https://example.com.')).toBe('https://example.com');
  });

  it('does not include trailing comma in URL', () => {
    expect(match('see https://example.com, more')).toBe('https://example.com');
  });

  it('does not match plain words', () => {
    expect(match('hello world')).toBeNull();
  });
});

describe('LOCALHOST_REGEX', () => {
  const matchAll = (text: string) => {
    LOCALHOST_REGEX.lastIndex = 0;
    return [...text.matchAll(LOCALHOST_REGEX)].map(m => m[0]);
  };

  it('matches http://localhost with port', () => {
    expect(matchAll('Server at http://localhost:3000')).toEqual(['http://localhost:3000']);
  });

  it('matches http://127.0.0.1 with port', () => {
    expect(matchAll('Running on http://127.0.0.1:8080')).toEqual(['http://127.0.0.1:8080']);
  });

  it('matches http://0.0.0.0 with port', () => {
    expect(matchAll('Listening on http://0.0.0.0:5173')).toEqual(['http://0.0.0.0:5173']);
  });

  it('matches https://localhost with port', () => {
    expect(matchAll('Secure at https://localhost:443')).toEqual(['https://localhost:443']);
  });

  it('matches localhost URL with path', () => {
    expect(matchAll('API at http://localhost:3000/api/v1')).toEqual(['http://localhost:3000/api/v1']);
  });

  it('does not include trailing period', () => {
    expect(matchAll('Visit http://localhost:3000.')).toEqual(['http://localhost:3000']);
  });

  it('does not include trailing comma', () => {
    expect(matchAll('at http://localhost:3000, then')).toEqual(['http://localhost:3000']);
  });

  it('matches multiple localhost URLs', () => {
    expect(matchAll('http://localhost:3000 and http://localhost:3001'))
      .toEqual(['http://localhost:3000', 'http://localhost:3001']);
  });

  it('does not match localhost without port', () => {
    expect(matchAll('http://localhost/path')).toEqual([]);
  });

  it('does not match non-localhost URLs', () => {
    expect(matchAll('https://example.com:3000')).toEqual([]);
  });
});

describe('isUrl', () => {
  it('returns true for http', () => {
    expect(isUrl('http://example.com')).toBe(true);
  });

  it('returns true for https', () => {
    expect(isUrl('https://github.com/foo')).toBe(true);
  });

  it('returns false for file paths', () => {
    expect(isUrl('./src/foo.ts')).toBe(false);
  });

  it('returns false for relative paths', () => {
    expect(isUrl('src/foo.ts:42')).toBe(false);
  });
});
