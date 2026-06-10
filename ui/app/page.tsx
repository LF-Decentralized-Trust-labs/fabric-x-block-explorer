 'use client';

import { useEffect, useMemo, useState } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import {
  Activity,
  ArrowDownUp,
  ArrowRight,
  ChevronDown,
  ChevronUp,
  Database,
  FileSearch,
  Layers,
  Search,
  Shield,
} from 'lucide-react';
import {
  ResponsiveContainer,
  BarChart,
  Bar,
  CartesianGrid,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts';
import { api, BlockSummary, NamespacePolicy, Transaction } from '@/lib/api';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/Card';
import { Button } from '@/components/ui/Button';
import { LoadingSpinner } from '@/components/ui/Loading';
import { SearchInput } from '@/components/ui/SearchInput';
import { Badge } from '@/components/ui/Badge';
import { MetricCard } from '@/components/explorer/MetricCard';
import { HashValue } from '@/components/explorer/HashValue';
import { EmptyState } from '@/components/explorer/EmptyState';
import {
  formatNumber,
  getValidationCodeText,
  getValidationTone,
  pluralize,
} from '@/lib/utils';

type DashboardState = {
  height: number;
  latestBlocks: BlockSummary[];
  recentTransactions: Transaction[];
  metaPolicies: NamespacePolicy[];
  namespacePolicies: NamespacePolicy[];
};

type LatestBlocksSortField = 'block_number' | 'transaction_count';
type LatestBlocksSortDirection = 'asc' | 'desc';

export default function Dashboard() {
  const [loading, setLoading] = useState(true);
  const [data, setData] = useState<DashboardState | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [blockQuery, setBlockQuery] = useState('');
  const [txQuery, setTxQuery] = useState('');
  const [namespaceQuery, setNamespaceQuery] = useState('_meta');
  const [latestBlocksSortField, setLatestBlocksSortField] = useState<LatestBlocksSortField>('block_number');
  const [latestBlocksSortDirection, setLatestBlocksSortDirection] = useState<LatestBlocksSortDirection>('desc');
  const router = useRouter();

  useEffect(() => {
    void fetchData();
    const interval = setInterval(() => {
      void fetchData();
    }, 15000);

    return () => clearInterval(interval);
  }, []);

  const fetchData = async () => {
    try {
      const [heightData, latestBlocks, recentTransactions, metaPolicies, namespacePolicies] = await Promise.all([
        api.getBlockHeight(),
        api.getLatestBlocks(5),
        api.getRecentTransactions(12),
        api.getPolicies('_meta').catch(() => []),
        api.getPolicies('0').catch(() => []),
      ]);

      setData({
        height: heightData.height,
        latestBlocks,
        recentTransactions,
        metaPolicies,
        namespacePolicies,
      });
      setError(null);
    } catch (err) {
      setError('Failed to fetch data from the running explorer backend.');
      console.error(err);
    } finally {
      setLoading(false);
    }
  };

  const handleBlockSearch = () => {
    const blockNumber = Number(blockQuery.trim());
    if (Number.isInteger(blockNumber) && blockNumber >= 0) {
      router.push(`/blocks/${blockNumber}`);
    }
  };

  const handleTransactionSearch = () => {
    const value = txQuery.trim();
    if (value) {
      router.push(`/transactions/${value}`);
    }
  };

  const handleNamespaceSearch = () => {
    const value = namespaceQuery.trim();
    if (value) {
      router.push(`/policies?namespace=${encodeURIComponent(value)}`);
    }
  };

  const latestBlocks = data?.latestBlocks ?? [];
  const recentTransactions = data?.recentTransactions ?? [];
  const metaPolicies = data?.metaPolicies ?? [];
  const namespacePolicies = data?.namespacePolicies ?? [];

  const sortedLatestBlocks = useMemo(() => {
    return [...latestBlocks].sort((left, right) => {
      const delta = left[latestBlocksSortField] - right[latestBlocksSortField];
      return latestBlocksSortDirection === 'asc' ? delta : -delta;
    });
  }, [latestBlocks, latestBlocksSortDirection, latestBlocksSortField]);

  if (loading) {
    return <LoadingSpinner />;
  }

  if (error || !data) {
    return (
      <div className="space-y-6">
        <Card>
          <CardContent className="py-12">
            <EmptyState
              icon={Database}
              title="Explorer backend unavailable"
              description={
                error ||
                'Unable to load blockchain data. Confirm the fabric-x-block-explorer service is reachable on localhost:8080.'
              }
            />
          </CardContent>
        </Card>
      </div>
    );
  }

  const latestBlock = latestBlocks[0];

  const chartData = [...latestBlocks].reverse().map((block) => ({
    block: `#${block.block_number}`,
    transactions: block.transaction_count,
  }));

  const validationBreakdown = recentTransactions.reduce<Record<string, number>>((acc, tx) => {
    const label = getValidationCodeText(tx.validation_code);
    acc[label] = (acc[label] ?? 0) + 1;
    return acc;
  }, {});

  const validationEntries = Object.entries(validationBreakdown);

  const toggleLatestBlocksSort = (field: LatestBlocksSortField) => {
    if (latestBlocksSortField === field) {
      setLatestBlocksSortDirection((current) => (current === 'asc' ? 'desc' : 'asc'));
      return;
    }

    setLatestBlocksSortField(field);
    setLatestBlocksSortDirection(field === 'block_number' ? 'desc' : 'asc');
  };

  const renderLatestBlocksSortIcon = (field: LatestBlocksSortField) => {
    if (latestBlocksSortField !== field) {
      return <ArrowDownUp className="h-3.5 w-3.5 text-[#4e4e4e]" />;
    }

    return latestBlocksSortDirection === 'asc'
      ? <ChevronUp className="h-3.5 w-3.5 text-[#75beff]" />
      : <ChevronDown className="h-3.5 w-3.5 text-[#75beff]" />;
  };

  return (
    <div className="space-y-8">
      <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
        <MetricCard
          title="Block height"
          value={formatNumber(data.height)}
          subtitle="Current highest block available from the explorer backend."
          icon={Layers}
          accent="blue"
        />
        <MetricCard
          title="Sampled transactions"
          value={formatNumber(recentTransactions.length)}
          subtitle="Collected from the newest blocks for UI snapshotting."
          icon={Activity}
          accent="emerald"
        />
        <MetricCard
          title="Latest block tx count"
          value={formatNumber(latestBlock?.transaction_count ?? 0)}
          subtitle="Transaction entries in the newest sampled block."
          icon={Activity}
          accent="violet"
        />
        <MetricCard
          title="Policy rows"
          value={formatNumber(metaPolicies.length + namespacePolicies.length)}
          subtitle="Rows returned for the known `_meta` and `0` namespaces."
          icon={Shield}
          accent="amber"
        />
      </section>

      <section>
        <Card className="overflow-hidden">
          <CardContent className="px-5 py-5">
            <div className="space-y-5">
              <div className="grid gap-4 md:grid-cols-3">
                <div className="rounded-md border border-[#606060] bg-[#3c3c3c] p-4">
                  <div className="mb-3 flex items-center gap-3 text-sm">
                    <div className="rounded-md bg-[#007acc]/15 p-2">
                      <Search className="h-4 w-4 text-[#75beff]" />
                    </div>
                    <div>
                      <p className="font-medium text-[#75beff]">Jump to block</p>
                      <p className="text-xs text-[#858585]">Open a block directly by height.</p>
                    </div>
                  </div>
                  <div className="space-y-2">
                    <SearchInput
                      value={blockQuery}
                      onChange={setBlockQuery}
                      placeholder="e.g. 50"
                    />
                    <Button className="w-full" onClick={handleBlockSearch}>
                      Open block
                    </Button>
                  </div>
                </div>

                <div className="rounded-md border border-[#606060] bg-[#3c3c3c] p-4">
                  <div className="mb-3 flex items-center gap-3 text-sm">
                    <div className="rounded-md bg-[#c586c0]/15 p-2">
                      <FileSearch className="h-4 w-4 text-[#c586c0]" />
                    </div>
                    <div>
                      <p className="font-medium text-[#c586c0]">Find transaction</p>
                      <p className="text-xs text-[#858585]">Inspect a known transaction id.</p>
                    </div>
                  </div>
                  <div className="space-y-2">
                    <SearchInput
                      value={txQuery}
                      onChange={setTxQuery}
                      placeholder="Paste tx id"
                    />
                    <Button className="w-full" onClick={handleTransactionSearch}>
                      Inspect tx
                    </Button>
                  </div>
                </div>

                <div className="rounded-md border border-[#606060] bg-[#3c3c3c] p-4">
                  <div className="mb-3 flex items-center gap-3 text-sm">
                    <div className="rounded-md bg-[#89d185]/15 p-2">
                      <Shield className="h-4 w-4 text-[#89d185]" />
                    </div>
                    <div>
                      <p className="font-medium text-[#89d185]">Explore policies</p>
                      <p className="text-xs text-[#858585]">Open readable namespace rules.</p>
                    </div>
                  </div>
                  <div className="space-y-2">
                    <SearchInput
                      value={namespaceQuery}
                      onChange={setNamespaceQuery}
                      placeholder="_meta or 0"
                    />
                    <Button className="w-full" onClick={handleNamespaceSearch}>
                      Open namespace
                    </Button>
                  </div>
                </div>
              </div>
            </div>
          </CardContent>
        </Card>
      </section>

      <section>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between gap-3">
            <div>
              <CardTitle>Recent block throughput</CardTitle>
              <p className="mt-1 text-sm text-[#b0b0b0]">
                Transaction counts across the latest sampled blocks.
              </p>
            </div>
            <Badge variant="info">{chartData.length} sampled blocks</Badge>
          </CardHeader>
          <CardContent>
            <div className="h-72">
              <ResponsiveContainer width="100%" height="100%">
                <BarChart data={chartData}>
                  <CartesianGrid stroke="#606060" vertical={false} />
                  <XAxis dataKey="block" stroke="#858585" tickLine={false} axisLine={false} fontSize={12} />
                  <YAxis stroke="#858585" tickLine={false} axisLine={false} fontSize={12} />
                  <Tooltip
                    cursor={{ fill: 'rgba(0,122,204,0.08)' }}
                    contentStyle={{
                      background: '#464646',
                      border: '1px solid #606060',
                      borderRadius: '4px',
                      color: '#e8e8e8',
                      fontSize: '13px',
                    }}
                  />
                  <Bar dataKey="transactions" fill="#007acc" radius={[3, 3, 0, 0]} />
                </BarChart>
              </ResponsiveContainer>
            </div>
          </CardContent>
        </Card>
      </section>

      <section>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between gap-3">
            <div>
              <CardTitle>Latest blocks</CardTitle>
              <p className="mt-1 text-sm text-[#b0b0b0]">
                Newest ledger entries sorted in descending block order.
              </p>
            </div>
            <Link href="/blocks" className="text-sm font-medium text-[#75beff] hover:underline">
              View all blocks
            </Link>
          </CardHeader>
          <CardContent>
            <div className="mb-3 flex items-center gap-3">
              <button
                type="button"
                onClick={() => toggleLatestBlocksSort('block_number')}
                className="inline-flex items-center gap-1.5 rounded-md border border-[#606060] bg-[#3c3c3c] px-2.5 py-1.5 text-xs font-medium uppercase tracking-wider text-[#858585] transition hover:text-[#e8e8e8]"
              >
                <span>Block #</span>
                {renderLatestBlocksSortIcon('block_number')}
              </button>
              <button
                type="button"
                onClick={() => toggleLatestBlocksSort('transaction_count')}
                className="inline-flex items-center gap-1.5 rounded-md border border-[#606060] bg-[#3c3c3c] px-2.5 py-1.5 text-xs font-medium uppercase tracking-wider text-[#858585] transition hover:text-[#e8e8e8]"
              >
                <span>Transactions</span>
                {renderLatestBlocksSortIcon('transaction_count')}
              </button>
            </div>

            <div className="space-y-3">
              {sortedLatestBlocks.map((block) => (
                <Link
                  key={block.block_number}
                  href={`/blocks/${block.block_number}`}
                  className="flex flex-col gap-3 rounded-md border border-[#606060] bg-[#3c3c3c] p-3 transition hover:border-[#007acc]/40 hover:bg-[#2a2d2e] lg:flex-row lg:items-center lg:justify-between"
                >
                  <div className="min-w-0 flex-1 space-y-2">
                    <div className="flex items-center gap-3">
                      <span className="text-sm font-semibold text-[#75beff]">#{block.block_number}</span>
                      <Badge variant="info">{formatNumber(block.transaction_count)} {block.transaction_count === 1 ? 'tx' : 'txs'}</Badge>
                    </div>
                    <div className="space-y-1">
                      <p className="text-xs text-[#858585]">
                        <span className="mr-1.5">Data:</span>
                        <HashValue value={block.data_hash} copyable={false} />
                      </p>
                      <p className="text-xs text-[#858585]">
                        <span className="mr-1.5">Prev:</span>
                        <HashValue value={block.previous_hash} copyable={false} />
                      </p>
                    </div>
                  </div>
                  <div className="flex shrink-0 items-center gap-3">
                    <ArrowRight className="h-4 w-4 text-[#4e4e4e]" />
                  </div>
                </Link>
              ))}
            </div>
          </CardContent>
        </Card>
      </section>

    </div>
  );
}
