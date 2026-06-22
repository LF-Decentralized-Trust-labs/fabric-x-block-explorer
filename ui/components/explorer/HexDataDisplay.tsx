'use client';

import { useState } from 'react';
import { Button } from '@/components/ui/Button';
import { Eye, EyeOff } from 'lucide-react';

interface HexDataDisplayProps {
  data: string;
  label?: string;
  className?: string;
}

/**
 * Attempts to decode hex string to UTF-8 text.
 * Returns null if the data is not valid UTF-8.
 */
function hexToText(hex: string): string | null {
  try {
    // Remove any whitespace
    const cleanHex = hex.replace(/\s/g, '');
    
    // Convert hex to bytes
    const bytes = new Uint8Array(cleanHex.match(/.{1,2}/g)?.map(byte => parseInt(byte, 16)) || []);
    
    // Try to decode as UTF-8
    const decoder = new TextDecoder('utf-8', { fatal: true });
    const text = decoder.decode(bytes);
    
    // Check if the text contains mostly printable characters
    const printableRatio = text.split('').filter(c => {
      const code = c.charCodeAt(0);
      return (code >= 32 && code <= 126) || code === 10 || code === 13 || code === 9;
    }).length / text.length;
    
    // If less than 80% printable, consider it binary
    if (printableRatio < 0.8) {
      return null;
    }
    
    return text;
  } catch {
    return null;
  }
}

export function HexDataDisplay({ data, label, className = '' }: HexDataDisplayProps) {
  const [showText, setShowText] = useState(false);
  
  if (!data || data === '—') {
    return <span className="text-sm text-[#858585]">—</span>;
  }
  
  const textVersion = hexToText(data);
  const canShowText = textVersion !== null;
  
  return (
    <div className={`space-y-2 ${className}`}>
      <div className="flex items-center gap-2">
        <span className="text-xs text-[#858585]">
          {Math.floor(data.length / 2)} bytes
        </span>
        {canShowText && (
          <Button
            variant="ghost"
            size="sm"
            onClick={() => setShowText(!showText)}
            className="h-6 px-2 text-xs"
          >
            {showText ? (
              <>
                <EyeOff className="h-3 w-3 mr-1" />
                Show Hex
              </>
            ) : (
              <>
                <Eye className="h-3 w-3 mr-1" />
                Show Text
              </>
            )}
          </Button>
        )}
        {!canShowText && (
          <span className="text-xs text-[#4ec9b0]">(binary data)</span>
        )}
      </div>
      
      <div className="font-mono text-xs break-all">
        {showText && textVersion ? (
          <div className="bg-[#2d2d2d] border border-[#454545] rounded p-2 text-[#d4d4d4] whitespace-pre-wrap">
            {textVersion}
          </div>
        ) : (
          <div className="text-[#ce9178]">
            {data.length > 80 ? `${data.slice(0, 40)}…${data.slice(-8)}` : data}
          </div>
        )}
      </div>
    </div>
  );
}

// Made with Bob
