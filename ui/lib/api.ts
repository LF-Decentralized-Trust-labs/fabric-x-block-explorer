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

interface RestRead {
  id: number;
  ns_id: string;
  key: string;
  is_read_write: boolean;
}

interface RestWrite {
  id: number;
  ns_id: string;
  key: string;
  value: string;
  is_blind_write: boolean;
}

interface RestEndorsement {
  id: number;
  ns_id: string;
  endorsement: string;
}

interface RestTransaction {
  id: number;
  tx_num: number;
  tx_id: string;
  validation_code: number;
  reads?: RestRead[];
  writes?: RestWrite[];
  endorsements?: RestEndorsement[];
}

interface RestBlock {
  block_num: number;
  tx_count: number;
  previous_hash: string;
  data_hash: string;
  transactions?: RestTransaction[];
}

interface RestTxWithBlock {
  transaction: RestTransaction;
  block: {
    block_num: number;
    tx_count: number;
    previous_hash: string;
    data_hash: string;
  };
}

interface RestNamespacePolicy {
  id: number;
  namespace: string;
  version: number;
  policy: { policy_bytes?: string } | null;
}

// ── Public types used by UI components ──────────────────────────────────────

export interface BlockSummary {
  block_number: number;
  previous_hash: string;
  data_hash: string;
  transaction_count: number;
}

export interface Block extends BlockSummary {
  transactions: Transaction[];
}

export interface ReadWriteRecord {
  namespace: string;
  key: string;
  value: string;
}

export interface ReadRecord {
  namespace: string;
  key: string;
}

export interface BlindWriteRecord {
  namespace: string;
  key: string;
  value: string;
}

export interface EndorsementRecord {
  namespace: string;
  endorsement: string;
  identity: string;
  msp_id?: string;
}

export interface Transaction {
  tx_id: string;
  block_number: number;
  tx_index: number;
  validation_code: number;
  blind_writes: BlindWriteRecord[];
  endorsements: EndorsementRecord[];
  read_writes: ReadWriteRecord[];
  reads_only: ReadRecord[];
}

export interface NamespacePolicy {
  namespace: string;
  version: number;
  policy_bytes: string;
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
  previous_hash: b.previous_hash,
  data_hash: b.data_hash,
  transaction_count: b.tx_count,
});

const transformTransaction = (tx: RestTransaction, blockNum: number): Transaction => ({
  tx_id: tx.tx_id,
  block_number: blockNum,
  tx_index: tx.tx_num,
  validation_code: tx.validation_code,
  read_writes: (tx.writes ?? [])
    .filter((w) => !w.is_blind_write)
    .map((w) => ({ namespace: w.ns_id, key: w.key, value: w.value })),
  blind_writes: (tx.writes ?? [])
    .filter((w) => w.is_blind_write)
    .map((w) => ({ namespace: w.ns_id, key: w.key, value: w.value })),
  reads_only: (tx.reads ?? [])
    .filter((r) => !r.is_read_write)
    .map((r) => ({ namespace: r.ns_id, key: r.key })),
  endorsements: (tx.endorsements ?? []).map((e) => ({
    namespace: e.ns_id,
    endorsement: e.endorsement,
    identity: '',
  })),
});

const transformBlock = (b: RestBlock): Block => ({
  ...transformBlockSummary(b),
  transactions: (b.transactions ?? []).map((tx) => transformTransaction(tx, b.block_num)),
});

const transformPolicy = (p: RestNamespacePolicy): NamespacePolicy => ({
  namespace: p.namespace,
  version: p.version,
  policy_bytes: p.policy?.policy_bytes ?? '',
});

// ── API client ────────────────────────────────────────────────────────────────

export const api = {
  getBlockHeight: async (): Promise<BlockHeight> => {
    const res = await apiClient.get<{ height: number }>('/blocks/height');
    return { height: res.data.height };
  },

  healthCheck: async (): Promise<{ status: string }> => {
    await apiClient.get('/healthz');
    return { status: 'online' };
  },

  getBlock: async (
    blockNumber: number,
    options?: { txLimit?: number; txOffset?: number }
  ): Promise<Block> => {
    const res = await apiClient.get<RestBlock>(`/blocks/${blockNumber}`, {
      params: {
        limitTx: options?.txLimit,
        offsetTx: options?.txOffset,
      },
    });
    return transformBlock(res.data);
  },

  getLatestBlocks: async (count: number = 8): Promise<BlockSummary[]> => {
    const { height } = await api.getBlockHeight();
    const from = Math.max(0, height - count + 1);
    const fetches = [];
    for (let n = height; n >= from; n--) {
      fetches.push(apiClient.get<RestBlock>(`/blocks/${n}`));
    }
    const results = await Promise.all(fetches);
    return results.map((r) => transformBlockSummary(r.data));
  },

  listBlocks: async (params: { offset: number; limit: number }): Promise<BlockSummary[]> => {
    const fetches = [];
    for (let n = params.offset + params.limit - 1; n >= params.offset; n--) {
      fetches.push(apiClient.get<RestBlock>(`/blocks/${n}`));
    }
    const results = await Promise.all(fetches);
    return results.map((r) => transformBlockSummary(r.data));
  },

  getBlockPage: async (page: number, pageSize: number): Promise<BlockPage> => {
    const { height } = await api.getBlockHeight();
    const totalPages = Math.max(1, Math.ceil((height + 1) / pageSize));
    const highestBlock = height - page * pageSize;

    if (highestBlock < 0) {
      return { height, page, page_size: pageSize, total_pages: totalPages, highest_block: 0, lowest_block: 0, blocks: [] };
    }

    const lowestBlock = Math.max(0, highestBlock - pageSize + 1);
    const fetches = [];
    for (let n = highestBlock; n >= lowestBlock; n--) {
      fetches.push(apiClient.get<RestBlock>(`/blocks/${n}`));
    }
    const results = await Promise.all(fetches);
    const blocks = results.map((r) => transformBlockSummary(r.data));

    return { height, page, page_size: pageSize, total_pages: totalPages, highest_block: highestBlock, lowest_block: lowestBlock, blocks };
  },

  getTransaction: async (txId: string): Promise<Transaction> => {
    const res = await apiClient.get<RestTxWithBlock>(`/tx/${txId}`);
    return transformTransaction(res.data.transaction, res.data.block.block_num);
  },

  getRecentTransactions: async (count: number = 12): Promise<Transaction[]> => {
    const { height } = await api.getBlockHeight();
    const transactions: Transaction[] = [];

    for (let n = height; n >= 0 && transactions.length < count; n--) {
      const block = await api.getBlock(n, { txLimit: Math.max(1, count - transactions.length) });
      if (block.transactions.length > 0) {
        transactions.push(...block.transactions);
      }
      if (height - n > 12) break;
    }

    return transactions.slice(0, count);
  },

  getPolicies: async (namespace: string): Promise<NamespacePolicy[]> => {
    const res = await apiClient.get<RestNamespacePolicy[]>(`/policies/${namespace}`);
    return (res.data ?? []).map(transformPolicy);
  },
};

export default apiClient;
