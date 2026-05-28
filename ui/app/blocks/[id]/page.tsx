'use client';

import { useState, useEffect, useCallback, useMemo } from 'react';
import { useParams, useRouter } from 'next/navigation';
import { api, Block } from '@/lib/api';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/Card';
import { Button } from '@/components/ui/Button';
import { Badge } from '@/components/ui/Badge';
import { LoadingSpinner } from '@/components/ui/Loading';
import { Activity, ArrowDownUp, ArrowLeft, ArrowRight, BookOpen, ChevronDown, ChevronUp, Layers } from 'lucide-react';
import Link from 'next/link';
import { MetricCard } from '@/components/explorer/MetricCard';
import { HashValue } from '@/components/explorer/HashValue';
import { formatNumber, getValidationCodeText, getValidationTone, pluralize } from '@/lib/utils';

type TxSortDirection = 'asc' | 'desc';

export default function BlockDetailPage() {
  const params = useParams();
  const router = useRouter();
  const blockNumber = parseInt(params.id as string);
  const txPageSize = 25;
  
  const [block, setBlock] = useState<Block | null>(null);
  const [blockHeight, setBlockHeight] = useState(0);
  const [txPage, setTxPage] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [txSortDirection, setTxSortDirection] = useState<TxSortDirection>('asc');

  useEffect(() => {
    if (!isNaN(blockNumber)) {
      setTxPage(0);
    }
  }, [blockNumber]);

  const fetchBlock = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [height, data] = await Promise.all([
        api.getBlockHeight(),
        api.getBlock(blockNumber),
      ]);
      setBlockHeight(height.height);
      setBlock(data);
    } catch (err) {
      setError('Block not found');
      console.error(err);
    } finally {
      setLoading(false);
    }
  }, [blockNumber, txPage]);

  useEffect(() => {
    if (!isNaN(blockNumber)) {
      void fetchBlock();
    }
  }, [blockNumber, txPage, fetchBlock]);

  const transactions = block?.transactions ?? [];
  const sortedTransactions = useMemo(() => {
    return [...transactions].sort((left, right) => {
      return txSortDirection === 'asc'
        ? left.tx_index - right.tx_index
        : right.tx_index - left.tx_index;
    });
  }, [transactions, txSortDirection]);

  const txSortIcon = txSortDirection === 'asc'
    ? <ChevronUp className="h-3.5 w-3.5 text-[#75beff]" />
    : <ChevronDown className="h-3.5 w-3.5 text-[#75beff]" />;

  if (loading) {
    return <LoadingSpinner />;
  }

  if (error || !block) {
    return (
      <div className="space-y-6">
        <Button onClick={() => router.back()} variant="outline">
          <ArrowLeft className="h-4 w-4 mr-2" />
          Back
        </Button>
        <Card>
          <CardContent className="text-center py-12">
            <p className="text-red-600 font-medium">{error || 'Block not found'}</p>
          </CardContent>
        </Card>
      </div>
    );
  }

  const totalTxPages = Math.max(1, Math.ceil(block.transaction_count / txPageSize));

  return (
    <div className="space-y-8">
      <div className="flex items-center justify-between">
        <Button onClick={() => router.back()} variant="outline">
          <ArrowLeft className="h-4 w-4 mr-2" />
          Back to Blocks
        </Button>
      </div>

      <div>
        <h1 className="text-2xl font-semibold text-[#4ec9b0]">Block #{block.block_number}</h1>
        <p className="mt-1 text-sm text-[#b0b0b0]">Detailed block metadata with paginated transaction inspection.</p>
      </div>

      <section className="grid gap-4 md:grid-cols-3">
        <MetricCard title="Block number" value={`#${block.block_number}`} subtitle="Selected block identifier." icon={Layers} accent="blue" />
        <MetricCard title="Transactions" value={formatNumber(block.transaction_count)} subtitle="Total transactions recorded in this block." icon={Activity} accent="emerald" />
        <MetricCard title="Tx page" value={`${txPage + 1} / ${totalTxPages}`} subtitle="Server-side transaction pagination using tx_offset and tx_limit." icon={BookOpen} accent="violet" />
      </section>

      <Card>
        <CardHeader>
          <CardTitle>Block hashes</CardTitle>
        </CardHeader>
        <CardContent className="grid gap-4 md:grid-cols-2">
          <div>
            <p className="mb-2 text-sm font-medium text-[#858585]">Data hash</p>
            <HashValue value={block.data_hash} fullWidth copyable={false} />
          </div>
          <div>
            <p className="mb-2 text-sm font-medium text-[#858585]">Previous hash</p>
            <HashValue value={block.previous_hash} fullWidth copyable={false} />
            <p className="mt-2 text-xs text-[#858585]">Latest known block in explorer: #{blockHeight}</p>
          </div>
        </CardContent>
      </Card>

      {block.transactions.length > 0 && (
        <Card>
          <CardHeader className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <div>
              <CardTitle>Transactions ({block.transactions.length})</CardTitle>
              <p className="mt-1 text-sm text-[#b0b0b0]">
                Showing transaction window {txPage * txPageSize + 1}–{Math.min((txPage + 1) * txPageSize, block.transaction_count)} of {formatNumber(block.transaction_count)}.
              </p>
            </div>
            <div className="flex gap-2">
              <Button variant="outline" size="sm" disabled={txPage === 0} onClick={() => setTxPage((page) => page - 1)}>
                <ArrowLeft className="h-4 w-4" />
                Prev tx page
              </Button>
              <Button variant="outline" size="sm" disabled={txPage >= totalTxPages - 1} onClick={() => setTxPage((page) => page + 1)}>
                Next tx page
                <ArrowRight className="h-4 w-4" />
              </Button>
            </div>
          </CardHeader>
          <CardContent>
            <div className="mb-3 flex items-center gap-3">
              <button
                type="button"
                onClick={() => setTxSortDirection((current) => current === 'asc' ? 'desc' : 'asc')}
                className="inline-flex items-center gap-1.5 rounded-md border border-[#606060] bg-[#3c3c3c] px-2.5 py-1.5 text-xs font-medium uppercase tracking-wider text-[#858585] transition hover:text-[#e8e8e8]"
              >
                <span>TX Index</span>
                {block.transactions.length > 0 ? txSortIcon : <ArrowDownUp className="h-3.5 w-3.5 text-slate-400" />}
              </button>
            </div>

            <div className="space-y-3">
              {sortedTransactions.map((tx) => (
                <Link
                  key={tx.tx_id}
                  href={`/transactions/${tx.tx_id}`}
                  className="flex flex-col gap-3 rounded-md border border-[#606060] bg-[#3c3c3c] p-3 transition hover:border-[#007acc]/40 hover:bg-[#2a2d2e] lg:flex-row lg:items-center lg:justify-between"
                >
                  <div className="min-w-0 flex-1 space-y-2">
                    <div className="flex items-center gap-3">
                      <span className="text-sm font-semibold text-[#75beff]">#{tx.tx_index}</span>
                      <Badge variant={getValidationTone(tx.validation_code)}>
                        {getValidationCodeText(tx.validation_code)}
                      </Badge>
                    </div>
                    <HashValue value={tx.tx_id} />
                    <p className="text-xs text-[#858585]">
                      {tx.read_writes.length + tx.blind_writes.length} {pluralize(tx.read_writes.length + tx.blind_writes.length, 'write')} • {tx.endorsements.length} endorsements
                    </p>
                  </div>
                  <div className="flex shrink-0 items-center gap-3">
                    <ArrowRight className="h-4 w-4 text-[#4e4e4e]" />
                  </div>
                </Link>
              ))}
            </div>
          </CardContent>
        </Card>
      )}

      <div className="flex justify-between">
        <Button
          onClick={() => router.push(`/blocks/${blockNumber - 1}`)}
          variant="outline"
          disabled={blockNumber === 0}
        >
          <ArrowLeft className="h-4 w-4 mr-2" />
          Previous Block
        </Button>
        <Button
          onClick={() => router.push(`/blocks/${blockNumber + 1}`)}
          variant="outline"
          disabled={blockNumber >= blockHeight}
        >
          Next Block
          <ArrowRight className="h-4 w-4 ml-2" />
        </Button>
      </div>
    </div>
  );
}
