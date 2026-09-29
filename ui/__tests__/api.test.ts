import { describe, it, expect, vi, beforeEach } from 'vitest';
import axios from 'axios';

// Mock the axios instance used by api.ts — must happen before importing api.ts
vi.mock('axios', async (importOriginal) => {
  const actual = await importOriginal<typeof import('axios')>();
  return {
    ...actual,
    default: {
      ...actual.default,
      create: () => ({
        get: vi.fn(),
      }),
    },
  };
});

// Dynamically import after mock is set up
const { api, API_BASE_URL } = await import('../lib/api');

describe('API_BASE_URL', () => {
  it('is /api (Next.js proxy)', () => {
    expect(API_BASE_URL).toBe('/api');
  });
});

// ── transformBlockSummary (via api.listBlocks) ────────────────────────────────

describe('transformBlockSummary', () => {
  it('maps block_num to block_number and tx_count to transaction_count', async () => {
    const mockGet = vi.fn().mockResolvedValueOnce({
      data: {
        blocks: [
          {
            block_num: 7,
            tx_count: 3,
            block_size: 1024,
            created_at: '2024-01-01T00:00:00Z',
            previous_hash: null,
            data_hash: 'aabbcc',
            metadata_signatures: null,
            last_config_index: null,
            tx_status_codes: ['COMMITTED'],
            commit_hash: 'deadbeef',
          },
        ],
        offset: 0,
        limit: 10,
        has_more: false,
      },
    });

    // Patch the underlying apiClient.get for this test
    const apiClient = (await import('../lib/api')).default;
    (apiClient.get as ReturnType<typeof vi.fn>) = mockGet;

    const blocks = await api.listBlocks({ offset: 0, limit: 10 });
    expect(blocks).toHaveLength(1);
    expect(blocks[0].block_number).toBe(7);
    expect(blocks[0].transaction_count).toBe(3);
    expect(blocks[0].previous_hash).toBeNull();
  });
});

// ── transformTransaction (shape checks) ──────────────────────────────────────

describe('transformTransaction mapping', () => {
  it('maps block_num → block_number and tx_num → tx_index', async () => {
    const mockGet = vi.fn().mockResolvedValueOnce({
      data: {
        block_num: 5,
        tx_num: 2,
        tx_id: 'deadbeef',
        validation_code: 'COMMITTED',
        tx_type: null,
        chaincode_name: 'basic',
        creator_msp_id: 'Org1MSP',
        creator_identity: null,
        creator_nonce: null,
        envelope_signature: null,
        payload_extension: null,
        channel_version: null,
        channel_id: 'mychannel',
        epoch: null,
        tls_cert_hash: null,
        created_at: '2024-01-01T00:00:00Z',
        metadata: null,
        namespaces: [],
        read_writes: [],
        blind_writes: [],
        reads_only: [],
        endorsements: [],
      },
    });

    const apiClient = (await import('../lib/api')).default;
    (apiClient.get as ReturnType<typeof vi.fn>) = mockGet;

    const tx = await api.getTransaction('deadbeef');
    expect(tx.block_number).toBe(5);
    expect(tx.tx_index).toBe(2);
    expect(tx.chaincode_name).toBe('basic');
  });

  it('maps null chaincode_name to null', async () => {
    const mockGet = vi.fn().mockResolvedValueOnce({
      data: {
        block_num: 1, tx_num: 0, tx_id: 'aa', validation_code: 'VALID',
        tx_type: null, chaincode_name: null, creator_msp_id: null,
        creator_identity: null, creator_nonce: null, envelope_signature: null,
        payload_extension: null, channel_version: null, channel_id: '',
        epoch: null, tls_cert_hash: null, created_at: '', metadata: null,
        namespaces: [], read_writes: [], blind_writes: [], reads_only: [], endorsements: [],
      },
    });

    const apiClient = (await import('../lib/api')).default;
    (apiClient.get as ReturnType<typeof vi.fn>) = mockGet;

    const tx = await api.getTransaction('aa');
    expect(tx.chaincode_name).toBeNull();
  });
});

// ── api.getPolicies ────────────────────────────────────────────────────────────

describe('api.getPolicies', () => {
  it('returns empty array for empty policies response', async () => {
    const mockGet = vi.fn().mockResolvedValueOnce({
      data: { policies: [] },
    });

    const apiClient = (await import('../lib/api')).default;
    (apiClient.get as ReturnType<typeof vi.fn>) = mockGet;

    const policies = await api.getPolicies('myns');
    expect(policies).toEqual([]);
  });

  it('maps policy fields correctly', async () => {
    const mockGet = vi.fn().mockResolvedValueOnce({
      data: {
        policies: [{
          namespace: 'basic',
          version: 1,
          policy: 'OutOf(1, "Org1MSP.member")',
          certificates: ['cert1'],
          msp_ids: ['Org1MSP'],
          endpoints: ['peer0.org1.example.com:7051'],
          hash_algorithm: 'SHA256',
        }],
      },
    });

    const apiClient = (await import('../lib/api')).default;
    (apiClient.get as ReturnType<typeof vi.fn>) = mockGet;

    const policies = await api.getPolicies('basic');
    expect(policies).toHaveLength(1);
    expect(policies[0].namespace).toBe('basic');
    expect(policies[0].msp_ids).toEqual(['Org1MSP']);
  });
});
