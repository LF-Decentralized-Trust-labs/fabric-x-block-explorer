import { type LucideIcon } from 'lucide-react';

type Accent = 'blue' | 'emerald' | 'violet' | 'amber';

interface MetricCardProps {
  title: string;
  value: string | number;
  subtitle: string;
  icon: LucideIcon;
  accent?: Accent;
}

const accentStyles: Record<Accent, { bg: string; icon: string; value: string }> = {
  blue:    { bg: 'bg-[#007acc]/15', icon: 'text-[#75beff]', value: 'text-[#75beff]' },
  emerald: { bg: 'bg-[#4ec9b0]/15', icon: 'text-[#4ec9b0]', value: 'text-[#4ec9b0]' },
  violet:  { bg: 'bg-[#c586c0]/15', icon: 'text-[#c586c0]', value: 'text-[#c586c0]' },
  amber:   { bg: 'bg-[#ce9178]/15', icon: 'text-[#ce9178]', value: 'text-[#ce9178]' },
};

export function MetricCard({ title, value, subtitle, icon: Icon, accent = 'blue' }: MetricCardProps) {
  const styles = accentStyles[accent];

  return (
    <div className="rounded-md border border-[#606060] bg-[#464646] p-4">
      <div className="flex items-center gap-3">
        <div className={`rounded-md p-2 ${styles.bg}`}>
          <Icon className={`h-5 w-5 ${styles.icon}`} />
        </div>
        <p className="text-sm font-medium text-[#b0b0b0]">{title}</p>
      </div>
      <p className={`mt-3 text-2xl font-semibold ${styles.value}`}>{value}</p>
      <p className="mt-1 text-xs text-[#858585]">{subtitle}</p>
    </div>
  );
}
