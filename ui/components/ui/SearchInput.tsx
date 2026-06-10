import { InputHTMLAttributes, KeyboardEvent } from 'react';
import { Search } from 'lucide-react';
import { cn } from '@/lib/utils';

interface SearchInputProps extends Omit<InputHTMLAttributes<HTMLInputElement>, 'onChange'> {
  value: string;
  onChange: (value: string) => void;
  className?: string;
  onKeyPress?: (e: KeyboardEvent<HTMLInputElement>) => void;
}

export function SearchInput({
  value,
  onChange,
  placeholder = 'Search...',
  className = '',
  onKeyPress,
  ...props
}: SearchInputProps) {
  return (
    <div className={cn('relative', className)}>
      <div className="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3">
        <Search className="h-4 w-4 text-[#858585]" />
      </div>
      <input
        type="text"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        onKeyPress={onKeyPress}
        className="block w-full rounded-md border border-[#606060] bg-[#3c3c3c] py-2 pl-9 pr-3 text-sm text-[#e8e8e8] placeholder:text-[#858585] focus:border-[#007acc] focus:outline-none focus:ring-1 focus:ring-[#007acc]"
        placeholder={placeholder}
        {...props}
      />
    </div>
  );
}
