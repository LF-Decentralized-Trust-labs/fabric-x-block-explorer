import { type LucideIcon } from 'lucide-react';
import { type ReactNode } from 'react';

interface EmptyStateProps {
  icon: LucideIcon;
  title: string;
  description: string | ReactNode;
}

export function EmptyState({ icon: Icon, title, description }: EmptyStateProps) {
  return (
    <div className="flex flex-col items-center justify-center gap-3 py-10 text-center">
      <div className="rounded-full bg-[#3c3c3c] p-4">
        <Icon className="h-8 w-8 text-[#858585]" />
      </div>
      <p className="text-sm font-medium text-[#e8e8e8]">{title}</p>
      <p className="max-w-sm text-xs text-[#858585]">{description}</p>
    </div>
  );
}
