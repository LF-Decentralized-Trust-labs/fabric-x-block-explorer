'use client';

import { useEffect, useState, useCallback } from 'react';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/Card';
import { SearchInput } from '@/components/ui/SearchInput';
import { Button } from '@/components/ui/Button';
import { Badge } from '@/components/ui/Badge';
import { LoadingSpinner } from '@/components/ui/Loading';
import { useRouter } from 'next/navigation';
import { ArrowRight, ServerCog } from 'lucide-react';
import Link from 'next/link';
import { api, Transaction } from '@/lib/api';
import { MetricCard } from '@/components/explorer/MetricCard';
import { HashValue } from '@/components/explorer/HashValue';
import { getValidationCodeText, getValidationTone, pluralize } from '@/lib/utils';

export default function TransactionsPage() {
  const [searchQuery, setSearchQuery] = useState('');
  const [transactions, setTransactions] = useState<Transaction[]>([]);
  const [loading, setLoading] = useState(true);
  const router = useRouter();

  const loadTransactions = useCallback(async () => {
    try {
      setTransactions(await api.getRecentTransactions(5));
    } catch (error) {
      console.error(error);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void loadTransactions();
  }, [loadTransactions]);

  const handleSearch = () => {
    if (searchQuery.trim()) {
      router.push(`/transactions/${searchQuery.trim()}`);
    }
  };

  const handleKeyPress = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter') {
      handleSearch();
    }
  };

  if (loading) {
    return <LoadingSpinner />;
  }

  return (
    <div className="space-y-8">
      <section className="grid gap-4">
        <MetricCard
          title="Backend route"
          value="/transactions/{tx_id}"
          subtitle="Direct record lookup uses the backend transaction endpoint."
          icon={ServerCog}
          accent="emerald"
        />
      </section>

      <Card>
        <CardHeader>
          <div className="flex flex-col gap-1">
            <CardTitle>Lookup</CardTitle>
            <p className="text-sm text-[#b0b0b0]">
              Open any record directly by id.
            </p>
          </div>
        </CardHeader>
        <CardContent>
          <div className="rounded-md border border-[#606060] bg-[#3c3c3c] p-4">
            <div className="flex flex-col gap-3 lg:flex-row lg:items-end lg:justify-between">
              <div className="min-w-0 flex-1">
                <div className="flex flex-col gap-2 sm:flex-row">
                  <SearchInput
                    value={searchQuery}
                    onChange={setSearchQuery}
                    placeholder="Paste id"
                    className="flex-1"
                    onKeyPress={handleKeyPress}
                  />
                  <Button onClick={handleSearch} disabled={!searchQuery.trim()} className="sm:min-w-[120px]">
                    Open
                  </Button>
                </div>
              </div>
            </div>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <div className="flex items-center justify-between gap-3">
            <CardTitle>Latest blocks</CardTitle>
          </div>
        </CardHeader>
        <CardContent>
          <div className="space-y-3">
            {transactions.map((tx) => (
              <Link
                key={tx.tx_id}
                href={`/transactions/${tx.tx_id}`}
                className="flex flex-col gap-3 rounded-md border border-[#606060] bg-[#3c3c3c] p-3 transition hover:border-[#007acc]/40 hover:bg-[#2a2d2e] lg:flex-row lg:items-center lg:justify-between"
              >
                <div className="min-w-0 flex-1 space-y-2">
                  <HashValue value={tx.tx_id} fullWidth className="w-full" />
                  <p className="text-sm text-[#b0b0b0]">
                    Block #{tx.block_number} • {tx.read_writes.length + tx.blind_writes.length} {pluralize(tx.read_writes.length + tx.blind_writes.length, 'write')} • {tx.endorsements.length} endorsements
                  </p>
                </div>
                <div className="flex shrink-0 items-center gap-3">
                  <Badge variant={getValidationTone(tx.validation_code)}>
                    {getValidationCodeText(tx.validation_code)}
                  </Badge>
                  <ArrowRight className="h-4 w-4 text-[#4e4e4e]" />
                </div>
              </Link>
            ))}
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
