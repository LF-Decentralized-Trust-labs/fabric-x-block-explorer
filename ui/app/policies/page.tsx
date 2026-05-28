'use client';

import { useEffect, useState, useCallback } from 'react';
import { ChevronDown, ChevronUp, FileStack, Search, ServerCog, Shield } from 'lucide-react';
import { api, NamespacePolicy } from '@/lib/api';
import { Badge } from '@/components/ui/Badge';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/Card';
import { SearchInput } from '@/components/ui/SearchInput';
import { Button } from '@/components/ui/Button';
import { LoadingSpinner } from '@/components/ui/Loading';
import { MetricCard } from '@/components/explorer/MetricCard';
import { EmptyState } from '@/components/explorer/EmptyState';

export default function PoliciesPage() {
  const [searchQuery, setSearchQuery] = useState('_meta');
  const [policies, setPolicies] = useState<NamespacePolicy[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [activeNamespace, setActiveNamespace] = useState('_meta');
  const [expandedPolicies, setExpandedPolicies] = useState<Set<string>>(new Set());

  const handleSearch = useCallback(async (namespaceArg?: string) => {
    const namespace = (namespaceArg ?? searchQuery).trim();
    if (!namespace) {
      return;
    }

    setLoading(true);
    setError(null);
    setActiveNamespace(namespace);

    try {
      const nextPolicies = await api.getPolicies(namespace);
      setPolicies(nextPolicies);
      if (nextPolicies.length === 0) {
        setError('No policies found for this namespace.');
      }
    } catch (err) {
      setPolicies([]);
      setError('Failed to fetch policies from the explorer backend.');
      console.error(err);
    } finally {
      setLoading(false);
    }
  }, [searchQuery]);

  useEffect(() => {
    const namespace = new URLSearchParams(window.location.search).get('namespace') || '_meta';
    setSearchQuery(namespace);
    void handleSearch(namespace);
  }, [handleSearch]);

  const togglePolicyExpansion = (key: string) => {
    setExpandedPolicies((prev) => {
      const next = new Set(prev);
      if (next.has(key)) {
        next.delete(key);
      } else {
        next.add(key);
      }
      return next;
    });
  };

  return (
    <div className="space-y-8">
      <section className="grid gap-4 md:grid-cols-3">
        <MetricCard
          title="Active namespace"
          value={activeNamespace}
          subtitle="Namespace currently queried against the live policies endpoint."
          icon={Shield}
          accent="amber"
        />
        <MetricCard
          title="Returned rows"
          value={policies.length}
          subtitle="Policy rows returned for the selected namespace."
          icon={FileStack}
          accent="blue"
        />
        <MetricCard
          title="Backend route"
          value="/namespaces/{namespace}/policies"
          subtitle="The UI renders the backend’s native readable policy payload."
          icon={ServerCog}
          accent="emerald"
        />
      </section>

      <Card>
        <CardHeader>
          <CardTitle>Namespace policy lookup</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="space-y-4">
            <div className="flex gap-2">
              <SearchInput
                value={searchQuery}
                onChange={setSearchQuery}
                placeholder="Enter namespace (e.g. _meta or 0)..."
                className="flex-1"
                onKeyPress={(e) => {
                  if (e.key === 'Enter') {
                    void handleSearch();
                  }
                }}
              />
              <Button onClick={() => void handleSearch()} disabled={!searchQuery.trim() || loading}>
                {loading ? 'Searching...' : 'Search'}
              </Button>
            </div>

            <div className="flex flex-wrap gap-2">
              {['_meta', '0'].map((namespace) => (
                <Button
                  key={namespace}
                  variant={searchQuery === namespace ? 'primary' : 'outline'}
                  size="sm"
                  onClick={() => {
                    setSearchQuery(namespace);
                    void handleSearch(namespace);
                  }}
                >
                  {namespace}
                </Button>
              ))}
            </div>
          </div>
        </CardContent>
      </Card>

      {loading ? <LoadingSpinner /> : null}

      {!loading && error ? (
        <Card>
          <CardContent className="py-12">
            <EmptyState icon={Shield} title="No policy rows found" description={error} />
          </CardContent>
        </Card>
      ) : null}

      {!loading && !error && policies.length === 0 ? (
        <Card>
          <CardContent className="py-12">
            <EmptyState
              icon={Search}
              title="Search a namespace"
              description="Try `_meta` for channel policy metadata or `0` for ledger namespace data if available."
            />
          </CardContent>
        </Card>
      ) : null}

      {!loading && policies.length > 0 ? (
        <div className="space-y-4">
          {policies.map((policy) => {
            const expansionKey = `${policy.namespace}-${policy.version}`;
            const isExpanded = expandedPolicies.has(expansionKey);

            return (
              <div key={expansionKey} className="rounded-md border border-[#606060] bg-[#464646] p-4">
                {/* Header row */}
                <div className="mb-4 flex items-start justify-between gap-4">
                  <div className="grid grid-cols-2 gap-x-8 gap-y-2 text-sm">
                    <div>
                      <span className="text-[#858585]">Namespace: </span>
                      <span className="font-semibold text-[#e8e8e8]">{policy.namespace}</span>
                    </div>
                    <div>
                      <span className="text-[#858585]">Version: </span>
                      <span className="text-[#e8e8e8]">v{policy.version}</span>
                    </div>
                    {policy.hash_algorithm ? (
                      <div>
                        <span className="text-[#858585]">Hash: </span>
                        <span className="text-[#e8e8e8]">{policy.hash_algorithm}</span>
                      </div>
                    ) : null}
                    {policy.certificates.length > 0 ? (
                      <div>
                        <span className="text-[#858585]">Certificates: </span>
                        <span className="text-[#e8e8e8]">{policy.certificates.length}</span>
                      </div>
                    ) : null}
                  </div>
                  <Button onClick={() => togglePolicyExpansion(expansionKey)} variant="outline" size="sm">
                    {isExpanded ? <><ChevronUp className="h-4 w-4" /> Hide</> : <><ChevronDown className="h-4 w-4" /> Details</>}
                  </Button>
                </div>

                {/* Always-visible summary chips */}
                <div className="space-y-3">
                  {policy.msp_ids.length > 0 ? (
                    <div>
                      <p className="mb-1.5 text-xs font-semibold uppercase tracking-wide text-[#858585]">MSP IDs</p>
                      <div className="flex flex-wrap gap-1.5">
                        {policy.msp_ids.map((id) => (
                          <Badge key={id} variant="success">{id}</Badge>
                        ))}
                      </div>
                    </div>
                  ) : null}

                  {policy.endpoints.length > 0 ? (
                    <div>
                      <p className="mb-1.5 text-xs font-semibold uppercase tracking-wide text-[#858585]">Endpoints</p>
                      <div className="flex flex-wrap gap-1.5">
                        {policy.endpoints.map((ep) => (
                          <Badge key={ep} variant="default">{ep}</Badge>
                        ))}
                      </div>
                    </div>
                  ) : null}
                </div>

                {/* Expanded details */}
                {isExpanded ? (
                  <div className="mt-4 space-y-4 border-t border-[#606060] pt-4">
                    <div>
                      <p className="mb-1.5 text-xs font-semibold uppercase tracking-wide text-[#858585]">Policy</p>
                      <pre className="max-h-48 overflow-y-auto whitespace-pre-wrap rounded-md border border-[#606060] bg-[#3c3c3c] p-3 text-[10px] font-mono leading-4 text-[#858585]">
                        {policy.policy}
                      </pre>
                    </div>

                    {policy.certificates.length > 0 ? (
                      <div>
                        <p className="mb-1.5 text-xs font-semibold uppercase tracking-wide text-[#858585]">Certificates ({policy.certificates.length})</p>
                        <div className="max-h-48 overflow-y-auto space-y-2">
                          {policy.certificates.map((cert, i) => (
                            <pre key={i} className="whitespace-pre-wrap break-all rounded-md border border-[#606060] bg-[#3c3c3c] p-3 text-[10px] font-mono leading-4 text-[#858585]">
                              {cert}
                            </pre>
                          ))}
                        </div>
                      </div>
                    ) : null}
                  </div>
                ) : null}
              </div>
            );
          })}
        </div>
      ) : null}
    </div>
  );
}
