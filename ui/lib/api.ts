import axios from 'axios';

// Browser always hits the Next.js /api proxy — avoids CORS and host resolution issues in Docker
export const API_BASE_URL = '/api';
export const API_DISPLAY_URL = process.env.NEXT_PUBLIC_BACKEND_DISPLAY_URL || 'http://localhost:8080';

const apiClient = axios.create({
  baseURL: API_BASE_URL,
  timeout: 10000,
  headers: {
    'Content-Type': 'application/json',
  },
});

// ── Backend response shapes ──────────────────────────────────────────────────

interface RestReadOnly {
  ns_id: string;
  seq_num: number;
  key: string;
}

interface RestReadWrite {
  ns_id: string;
  seq_num: number;
  key: string;
  read_version: number | null;
  value: string | null;
}

interface RestBlindWrite {
  ns_id: string;
  seq_num: number;
  key: string;
  value: string | null;
}

interface RestEndorsement {
  ns_id: string;
  seq_num: number;
  msp_id: string;
  endorsement: string;
  identity: {
    mspid: string;
    certificate_id: string;
  } | null;
}

interface RestNamespace {
  ns_id: string;
  ns_version: number;
}

interface RestTransaction {
  block_num: number;
  tx_num: number;
  tx_id: string;
  validation_code: string;
  tx_type: string | null;
  chaincode_name: string | null;
  creator_msp_id: string | null;
  creator_identity: string | null;
  creator_nonce: string | null;
  envelope_signature: string | null;
  payload_extension: string | null;
  channel_version: number | null;
  channel_id: string;
  epoch: number | null;
  tls_cert_hash: string | null;
  created_at: string;
  metadata?: string | null;
  namespaces: RestNamespace[];
  read_writes?: RestReadWrite[];
  blind_writes?: RestBlindWrite[];
  reads_only?: RestReadOnly[];
  endorsements?: RestEndorsement[];
}

interface RestBlock {
  block_num: number;
  tx_count: number;
  block_size: number;
  created_at: string | null;
  previous_hash: string | null;
  data_hash: string;
  metadata_signatures: string | null;
  last_config_index: number | null;
  tx_status_codes: string[];
  commit_hash: string;
  transactions?: RestTransaction[];
  envelope_errors?: string[];
}

interface RestBlockListResponse {
  blocks: RestBlock[];
  offset: number;
  limit: number;
  has_more: boolean;
}

interface RestNamespacePolicy {
  namespace: string;
  version: number;
  policy: string;
  certificates: string[];
  msp_ids: string[];
  endpoints: string[];
  hash_algorithm: string;
}

interface RestPoliciesResponse {
  policies: RestNamespacePolicy[];
}

// ── Public types used by UI components ──────────────────────────────────────

export interface BlockSummary {
  block_number: number;
  previous_hash: string | null;
  data_hash: string;
  transaction_count: number;
  block_size: number;
  created_at: string | null;
  tx_status_codes: string[];
  commit_hash: string;
  metadata_signatures: string | null;
  last_config_index: number | null;
}

export interface Block extends BlockSummary {
  transactions: Transaction[];
  envelope_errors: string[];
}

export interface ReadWriteRecord {
  namespace: string;
  seq_num: number;
  key: string;
  read_version: number | null;
  value: string | null;
}

export interface ReadRecord {
  namespace: string;
  seq_num: number;
  key: string;
}

export interface BlindWriteRecord {
  namespace: string;
  seq_num: number;
  key: string;
  value: string | null;
}

export interface NamespaceRecord {
  ns_id: string;
  ns_version: number;
}

export interface EndorsementRecord {
  ns_id: string;
  seq_num: number;
  msp_id: string;
  endorsement: string;
  certificate_id: string;
}

export interface Transaction {
  tx_id: string;
  block_number: number;
  tx_index: number;
  validation_code: string;
  tx_type: string | null;
  chaincode_name: string | null;
  creator_msp_id: string | null;
  creator_identity: string | null;
  creator_nonce: string | null;
  envelope_signature: string | null;
  payload_extension: string | null;
  channel_version: number | null;
  channel_id: string;
  epoch: number | null;
  tls_cert_hash: string | null;
  created_at: string;
  metadata: string | null;
  namespaces: NamespaceRecord[];
  blind_writes: BlindWriteRecord[];
  endorsements: EndorsementRecord[];
  read_writes: ReadWriteRecord[];
  reads_only: ReadRecord[];
}

export interface NamespacePolicy {
  namespace: string;
  version: number;
  policy: string;
  certificates: string[];
  msp_ids: string[];
  endpoints: string[];
  hash_algorithm: string;
}

export interface BlockHeight {
  height: number;
}

export interface BlockPage {
  height: number;
  page: number;
  page_size: number;
  total_pages: number;
  highest_block: number;
  lowest_block: number;
  blocks: BlockSummary[];
}

// ── Transform helpers ────────────────────────────────────────────────────────

const transformBlockSummary = (b: RestBlock): BlockSummary => ({
  block_number: b.block_num,
  previous_hash: b.previous_hash ?? null,
  data_hash: b.data_hash,
  transaction_count: b.tx_count,
  block_size: b.block_size ?? 0,
  created_at: b.created_at ?? null,
  tx_status_codes: b.tx_status_codes ?? [],
  commit_hash: b.commit_hash ?? '',
  metadata_signatures: b.metadata_signatures ?? null,
  last_config_index: b.last_config_index ?? null,
});

const transformTransaction = (tx: RestTransaction): Transaction => ({
  tx_id: tx.tx_id,
  block_number: tx.block_num,
  tx_index: tx.tx_num,
  validation_code: tx.validation_code,
  tx_type: tx.tx_type ?? null,
  chaincode_name: tx.chaincode_name ?? null,
  creator_msp_id: tx.creator_msp_id ?? null,
  creator_identity: tx.creator_identity ?? null,
  creator_nonce: tx.creator_nonce ?? null,
  envelope_signature: tx.envelope_signature ?? null,
  payload_extension: tx.payload_extension ?? null,
  channel_version: tx.channel_version ?? null,
  channel_id: tx.channel_id ?? '',
  epoch: tx.epoch ?? null,
  tls_cert_hash: tx.tls_cert_hash ?? null,
  created_at: tx.created_at ?? '',
  metadata: tx.metadata ?? null,
  namespaces: (tx.namespaces ?? []).map((n) => ({ ns_id: n.ns_id, ns_version: n.ns_version })),
  read_writes: (tx.read_writes ?? []).map((w) => ({
    namespace: w.ns_id,
    seq_num: w.seq_num,
    key: w.key,
    read_version: w.read_version ?? null,
    value: w.value ?? null,
  })),
  blind_writes: (tx.blind_writes ?? []).map((w) => ({
    namespace: w.ns_id,
    seq_num: w.seq_num,
    key: w.key,
    value: w.value ?? null,
  })),
  reads_only: (tx.reads_only ?? []).map((r) => ({ namespace: r.ns_id, seq_num: r.seq_num, key: r.key })),
  endorsements: (tx.endorsements ?? []).map((e) => ({
    ns_id: e.ns_id,
    seq_num: e.seq_num,
    msp_id: e.msp_id,
    endorsement: e.endorsement,
    certificate_id: e.identity?.certificate_id ?? '',
  })),
});

const transformBlock = (b: RestBlock): Block => ({
  ...transformBlockSummary(b),
  transactions: (b.transactions ?? []).map((tx) => transformTransaction(tx)),
  envelope_errors: b.envelope_errors ?? [],
});

const transformPolicy = (p: RestNamespacePolicy): NamespacePolicy => ({
  namespace: p.namespace,
  version: p.version,
  policy: p.policy ?? '',
  certificates: p.certificates ?? [],
  msp_ids: p.msp_ids ?? [],
  endpoints: p.endpoints ?? [],
  hash_algorithm: p.hash_algorithm ?? '',
});

// ── API client ────────────────────────────────────────────────────────────────

export const api = {
  getBlockHeight: async (): Promise<BlockHeight> => {
    const res = await apiClient.get<{ height: number }>('/blocks/height');
    return { height: res.data.height };
  },

  healthCheck: async (): Promise<{ status: string }> => {
    await apiClient.get('/blocks/height');
    return { status: 'online' };
  },

  getBlock: async (blockNumber: number): Promise<Block> => {
    const res = await apiClient.get<RestBlock>(`/blocks/${blockNumber}`);
    return transformBlock(res.data);
  },

  getLatestBlocks: async (count: number = 8): Promise<BlockSummary[]> => {
    const { height } = await api.getBlockHeight();
    const offset = Math.max(0, height - count + 1);
    const res = await apiClient.get<RestBlockListResponse>('/blocks', { params: { offset, limit: count } });
    return (res.data.blocks ?? []).map(transformBlockSummary).reverse();
  },

  listBlocks: async (params: { offset: number; limit: number }): Promise<BlockSummary[]> => {
    const res = await apiClient.get<RestBlockListResponse>('/blocks', { params });
    return (res.data.blocks ?? []).map(transformBlockSummary);
  },

  getBlockPage: async (page: number, pageSize: number): Promise<BlockPage> => {
    const { height } = await api.getBlockHeight();
    const totalPages = Math.max(1, Math.ceil((height + 1) / pageSize));
    const highestBlock = height - page * pageSize;

    if (highestBlock < 0) {
      return { height, page, page_size: pageSize, total_pages: totalPages, highest_block: 0, lowest_block: 0, blocks: [] };
    }

    const lowestBlock = Math.max(0, highestBlock - pageSize + 1);
    const offset = lowestBlock;
    const limit = highestBlock - lowestBlock + 1;
    const res = await apiClient.get<RestBlockListResponse>('/blocks', { params: { offset, limit } });
    const blocks = (res.data.blocks ?? []).map(transformBlockSummary).reverse();

    return { height, page, page_size: pageSize, total_pages: totalPages, highest_block: highestBlock, lowest_block: lowestBlock, blocks };
  },

  getTransaction: async (txId: string): Promise<Transaction> => {
    const res = await apiClient.get<RestTransaction>(`/transactions/${txId}`);
    return transformTransaction(res.data);
  },

  getRecentTransactions: async (count: number = 12): Promise<Transaction[]> => {
    const { height } = await api.getBlockHeight();
    const transactions: Transaction[] = [];

    for (let n = height; n >= 0 && transactions.length < count; n--) {
      const block = await api.getBlock(n);
      if (block.transactions.length > 0) {
        transactions.push(...block.transactions);
      }
      if (height - n > 12) break;
    }

    return transactions.slice(0, count);
  },

  getPolicies: async (namespace: string): Promise<NamespacePolicy[]> => {
    const res = await apiClient.get<RestPoliciesResponse>(`/namespaces/${namespace}/policies`);
    return (res.data.policies ?? []).map(transformPolicy);
  },
};

export default apiClient;
