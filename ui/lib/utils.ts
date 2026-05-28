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

export function getValidationCodeText(code: number): string {
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
  return codes[code] || `UNKNOWN (${code})`;
}

export function getValidationTone(code: number): 'success' | 'warning' | 'error' | 'info' {
  if (code === 0) {
    return 'success';
  }

  if (code === 1) {
    return 'warning';
  }

  return 'error';
}

export async function copyToClipboard(text: string): Promise<void> {
  if (typeof navigator !== 'undefined' && navigator.clipboard?.writeText) {
    await navigator.clipboard.writeText(text);
    return;
  }

  throw new Error('Clipboard API unavailable');
}

export function pluralize(count: number, singular: string, plural?: string): string {
  if (count === 1) {
    return singular;
  }

  return plural ?? `${singular}s`;
}
