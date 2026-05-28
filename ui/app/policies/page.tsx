'use client';

import { useEffect, useState, useCallback } from 'react';
import { ChevronDown, ChevronUp, FileStack, Search, ServerCog, Shield } from 'lucide-react';
import { api, NamespacePolicy } from '@/lib/api';
import { decodePolicyBytes } from '@/lib/policyDecoder';
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
            const decoded = decodePolicyBytes(policy.policy_bytes);

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
                    {decoded?.consensusType ? (
                      <div>
                        <span className="text-[#858585]">Consensus: </span>
                        <span className="text-[#e8e8e8]">{decoded.consensusType}</span>
                      </div>
                    ) : null}
                    {decoded?.hashAlgorithm ? (
                      <div>
                        <span className="text-[#858585]">Hash: </span>
                        <span className="text-[#e8e8e8]">{decoded.hashAlgorithm}</span>
                      </div>
                    ) : null}
                    {decoded?.consortium ? (
                      <div>
                        <span className="text-[#858585]">Consortium: </span>
                        <span className="text-[#e8e8e8]">{decoded.consortium}</span>
                      </div>
                    ) : null}
                    {decoded ? (
                      <div>
                        <span className="text-[#858585]">Certificates: </span>
                        <span className="text-[#e8e8e8]">{decoded.certificates}</span>
                      </div>
                    ) : null}
                  </div>
                  <Button onClick={() => togglePolicyExpansion(expansionKey)} variant="outline" size="sm">
                    {isExpanded ? <><ChevronUp className="h-4 w-4" /> Hide</> : <><ChevronDown className="h-4 w-4" /> Details</>}
                  </Button>
                </div>

                {/* Always-visible summary chips */}
                {decoded ? (
                  <div className="space-y-3">
                    {decoded.organizations.length > 0 ? (
                      <div>
                        <p className="mb-1.5 text-xs font-semibold uppercase tracking-wide text-[#858585]">Organizations</p>
                        <div className="flex flex-wrap gap-1.5">
                          {decoded.organizations.map((org) => (
                            <Badge key={org} variant="success">{org}</Badge>
                          ))}
                        </div>
                      </div>
                    ) : null}

                    {decoded.capabilities.length > 0 ? (
                      <div>
                        <p className="mb-1.5 text-xs font-semibold uppercase tracking-wide text-[#858585]">Capabilities</p>
                        <div className="flex flex-wrap gap-1.5">
                          {decoded.capabilities.map((cap) => (
                            <Badge key={cap} variant="info">{cap}</Badge>
                          ))}
                        </div>
                      </div>
                    ) : null}

                    {decoded.policyRoles.length > 0 ? (
                      <div>
                        <p className="mb-1.5 text-xs font-semibold uppercase tracking-wide text-[#858585]">Policy Roles</p>
                        <div className="flex flex-wrap gap-1.5">
                          {decoded.policyRoles.map((role) => (
                            <Badge key={role} variant="warning">{role}</Badge>
                          ))}
                        </div>
                      </div>
                    ) : null}
                  </div>
                ) : null}

                {/* Expanded details */}
                {isExpanded && decoded ? (
                  <div className="mt-4 space-y-4 border-t border-[#606060] pt-4">
                    {decoded.ordererNodes.length > 0 ? (
                      <div>
                        <p className="mb-1.5 text-xs font-semibold uppercase tracking-wide text-[#858585]">Orderer Nodes</p>
                        <div className="flex flex-wrap gap-1.5">
                          {decoded.ordererNodes.map((node) => (
                            <Badge key={node} variant="default">{node}</Badge>
                          ))}
                        </div>
                      </div>
                    ) : null}

                    {decoded.endpoints.length > 0 ? (
                      <div>
                        <p className="mb-1.5 text-xs font-semibold uppercase tracking-wide text-[#858585]">Endpoints</p>
                        <div className="flex flex-wrap gap-1.5">
                          {decoded.endpoints.map((ep) => (
                            <Badge key={ep} variant="default">{ep}</Badge>
                          ))}
                        </div>
                      </div>
                    ) : null}

                    {decoded.aclRules.length > 0 ? (
                      <div>
                        <p className="mb-1.5 text-xs font-semibold uppercase tracking-wide text-[#858585]">ACL Rules ({decoded.aclRules.length})</p>
                        <div className="max-h-64 overflow-y-auto rounded-md border border-[#606060] bg-[#3c3c3c]">
                          <table className="w-full text-xs">
                            <thead className="sticky top-0 bg-[#303030]">
                              <tr>
                                <th className="px-3 py-2 text-left text-[#858585] font-medium">Resource</th>
                                <th className="px-3 py-2 text-left text-[#858585] font-medium">Policy</th>
                              </tr>
                            </thead>
                            <tbody>
                              {decoded.aclRules.map((rule, i) => (
                                <tr key={i} className="border-t border-[#606060]">
                                  <td className="px-3 py-1.5 text-[#9cdcfe] font-mono">{rule.resource}</td>
                                  <td className="px-3 py-1.5 text-[#e8e8e8]">{rule.policy}</td>
                                </tr>
                              ))}
                            </tbody>
                          </table>
                        </div>
                      </div>
                    ) : null}

                    <div>
                      <p className="mb-1.5 text-xs font-semibold uppercase tracking-wide text-[#858585]">Raw Policy Bytes (base64)</p>
                      <pre className="max-h-32 overflow-y-auto whitespace-pre-wrap break-all rounded-md border border-[#606060] bg-[#3c3c3c] p-3 text-[10px] font-mono leading-4 text-[#858585]">
                        {policy.policy_bytes}
                      </pre>
                    </div>
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
