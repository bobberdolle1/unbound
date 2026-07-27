import { useState, useEffect, useRef } from 'react';
import { cn } from '../lib/cn';

export const DoodleSelect = ({ value, options, onChange, disabled, up }: { value: string, options: string[], onChange: (v: string) => void, disabled?: boolean, up?: boolean }) => {
  const [isOpen, setIsOpen] = useState(false);
  const dropdownRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (dropdownRef.current && !dropdownRef.current.contains(event.target as Node)) {
        setIsOpen(false);
      }
    };
    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, []);

  return (
    <div className="relative w-full" ref={dropdownRef}>
      <div 
        className={cn(
          "w-full px-4 py-3 font-bold text-base flex items-center justify-between transition-all duration-200 rounded-xl border-2",
          "cursor-pointer",
          disabled ? "opacity-60 cursor-not-allowed" : "hover:scale-[1.01]",
          isOpen ? "z-50 relative" : ""
        )}
        style={{
          background: 'var(--ui-panel)',
          color: 'var(--ui-text)',
          borderColor: 'var(--ui-border)',
        }}
        onClick={() => !disabled && setIsOpen(!isOpen)}
      >
        <span className="truncate">{value || 'Выбрать стратегию'}</span>
        <span className={cn("font-bold text-lg transition-transform duration-200", isOpen && "rotate-180")} style={{ color: 'var(--ui-text-muted)' }}>
          {isOpen ? '▲' : '▼'}
        </span>
      </div>
      
      {isOpen && (
        <ul className={cn(
          "absolute left-0 w-full z-[100] max-h-52 overflow-y-auto py-1 rounded-xl border-2 shadow-xl animate-in slide-in-from-top-2 fade-in duration-200",
          up ? "bottom-[calc(100%+8px)]" : "top-[calc(100%+8px)]"
        )}
        style={{
          background: 'var(--ui-panel)',
          borderColor: 'var(--ui-border)',
        }}>
          {options.map((opt) => (
            <li 
              key={opt}
              className={cn(
                "px-4 py-2 cursor-pointer truncate font-semibold text-base transition-all duration-150",
                value === opt ? "font-bold" : ""
              )}
              style={{
                color: value === opt ? 'var(--ui-accent)' : 'var(--ui-text)',
                background: value === opt ? 'color-mix(in srgb, var(--ui-accent) 12%, transparent)' : 'transparent',
              }}
              onMouseEnter={e => {
                (e.currentTarget as HTMLElement).style.background = 'color-mix(in srgb, var(--ui-accent) 12%, transparent)';
                (e.currentTarget as HTMLElement).style.color = 'var(--ui-accent)';
              }}
              onMouseLeave={e => {
                (e.currentTarget as HTMLElement).style.background = value === opt ? 'color-mix(in srgb, var(--ui-accent) 12%, transparent)' : 'transparent';
                (e.currentTarget as HTMLElement).style.color = value === opt ? 'var(--ui-accent)' : 'var(--ui-text)';
              }}
              onClick={() => {
                onChange(opt);
                setIsOpen(false);
              }}
            >
              {opt}
            </li>
          ))}
        </ul>
      )}
    </div>
  );
};
