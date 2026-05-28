'use client';

import { useState, useEffect, useCallback, useMemo } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { ArrowDownUp, ArrowRight, ChevronDown, ChevronLeft, ChevronRight, ChevronUp, Layers, MonitorDot, ServerCog } from 'lucide-react';
import { api, BlockPage } from '@/lib/api';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/Card';
import { SearchInput } from '@/components/ui/SearchInput';
import { LoadingSpinner } from '@/components/ui/Loading';
import { Button } from '@/components/ui/Button';
import { Badge } from '@/components/ui/Badge';
import { MetricCard } from '@/components/explorer/MetricCard';
import { HashValue } from '@/components/explorer/HashValue';
import { formatNumber, formatBytes } from '@/lib/utils';

const BLOCKS_PER_PAGE = 5;

type SortField = 'block_number' | 'transaction_count';
type SortDirection = 'asc' | 'desc';

export default function BlocksPage() {
  const [loading, setLoading] = useState(true);
  const [data, setData] = useState<BlockPage | null>(null);
  const [searchQuery, setSearchQuery] = useState('');
  const [currentPage, setCurrentPage] = useState(0);
  const [error, setError] = useState<string | null>(null);
  const [sortField, setSortField] = useState<SortField>('block_number');
  const [sortDirection, setSortDirection] = useState<SortDirection>('desc');
  const router = useRouter();

  const fetchBlocks = useCallback(async () => {
    setLoading(true);
    try {
      const nextData = await api.getBlockPage(currentPage, BLOCKS_PER_PAGE);
      setData(nextData);
      setError(null);
    } catch (error) {
      console.error('Failed to fetch blocks:', error);
      setError('Unable to load block page from the backend.');
    } finally {
      setLoading(false);
    }
  }, [currentPage]);

  useEffect(() => {
    void fetchBlocks();
  }, [fetchBlocks]);

  const handleSearch = () => {
    if (!searchQuery.trim()) return;
    
    const blockNumber = parseInt(searchQuery);
    if (!isNaN(blockNumber)) {
      router.push(`/blocks/${blockNumber}`);
    }
  };

  const totalPages = data?.total_pages ?? 1;
  const canGoPrevious = currentPage > 0;
  const canGoNext = currentPage < totalPages - 1;

  const sortedBlocks = useMemo(() => {
    if (!data?.blocks) return [];

    return [...data.blocks].sort((left, right) => {
      const delta = left[sortField] - right[sortField];
      return sortDirection === 'asc' ? delta : -delta;
    });
  }, [data?.blocks, sortDirection, sortField]);

  const toggleSort = (field: SortField) => {
    if (sortField === field) {
      setSortDirection((current) => (current === 'asc' ? 'desc' : 'asc'));
      return;
    }

    setSortField(field);
    setSortDirection(field === 'block_number' ? 'desc' : 'asc');
  };

  const renderSortIcon = (field: SortField) => {
    if (sortField !== field) {
      return <ArrowDownUp className="h-3.5 w-3.5 text-[#4e4e4e]" />;
    }

    return sortDirection === 'asc'
      ? <ChevronUp className="h-3.5 w-3.5 text-[#75beff]" />
      : <ChevronDown className="h-3.5 w-3.5 text-[#75beff]" />;
  };

  if (loading && !data) {
    return <LoadingSpinner />;
  }

  return (
    <div className="space-y-8">
      <section className="grid gap-4 md:grid-cols-3">
        <MetricCard title="Total blocks" value={formatNumber(data?.height ?? 0)} subtitle="Current highest block number reported by the explorer backend." icon={Layers} accent="blue" />
        <MetricCard title="Visible window" value={data ? `#${data.highest_block} → #${data.lowest_block}` : '—'} subtitle="Blocks rendered on this page, shown newest first." icon={MonitorDot} accent="violet" />
        <MetricCard title="Backend route" value="/blocks" subtitle="List view uses the backend range endpoint with pagination inputs." icon={ServerCog} accent="emerald" />
      </section>

      <section className="flex flex-col gap-4 xl:flex-row xl:items-center xl:justify-between">
        <div>
          <h1 className="text-2xl font-semibold text-[#4ec9b0]">Ledger blocks</h1>
        </div>
        
        <div className="flex w-full max-w-xl items-center gap-2">
          <SearchInput
            value={searchQuery}
            onChange={setSearchQuery}
            placeholder="Search by block number..."
            className="flex-1"
          />
          <Button onClick={handleSearch}>Search</Button>
        </div>
      </section>

      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <CardTitle>Block List</CardTitle>
            <div className="text-sm text-slate-500">
              Page {currentPage + 1} of {totalPages}
            </div>
          </div>
        </CardHeader>
        <CardContent>
          {loading && data ? (
            <LoadingSpinner />
          ) : error ? (
            <div className="rounded-md border border-[#f48771]/25 bg-[#f48771]/10 px-4 py-3 text-sm text-[#f48771]">
              {error}
            </div>
          ) : (
            <>
              <div className="mb-3 flex items-center gap-3">
                <button
                  type="button"
                  onClick={() => toggleSort('block_number')}
                  className="inline-flex items-center gap-1.5 rounded-md border border-[#606060] bg-[#3c3c3c] px-2.5 py-1.5 text-xs font-medium uppercase tracking-wider text-[#858585] transition hover:text-[#e8e8e8]"
                >
                  <span>Block #</span>
                  {renderSortIcon('block_number')}
                </button>
                <button
                  type="button"
                  onClick={() => toggleSort('transaction_count')}
                  className="inline-flex items-center gap-1.5 rounded-md border border-[#606060] bg-[#3c3c3c] px-2.5 py-1.5 text-xs font-medium uppercase tracking-wider text-[#858585] transition hover:text-[#e8e8e8]"
                >
                  <span>Transactions</span>
                  {renderSortIcon('transaction_count')}
                </button>
              </div>

              <div className="space-y-3">
                {sortedBlocks.map((block) => (
                  <Link
                    key={block.block_number}
                    href={`/blocks/${block.block_number}`}
                    className="flex flex-col gap-3 rounded-md border border-[#606060] bg-[#3c3c3c] p-3 transition hover:border-[#007acc]/40 hover:bg-[#2a2d2e] lg:flex-row lg:items-center lg:justify-between"
                  >
                    <div className="min-w-0 flex-1 space-y-2">
                      <div className="flex items-center gap-3">
                        <span className="text-sm font-semibold text-[#75beff]">#{block.block_number}</span>
                        <Badge variant="info">{formatNumber(block.transaction_count)} {block.transaction_count === 1 ? 'tx' : 'txs'}</Badge>
                        {block.block_size > 0 && (
                          <span className="text-xs text-[#858585]">{formatBytes(block.block_size)}</span>
                        )}
                      </div>
                      {block.created_at && (
                        <p className="text-xs text-[#9cdcfe]">
                          {new Date(block.created_at).toLocaleString(undefined, {
                            year: 'numeric', month: 'short', day: 'numeric',
                            hour: '2-digit', minute: '2-digit', second: '2-digit',
                          })}
                        </p>
                      )}
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
                      <Button variant="outline" size="sm" onClick={(e) => { e.preventDefault(); router.push(`/blocks/${block.block_number}`); }}>
                        View details
                      </Button>
                      <ArrowRight className="h-4 w-4 text-[#4e4e4e]" />
                    </div>
                  </Link>
                ))}
              </div>

              <div className="mt-5 flex items-center justify-between border-t border-[#606060] pt-4">
                <div className="text-sm text-[#858585]">
                  Page {currentPage + 1} of {totalPages}
                </div>
                <div className="flex gap-2">
                  <Button
                    onClick={() => setCurrentPage(p => p - 1)}
                    disabled={!canGoPrevious}
                    variant="outline"
                  >
                    <ChevronLeft className="h-4 w-4 mr-1" />
                    Previous
                  </Button>
                  <Button
                    onClick={() => setCurrentPage(p => p + 1)}
                    disabled={!canGoNext}
                    variant="outline"
                  >
                    Next
                    <ChevronRight className="h-4 w-4 ml-1" />
                  </Button>
                </div>
              </div>
            </>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
