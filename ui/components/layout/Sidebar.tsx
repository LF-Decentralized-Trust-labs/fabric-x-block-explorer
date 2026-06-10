'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { Home, Layers, FileText, Shield, Menu, X, ChevronRight, Cpu } from 'lucide-react';
import { useState } from 'react';
import { cn } from '@/lib/utils';
import { API_DISPLAY_URL } from '@/lib/api';

const navigation = [
  { name: 'Dashboard', href: '/', icon: Home },
  { name: 'Blocks', href: '/blocks', icon: Layers },
  { name: 'Transactions', href: '/transactions', icon: FileText },
  { name: 'Policies', href: '/policies', icon: Shield },
];

export function Sidebar() {
  const pathname = usePathname();
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false);

  return (
    <>
      <div className="fixed left-4 top-4 z-50 lg:hidden">
        <button
          onClick={() => setMobileMenuOpen(!mobileMenuOpen)}
          className="rounded-lg border border-[#606060] bg-[#464646] p-2 text-[#e8e8e8] shadow-sm"
        >
          {mobileMenuOpen ? <X className="h-6 w-6" /> : <Menu className="h-6 w-6" />}
        </button>
      </div>

      <aside
        className={cn(
          'fixed left-0 top-0 z-40 flex h-screen w-80 flex-col border-r border-[#606060] bg-[#3c3c3c] px-4 py-4 transition-transform',
          mobileMenuOpen ? 'translate-x-0' : '-translate-x-full lg:translate-x-0'
        )}
      >
        <div className="flex h-full flex-col">
          <div className="rounded-lg border border-[#606060] bg-[#464646] p-4">
            <div className="flex items-center gap-3">
              <div className="rounded-lg bg-[#007acc] p-2.5">
                <Cpu className="h-6 w-6 text-white" />
              </div>
              <div>
                <p className="text-xs font-semibold uppercase tracking-[0.2em] text-[#007acc]">
                  Fabric-X
                </p>
                <h1 className="text-lg font-semibold bg-gradient-to-r from-[#4ec9b0] to-[#007acc] bg-clip-text text-transparent">Block Explorer</h1>
              </div>
            </div>
          </div>

          <nav className="mt-5 flex-1 space-y-1 overflow-y-auto">
            {navigation.map((item) => {
              const isActive = item.href === '/'
                ? pathname === item.href
                : pathname.startsWith(item.href);
              const Icon = item.icon;
              
              return (
                <Link
                  key={item.name}
                  href={item.href}
                  onClick={() => setMobileMenuOpen(false)}
                  className={cn(
                    'group flex items-center justify-between rounded-md px-3 py-2.5 text-sm font-medium transition-colors',
                    isActive
                      ? 'bg-[#37373d] text-[#007acc] border-l-2 border-[#007acc]'
                      : 'text-[#b0b0b0] hover:bg-[#2a2d2e] hover:text-[#e8e8e8]'
                  )}
                >
                  <div className="flex items-center gap-3">
                    <Icon className={cn('h-[18px] w-[18px]', isActive ? 'text-[#007acc]' : 'text-[#858585] group-hover:text-[#b0b0b0]')} />
                    <span>{item.name}</span>
                  </div>
                  <ChevronRight className={cn('h-4 w-4', isActive ? 'text-[#007acc]' : 'text-[#4e4e4e] group-hover:text-[#858585]')} />
                </Link>
              );
            })}
          </nav>

          <div className="rounded-lg border border-[#606060] bg-[#464646] p-4">
            <p className="text-xs font-medium uppercase tracking-[0.18em] text-[#858585]">
              Backend
            </p>
            <div className="mt-2 flex items-start gap-2 text-sm font-medium text-[#e8e8e8]">
              <span className="mt-1.5 h-2 w-2 shrink-0 rounded-full bg-[#007acc]" />
              <p className="break-all leading-5">{API_DISPLAY_URL}</p>
            </div>
          </div>
        </div>
      </aside>

      {mobileMenuOpen && (
        <div
          className="fixed inset-0 z-30 bg-black/40 lg:hidden"
          onClick={() => setMobileMenuOpen(false)}
        />
      )}
    </>
  );
}
