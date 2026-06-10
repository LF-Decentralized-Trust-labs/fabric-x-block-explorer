'use client';

import { useState } from 'react';
import { ChevronDown, ChevronRight } from 'lucide-react';
import { decodeHexBytes } from '@/lib/utils';

interface HexFieldProps {
  label: string;
  hex: string | null | undefined;
  /** When true, renders a "deleted / null" badge for null/empty values */
  showDeleted?: boolean;
}

export function HexField({ label, hex, showDeleted = false }: HexFieldProps) {
  const [expanded, setExpanded] = useState(false);

  if (!hex) {
    return (
      <div className="flex items-start gap-2 text-xs">
        <span className="w-14 shrink-0 font-medium text-[#858585]">{label}</span>
        {showDeleted ? (
          <span className="rounded border border-[#f48771]/30 bg-[#f48771]/10 px-1.5 py-0.5 font-mono text-[#f48771]">
            deleted / null
          </span>
        ) : (
          <span className="text-[#858585]">—</span>
        )}
      </div>
    );
  }

  const { text, isReadable, isJson, jsonValue } = decodeHexBytes(hex);

  if (isJson) {
    return (
      <div className="text-xs">
        <div className="flex items-start gap-2">
          <span className="w-14 shrink-0 font-medium text-[#858585]">{label}</span>
          <div className="flex-1 overflow-hidden">
            <button
              type="button"
              onClick={() => setExpanded((v) => !v)}
              className="inline-flex items-center gap-1 font-medium text-[#89d185] hover:underline"
            >
              {expanded ? <ChevronDown className="h-3.5 w-3.5" /> : <ChevronRight className="h-3.5 w-3.5" />}
              JSON value
            </button>
            {expanded && (
              <pre className="mt-1 overflow-x-auto rounded bg-[#2d2d2d] p-2 font-mono text-[#ce9178]">
                {JSON.stringify(jsonValue, null, 2)}
              </pre>
            )}
          </div>
        </div>
      </div>
    );
  }

  if (isReadable) {
    return (
      <div className="flex items-start gap-2 text-xs">
        <span className="w-14 shrink-0 font-medium text-[#858585]">{label}</span>
        <span className="break-all font-mono text-[#89d185]">{text}</span>
      </div>
    );
  }

  // Raw hex — truncate long values with expand
  const SHORT = 48;
  const isLong = hex.length > SHORT;
  return (
    <div className="text-xs">
      <div className="flex items-start gap-2">
        <span className="w-14 shrink-0 font-medium text-[#858585]">{label}</span>
        <div className="flex-1 overflow-hidden">
          <span className="break-all font-mono text-[#ce9178]">
            {expanded || !isLong ? hex : `${hex.slice(0, SHORT)}…`}
          </span>
          {isLong && (
            <button
              type="button"
              onClick={() => setExpanded((v) => !v)}
              className="ml-1.5 text-[#858585] hover:text-[#e8e8e8]"
            >
              {expanded ? '(collapse)' : '(expand)'}
            </button>
          )}
        </div>
      </div>
    </div>
  );
}
