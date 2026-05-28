import { type ClassValue, clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

export function parseProtoNumber(value: string | number | null | undefined): number {
  if (typeof value === 'number') {
    return value;
  }

  if (typeof value === 'string') {
    const parsed = Number(value);
    return Number.isFinite(parsed) ? parsed : 0;
  }

  return 0;
}

export function formatNumber(value: number): string {
  return new Intl.NumberFormat('en-US').format(value);
}

export function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB'];
  const k = 1024;
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${(bytes / Math.pow(k, i)).toFixed(i === 0 ? 0 : 1)} ${units[i]}`;
}

export function truncateMiddle(value: string, start: number = 10, end: number = 8): string {
  if (!value) {
    return '';
  }

  if (value.length <= start + end + 3) {
    return value;
  }

  return `${value.slice(0, start)}...${value.slice(-end)}`;
}

export function formatHash(hash: string, length: number = 12): string {
  return truncateMiddle(hash, length, 6);
}

export function getValidationCodeText(code: string | number): string {
  // Backend now returns validation_code as a string (e.g. 'COMMITTED', 'VALID')
  if (typeof code === 'string') return code;
  const codes: Record<number, string> = {
    0: 'VALID',
    1: 'NIL_ENVELOPE',
    2: 'BAD_PAYLOAD',
    3: 'BAD_COMMON_HEADER',
    4: 'BAD_CREATOR_SIGNATURE',
    5: 'INVALID_ENDORSER_TRANSACTION',
    10: 'INVALID_CONFIG_TRANSACTION',
    11: 'UNSUPPORTED_TX_PAYLOAD',
    12: 'BAD_PROPOSAL_TXID',
    13: 'DUPLICATE_TXID',
    14: 'ENDORSEMENT_POLICY_FAILURE',
    15: 'MVCC_READ_CONFLICT',
  };
  return codes[code] ?? `UNKNOWN (${code})`;
}

export function getValidationTone(code: string | number): 'success' | 'warning' | 'error' | 'info' {
  const valid = typeof code === 'string' ? ['VALID', 'COMMITTED'] : [0];
  const warn  = typeof code === 'string' ? ['NIL_ENVELOPE'] : [1];
  if ((valid as (string | number)[]).includes(code)) return 'success';
  if ((warn  as (string | number)[]).includes(code)) return 'warning';
  return 'error';
}

export async function copyToClipboard(text: string): Promise<void> {
  if (typeof navigator !== 'undefined' && navigator.clipboard?.writeText) {
    await navigator.clipboard.writeText(text);
    return;
  }

  throw new Error('Clipboard API unavailable');
}

/**
 * Decodes a hex-encoded byte string (as returned by the backend for keys/values).
 * Returns { text, isReadable, isJson, jsonValue, raw } where:
 *   - isJson: true if the bytes parse as valid JSON (object/array)
 *   - jsonValue: the parsed JSON value (only set when isJson is true)
 *   - isReadable: true if the bytes are valid printable UTF-8 text
 *   - text: the decoded display string
 *   - raw: the original hex string
 */
export function decodeHexBytes(hex: string | null | undefined): {
  text: string;
  isReadable: boolean;
  isJson: boolean;
  jsonValue: unknown;
  raw: string;
} {
  if (!hex) return { text: '', isReadable: false, isJson: false, jsonValue: null, raw: '' };

  try {
    // Hex string → byte array
    const bytes: number[] = [];
    for (let i = 0; i + 1 < hex.length; i += 2) {
      bytes.push(parseInt(hex.slice(i, i + 2), 16));
    }

    // Try decoding as UTF-8
    const text = new TextDecoder('utf-8', { fatal: true }).decode(new Uint8Array(bytes));
    const trimmed = text.trim();

    // Try JSON parse first — most Fabric chaincodes store JSON state
    if ((trimmed.startsWith('{') || trimmed.startsWith('[')) ) {
      try {
        const jsonValue = JSON.parse(trimmed);
        return { text: trimmed, isReadable: true, isJson: true, jsonValue, raw: hex };
      } catch { /* not JSON */ }
    }

    // Check if it's plain printable text
    const isPrintable = /^[\x09\x0a\x0d\x20-\x7e\u00a0-\ufffd]*$/.test(text) && text.trim().length > 0;
    if (isPrintable) {
      return { text, isReadable: true, isJson: false, jsonValue: null, raw: hex };
    }
  } catch {
    // Not valid UTF-8
  }

  return { text: hex, isReadable: false, isJson: false, jsonValue: null, raw: hex };
}

export function pluralize(count: number, singular: string, plural?: string): string {
  if (count === 1) {
    return singular;
  }

  return plural ?? `${singular}s`;
}
