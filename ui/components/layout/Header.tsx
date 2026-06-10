'use client';

import { useState, useEffect } from 'react';
import { Activity, Layers3 } from 'lucide-react';
import { usePathname } from 'next/navigation';
import { api } from '@/lib/api';
import { cn } from '@/lib/utils';

const titles: Record<string, { title: string; description: string }> = {
  '/': {
    title: 'Explorer Dashboard',
    description: 'Live network summary, search, and recent blockchain activity.',
  },
  '/blocks': {
    title: 'Blocks',
    description: 'Paginated blockchain ledger view with hash and transaction summaries.',
  },
  '/transactions': {
    title: 'Transactions',
    description: 'Direct transaction lookup and sampled activity from the newest blocks.',
  },
  '/policies': {
    title: 'Namespace Policies',
    description: 'Readable policy rules, MSP identities, endpoints, and certificates.',
  },
};

export function Header() {
  const [status, setStatus] = useState<'online' | 'offline' | 'checking'>('checking');
  const pathname = usePathname();

  const current = Object.entries(titles).find(([route]) =>
    route === '/' ? pathname === route : pathname.startsWith(route)
  )?.[1] ?? titles['/'];

  useEffect(() => {
    const checkHealth = async () => {
      try {
        await api.healthCheck();
        setStatus('online');
      } catch (error) {
        setStatus('offline');
      }
    };

    checkHealth();
    const interval = setInterval(checkHealth, 30000); // Check every 30 seconds

    return () => clearInterval(interval);
  }, []);

  return (
    <header className="sticky top-0 z-20 border-b border-[#606060] bg-[#3c3c3c] px-6 py-3">
      <div className="flex flex-col gap-3 xl:flex-row xl:items-center xl:justify-between">
        <div>
          <div className="flex items-center gap-3">
            <div className="inline-flex h-8 w-8 shrink-0 items-center justify-center rounded-md bg-[#007acc] text-white">
              <Layers3 className="h-4 w-4" />
            </div>
            <div>
              <h2 className="text-xl font-semibold text-[#9cdcfe]">{current.title}</h2>
              <p className="text-sm text-[#b0b0b0]">{current.description}</p>
            </div>
          </div>
        </div>

        <div className="grid gap-3 sm:grid-cols-1">
          <div className="rounded-md border border-[#606060] bg-[#464646] px-3 py-2">
            <div className="flex items-center gap-2 text-xs font-medium uppercase tracking-wider text-[#858585]">
              <Activity className="h-3.5 w-3.5" />
              Status
            </div>
            <div
              className={cn(
                'mt-1 flex items-center gap-2 text-sm font-semibold',
                status === 'online' && 'text-[#89d185]',
                status === 'offline' && 'text-[#f48771]',
                status === 'checking' && 'text-[#cca700]'
              )}
            >
              <span
                className={cn(
                  'h-2 w-2 rounded-full',
                  status === 'online' && 'bg-[#89d185]',
                  status === 'offline' && 'bg-[#f48771]',
                  status === 'checking' && 'bg-[#cca700]'
                )}
              />
              {status === 'online' ? 'Connected' : status === 'offline' ? 'Unavailable' : 'Checking'}
            </div>
          </div>
        </div>
      </div>
    </header>
  );
}
