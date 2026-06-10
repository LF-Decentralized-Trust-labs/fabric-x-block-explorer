import { ReactNode } from 'react';
import { cn } from '@/lib/utils';

interface BadgeProps {
  children: ReactNode;
  variant?: 'success' | 'error' | 'warning' | 'info' | 'default';
  className?: string;
}

export function Badge({ children, variant = 'default', className = '' }: BadgeProps) {
  const variants = {
    success: 'border border-[#89d185]/25 bg-[#89d185]/10 text-[#89d185]',
    error: 'border border-[#f48771]/25 bg-[#f48771]/10 text-[#f48771]',
    warning: 'border border-[#cca700]/25 bg-[#cca700]/10 text-[#cca700]',
    info: 'border border-[#007acc]/25 bg-[#007acc]/15 text-[#75beff]',
    default: 'border border-[#606060] bg-[#3c3c3c] text-[#b0b0b0]',
  };
  
  return (
    <span className={cn('inline-flex items-center rounded-full px-2.5 py-1 text-xs font-medium', variants[variant], className)}>
      {children}
    </span>
  );
}
