import { describe, it, expect } from 'vitest';
import {
  parseProtoNumber,
  formatBytes,
  getValidationCodeText,
  getValidationTone,
  truncateMiddle,
  decodeHexBytes,
} from '../lib/utils';

// ── parseProtoNumber ──────────────────────────────────────────────────────────

describe('parseProtoNumber', () => {
  it('returns a number as-is', () => {
    expect(parseProtoNumber(42)).toBe(42);
  });
  it('parses a numeric string', () => {
    expect(parseProtoNumber('42')).toBe(42);
  });
  it('returns 0 for null', () => {
    expect(parseProtoNumber(null)).toBe(0);
  });
  it('returns 0 for undefined', () => {
    expect(parseProtoNumber(undefined)).toBe(0);
  });
  it('returns 0 for non-numeric string', () => {
    expect(parseProtoNumber('abc')).toBe(0);
  });
});

// ── formatBytes ───────────────────────────────────────────────────────────────

describe('formatBytes', () => {
  it('returns "0 B" for zero', () => {
    expect(formatBytes(0)).toBe('0 B');
  });
  it('returns bytes for sub-kilobyte values', () => {
    expect(formatBytes(1023)).toBe('1023 B');
  });
  it('formats kilobytes', () => {
    expect(formatBytes(1024)).toBe('1.0 KB');
  });
  it('formats megabytes', () => {
    expect(formatBytes(1048576)).toBe('1.0 MB');
  });
});

// ── getValidationCodeText ─────────────────────────────────────────────────────

describe('getValidationCodeText', () => {
  it('returns string codes as-is', () => {
    expect(getValidationCodeText('COMMITTED')).toBe('COMMITTED');
    expect(getValidationCodeText('VALID')).toBe('VALID');
  });
  it('maps number 0 to VALID', () => {
    expect(getValidationCodeText(0)).toBe('VALID');
  });
  it('maps number 14 to ENDORSEMENT_POLICY_FAILURE', () => {
    expect(getValidationCodeText(14)).toBe('ENDORSEMENT_POLICY_FAILURE');
  });
  it('returns UNKNOWN for unknown numeric code', () => {
    expect(getValidationCodeText(999)).toBe('UNKNOWN (999)');
  });
});

// ── getValidationTone ─────────────────────────────────────────────────────────

describe('getValidationTone', () => {
  it('returns success for VALID', () => {
    expect(getValidationTone('VALID')).toBe('success');
  });
  it('returns success for COMMITTED', () => {
    expect(getValidationTone('COMMITTED')).toBe('success');
  });
  it('returns warning for NIL_ENVELOPE', () => {
    expect(getValidationTone('NIL_ENVELOPE')).toBe('warning');
  });
  it('returns error for ENDORSEMENT_POLICY_FAILURE', () => {
    expect(getValidationTone('ENDORSEMENT_POLICY_FAILURE')).toBe('error');
  });
  it('returns error for unknown string code', () => {
    expect(getValidationTone('UNKNOWN_CODE')).toBe('error');
  });
});

// ── truncateMiddle ────────────────────────────────────────────────────────────

describe('truncateMiddle', () => {
  it('leaves short strings unchanged', () => {
    expect(truncateMiddle('abc')).toBe('abc');
  });
  it('truncates long strings with ellipsis', () => {
    const long = 'a'.repeat(30);
    const result = truncateMiddle(long, 5, 5);
    expect(result).toContain('...');
    expect(result.length).toBeLessThan(long.length);
  });
  it('returns empty string for empty input', () => {
    expect(truncateMiddle('')).toBe('');
  });
});

// ── decodeHexBytes ────────────────────────────────────────────────────────────

describe('decodeHexBytes', () => {
  it('returns empty result for null', () => {
    const r = decodeHexBytes(null);
    expect(r.text).toBe('');
    expect(r.isReadable).toBe(false);
    expect(r.isJson).toBe(false);
    expect(r.raw).toBe('');
  });

  it('returns empty result for undefined', () => {
    const r = decodeHexBytes(undefined);
    expect(r.text).toBe('');
    expect(r.isReadable).toBe(false);
  });

  it('decodes valid UTF-8 hex to readable text', () => {
    // "hello" in hex
    const hex = Buffer.from('hello').toString('hex');
    const r = decodeHexBytes(hex);
    expect(r.isReadable).toBe(true);
    expect(r.text).toBe('hello');
    expect(r.isJson).toBe(false);
  });

  it('detects JSON hex values', () => {
    const json = JSON.stringify({ key: 'value' });
    const hex = Buffer.from(json).toString('hex');
    const r = decodeHexBytes(hex);
    expect(r.isJson).toBe(true);
    expect(r.isReadable).toBe(true);
    expect(r.jsonValue).toEqual({ key: 'value' });
  });

  it('returns hex as text for non-UTF-8 bytes', () => {
    // Bytes that are not valid UTF-8
    const r = decodeHexBytes('ff00ff00');
    expect(r.isReadable).toBe(false);
    expect(r.raw).toBe('ff00ff00');
  });

  it('handles odd-length hex gracefully without throwing', () => {
    // Odd length — last nibble is ignored by the loop
    expect(() => decodeHexBytes('abc')).not.toThrow();
  });
});
