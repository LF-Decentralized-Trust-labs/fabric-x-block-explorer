import { describe, it, expect } from 'vitest';
import { decodePolicyBytes } from '../lib/policyDecoder';

describe('decodePolicyBytes', () => {
  it('returns null for empty input', () => {
    expect(decodePolicyBytes('')).toBeNull();
  });

  it('returns null for invalid base64', () => {
    expect(decodePolicyBytes('!!not-base64!!')).toBeNull();
  });

  it('returns a decoded policy object for valid base64 bytes', () => {
    // Encode a string that contains known policy keywords so the parser has
    // something to classify — we do not need a real proto payload here.
    const input = 'Admins\x00Writers\x00V2_0\x00SHA256';
    const b64 = btoa(input);
    const result = decodePolicyBytes(b64);
    expect(result).not.toBeNull();
    // The shape must match DecodedPolicy
    expect(Array.isArray(result!.organizations)).toBe(true);
    expect(Array.isArray(result!.policyRoles)).toBe(true);
    expect(Array.isArray(result!.capabilities)).toBe(true);
  });

  it('extracts known policy roles from payload', () => {
    // Build a buffer containing the ASCII string "Admins" surrounded by NUL bytes.
    const buf = new Uint8Array(20);
    const role = 'Admins';
    for (let i = 0; i < role.length; i++) buf[i + 2] = role.charCodeAt(i);
    const b64 = btoa(String.fromCharCode(...buf));
    const result = decodePolicyBytes(b64);
    expect(result).not.toBeNull();
    expect(result!.policyRoles).toContain('Admins');
  });

  it('extracts capability versions like V2_0', () => {
    const buf = new Uint8Array(20);
    const cap = 'V2_0';
    for (let i = 0; i < cap.length; i++) buf[i + 2] = cap.charCodeAt(i);
    const b64 = btoa(String.fromCharCode(...buf));
    const result = decodePolicyBytes(b64);
    expect(result).not.toBeNull();
    expect(result!.capabilities).toContain('V2_0');
  });

  it('returns empty collections for a payload with no known patterns', () => {
    // All zeroes — no printable strings.
    const b64 = btoa(String.fromCharCode(...new Uint8Array(16)));
    const result = decodePolicyBytes(b64);
    expect(result).not.toBeNull();
    expect(result!.organizations).toHaveLength(0);
    expect(result!.policyRoles).toHaveLength(0);
  });
});
