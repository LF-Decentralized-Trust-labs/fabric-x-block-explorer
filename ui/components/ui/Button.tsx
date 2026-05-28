import { ButtonHTMLAttributes, ReactNode } from 'react';
import { cn } from '@/lib/utils';

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  children: ReactNode;
  variant?: 'primary' | 'secondary' | 'outline' | 'ghost' | 'danger';
  size?: 'sm' | 'md' | 'lg' | 'icon';
}

export function Button({
  children,
  variant = 'primary',
  size = 'md',
  className = '',
  type = 'button',
  ...props
}: ButtonProps) {
  const baseClasses = 'inline-flex items-center justify-center gap-1.5 rounded-md font-medium transition-colors duration-150 focus:outline-none focus:ring-2 focus:ring-[#007acc]/40 focus:ring-offset-0 disabled:cursor-not-allowed disabled:opacity-50';
  
  const variantClasses = {
    primary: 'bg-[#007acc] text-white hover:bg-[#006bb3]',
    secondary: 'border border-[#606060] bg-[#3c3c3c] text-[#e8e8e8] hover:bg-[#454545]',
    outline: 'border border-[#606060] bg-[#464646] text-[#e8e8e8] hover:bg-[#37373d]',
    ghost: 'bg-transparent text-[#b0b0b0] hover:bg-[#2a2d2e] hover:text-[#e8e8e8]',
    danger: 'bg-[#f48771] text-[#303030] hover:bg-[#d6705d]',
  };
  
  const sizeClasses = {
    sm: 'px-3 py-1.5 text-xs',
    md: 'px-4 py-2 text-sm',
    lg: 'px-5 py-2.5 text-sm',
    icon: 'h-8 w-8 text-sm',
  };
  
  return (
    <button
      type={type}
      className={cn(baseClasses, variantClasses[variant], sizeClasses[size], className)}
      {...props}
    >
      {children}
    </button>
  );
}
