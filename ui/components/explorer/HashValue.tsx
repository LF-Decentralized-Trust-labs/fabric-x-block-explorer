'use client';

import { useState } from 'react';
import { Check, Copy } from 'lucide-react';
import { copyToClipboard, truncateMiddle } from '@/lib/utils';

interface HashValueProps {
  value: string | null | undefined;
  /** Show full hash without truncation */
  fullWidth?: boolean;
  /** Show a copy-to-clipboard button (default true) */
  copyable?: boolean;
  /** Additional class names applied to the outer span */
  className?: string;
}

export function HashValue({ value, fullWidth = false, copyable = true, className }: HashValueProps) {
  const [copied, setCopied] = useState(false);

  if (!value) {
    return <span className={`font-mono text-xs text-[#858585] ${className ?? ''}`}>—</span>;
  }

  const display = fullWidth ? value : truncateMiddle(value, 12, 8);

  const handleCopy = async () => {
    try {
      await copyToClipboard(value);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      // ignore
    }
  };

  return (
    <span className={`inline-flex items-center gap-1.5 ${className ?? ''}`}>
      <span className="break-all font-mono text-xs text-[#9cdcfe]">{display}</span>
      {copyable && (
        <button
          type="button"
          onClick={handleCopy}
          className="shrink-0 text-[#858585] transition-colors hover:text-[#e8e8e8]"
          title="Copy to clipboard"
        >
          {copied ? (
            <Check className="h-3.5 w-3.5 text-[#89d185]" />
          ) : (
            <Copy className="h-3.5 w-3.5" />
          )}
        </button>
      )}
    </span>
  );
}
