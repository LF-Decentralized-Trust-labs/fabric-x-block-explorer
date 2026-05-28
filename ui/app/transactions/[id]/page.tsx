'use client';

import { useState, useEffect, useCallback } from 'react';
import { useParams, useRouter } from 'next/navigation';
import { api, Transaction } from '@/lib/api';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/Card';
import { Button } from '@/components/ui/Button';
import { Badge } from '@/components/ui/Badge';
import { LoadingSpinner } from '@/components/ui/Loading';
import { copyToClipboard, getValidationCodeText, getValidationTone, pluralize } from '@/lib/utils';
import { ArrowLeft, Check, Copy, FileText, Shield, Database, KeyRound } from 'lucide-react';
import Link from 'next/link';
import { MetricCard } from '@/components/explorer/MetricCard';
import { EmptyState } from '@/components/explorer/EmptyState';
import { HexField } from '@/components/explorer/HexField';

export default function TransactionDetailPage() {
  const params = useParams();
  const router = useRouter();
  const txId = params.id as string;
  
  const [transaction, setTransaction] = useState<Transaction | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [copiedField, setCopiedField] = useState<string | null>(null);

  const fetchTransaction = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await api.getTransaction(txId);
      setTransaction(data);
    } catch (err) {
      setError('Transaction not found');
      console.error(err);
    } finally {
      setLoading(false);
    }
  }, [txId]);

  useEffect(() => {
    if (txId) {
      void fetchTransaction();
    }
  }, [txId, fetchTransaction]);

  const handleCopy = async (text: string, field: string) => {
    try {
      await copyToClipboard(text);
      setCopiedField(field);
      setTimeout(() => setCopiedField(null), 2000);
    } catch (err) {
      console.error('Failed to copy:', err);
    }
  };

  if (loading) {
    return <LoadingSpinner />;
  }

  if (error || !transaction) {
    return (
      <div className="space-y-6">
        <Button onClick={() => router.back()} variant="outline">
          <ArrowLeft className="h-4 w-4 mr-2" />
          Back
        </Button>
        <Card>
          <CardContent className="text-center py-12">
            <p className="text-red-600 font-medium">{error || 'Transaction not found'}</p>
          </CardContent>
        </Card>
      </div>
    );
  }

  const CopyButton = ({ text, field }: { text: string; field: string }) => (
    <button
      onClick={() => handleCopy(text, field)}
      className="ml-2 p-1 text-[#858585] hover:text-[#e8e8e8] transition-colors"
      title="Copy to clipboard"
    >
      {copiedField === field ? (
        <Check className="h-4 w-4 text-[#89d185]" />
      ) : (
        <Copy className="h-4 w-4" />
      )}
    </button>
  );

  const validationText = getValidationCodeText(transaction.validation_code);

  return (
    <div className="space-y-8">
      <div className="flex items-center justify-between">
        <Button onClick={() => router.back()} variant="outline">
          <ArrowLeft className="h-4 w-4 mr-2" />
          Back
        </Button>
      </div>

      <div>
        <h1 className="text-2xl font-semibold text-[#c586c0]">Transaction details</h1>
        <p className="mt-1 break-all font-mono text-sm text-[#b0b0b0]">{transaction.tx_id}</p>
      </div>

      <section className="grid gap-4 md:grid-cols-4">
        <MetricCard title="Validation" value={validationText} subtitle={`Code ${transaction.validation_code}`} icon={Shield} accent="amber" />
        <MetricCard title="Read/write rows" value={transaction.read_writes.length} subtitle="State mutations with key/value payloads." icon={Database} accent="emerald" />
        <MetricCard title="Blind writes" value={transaction.blind_writes.length} subtitle="Writes returned without read-version context." icon={FileText} accent="amber" />
        <MetricCard title="Read-only rows" value={transaction.reads_only.length} subtitle="Ledger reads captured for this transaction." icon={KeyRound} accent="blue" />
      </section>

      <Card>
        <CardHeader>
          <CardTitle>Transaction Information</CardTitle>
        </CardHeader>
        <CardContent>
          <dl className="space-y-4">
            <div className="grid grid-cols-1 sm:grid-cols-3 gap-2">
              <dt className="text-sm font-medium text-[#858585]">Transaction ID</dt>
              <dd className="sm:col-span-2 text-sm text-[#e8e8e8] font-mono break-all flex items-start">
                {transaction.tx_id}
                <CopyButton text={transaction.tx_id} field="tx_id" />
              </dd>
            </div>

            <div className="grid grid-cols-1 sm:grid-cols-3 gap-2">
              <dt className="text-sm font-medium text-[#858585]">Block Number</dt>
              <dd className="sm:col-span-2 text-sm">
                <Link
                  href={`/blocks/${transaction.block_number}`}
                  className="font-medium text-[#75beff] hover:underline"
                >
                  #{transaction.block_number}
                </Link>
              </dd>
            </div>

            <div className="grid grid-cols-1 sm:grid-cols-3 gap-2">
              <dt className="text-sm font-medium text-[#858585]">Transaction Index</dt>
              <dd className="sm:col-span-2 text-sm text-[#e8e8e8]">
                {transaction.tx_index}
              </dd>
            </div>

            <div className="grid grid-cols-1 sm:grid-cols-3 gap-2">
              <dt className="text-sm font-medium text-[#858585]">Validation Code</dt>
              <dd className="sm:col-span-2">
                <Badge variant={getValidationTone(transaction.validation_code)}>
                  {validationText}
                </Badge>
              </dd>
            </div>

            {transaction.chaincode_name && (
              <div className="grid grid-cols-1 sm:grid-cols-3 gap-2">
                <dt className="text-sm font-medium text-[#858585]">Chaincode</dt>
                <dd className="sm:col-span-2 text-sm text-[#e8e8e8] font-mono">{transaction.chaincode_name}</dd>
              </div>
            )}

            {transaction.channel_id && (
              <div className="grid grid-cols-1 sm:grid-cols-3 gap-2">
                <dt className="text-sm font-medium text-[#858585]">Channel</dt>
                <dd className="sm:col-span-2 text-sm text-[#e8e8e8]">{transaction.channel_id}</dd>
              </div>
            )}

            {transaction.created_at && (
              <div className="grid grid-cols-1 sm:grid-cols-3 gap-2">
                <dt className="text-sm font-medium text-[#858585]">Timestamp</dt>
                <dd className="sm:col-span-2 text-sm text-[#e8e8e8]">
                  {new Date(transaction.created_at).toLocaleString(undefined, {
                    year: 'numeric', month: 'short', day: 'numeric',
                    hour: '2-digit', minute: '2-digit', second: '2-digit',
                  })}
                  <span className="ml-2 text-[#858585] text-xs font-mono">({transaction.created_at})</span>
                </dd>
              </div>
            )}

            {transaction.creator_msp_id && (
              <div className="grid grid-cols-1 sm:grid-cols-3 gap-2">
                <dt className="text-sm font-medium text-[#858585]">Creator MSP</dt>
                <dd className="sm:col-span-2 text-sm text-[#e8e8e8] font-mono">{transaction.creator_msp_id}</dd>
              </div>
            )}

            {transaction.tx_type && (
              <div className="grid grid-cols-1 sm:grid-cols-3 gap-2">
                <dt className="text-sm font-medium text-[#858585]">Tx Type</dt>
                <dd className="sm:col-span-2 text-sm text-[#e8e8e8] font-mono">{transaction.tx_type}</dd>
              </div>
            )}
          </dl>
        </CardContent>
      </Card>

      <section className="grid gap-6 xl:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>Read/write rows ({transaction.read_writes.length})</CardTitle>
          </CardHeader>
          <CardContent>
            {transaction.read_writes.length > 0 ? (
              <div className="space-y-4">
                {transaction.read_writes.map((row, index) => (
                  <div key={`${row.key}-${index}`} className="rounded-md border border-[#606060] bg-[#3c3c3c] p-3">
                    <div className="grid gap-2 text-sm">
                      <div className="flex flex-col gap-0.5">
                        <span className="text-[#858585] text-xs font-medium">Namespace</span>
                        <span className="text-[#e8e8e8]">{row.namespace}</span>
                      </div>
                      <HexField label="Key" hex={row.key} />
                      <HexField label="Value" hex={row.value} showDeleted />
                    </div>
                  </div>
                ))}
              </div>
            ) : (
              <EmptyState icon={Database} title="No read/write rows" description="This transaction did not return any read-write records." />
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Blind writes ({transaction.blind_writes.length})</CardTitle>
          </CardHeader>
          <CardContent>
            {transaction.blind_writes.length > 0 ? (
              <div className="space-y-4">
                {transaction.blind_writes.map((row, index) => (
                  <div key={`${row.key}-${index}`} className="rounded-md border border-[#606060] bg-[#3c3c3c] p-3 text-sm">
                    <div className="grid gap-2">
                      <div className="flex flex-col gap-0.5">
                        <span className="text-[#858585] text-xs font-medium">Namespace</span>
                        <span className="text-[#e8e8e8]">{row.namespace}</span>
                      </div>
                      <HexField label="Key" hex={row.key} />
                      <HexField label="Value" hex={row.value} showDeleted />
                    </div>
                  </div>
                ))}
              </div>
            ) : (
              <EmptyState icon={FileText} title="No blind writes" description="The explorer did not return any blind write rows for this transaction." />
            )}
          </CardContent>
        </Card>
      </section>

      <section className="grid gap-6 xl:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>Read-only rows ({transaction.reads_only.length})</CardTitle>
          </CardHeader>
          <CardContent>
            {transaction.reads_only.length > 0 ? (
              <div className="space-y-4">
                {transaction.reads_only.map((row, index) => (
                  <div key={`${row.key}-${index}`} className="rounded-md border border-[#606060] bg-[#3c3c3c] p-3 text-sm">
                    <div className="grid gap-2">
                      <div className="flex flex-col gap-0.5">
                        <span className="text-[#858585] text-xs font-medium">Namespace</span>
                        <span className="text-[#e8e8e8]">{row.namespace}</span>
                      </div>
                      <HexField label="Key" hex={row.key} />
                    </div>
                  </div>
                ))}
              </div>
            ) : (
              <EmptyState icon={KeyRound} title="No read-only rows" description="No read-only ledger reads were captured for this transaction." />
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Endorsements ({transaction.endorsements.length})</CardTitle>
          </CardHeader>
          <CardContent>
            {transaction.endorsements.length > 0 ? (
              <div className="space-y-4">
                {transaction.endorsements.map((endorsement, index) => (
                  <div key={`${endorsement.endorsement}-${index}`} className="rounded-md border border-[#606060] bg-[#3c3c3c] p-3 text-sm">
                    <p><span className="text-[#858585]">Namespace:</span> <span className="text-[#e8e8e8]">{endorsement.namespace}</span></p>
                    <p className="mt-1.5"><span className="text-[#858585]">MSP ID:</span> <span className="text-[#e8e8e8]">{endorsement.msp_id || '—'}</span></p>
                    <p className="mt-1.5 break-all"><span className="text-[#858585]">Endorsement:</span> <span className="font-mono text-[#e8e8e8]">{endorsement.endorsement || '∅'}</span></p>
                    <p className="mt-1.5 break-all"><span className="text-[#858585]">Certificate ID:</span> <span className="font-mono text-[#e8e8e8]">{endorsement.certificate_id || '∅'}</span></p>
                  </div>
                ))}
              </div>
            ) : (
              <EmptyState icon={Shield} title="No endorsements" description="The explorer returned no endorsement rows for this transaction." />
            )}
          </CardContent>
        </Card>
      </section>

    </div>
  );
}
