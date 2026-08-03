import { useState, useEffect, useRef } from 'react';
import { cn } from '../lib/cn';

export const UISelect = ({ value, options, onChange, disabled, up }: { value: string, options: string[], onChange: (v: string) => void, disabled?: boolean, up?: boolean }) => {
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
    <div className="relative w-full app-no-drag" ref={dropdownRef}>
      <div 
        role="combobox"
        aria-expanded={isOpen}
        tabIndex={disabled ? -1 : 0}
        className={cn(
          "w-full px-3.5 py-2.5 text-sm font-medium flex items-center justify-between transition-all duration-200 rounded-xl border",
          "cursor-pointer shadow-sm",
          disabled ? "opacity-50 cursor-not-allowed" : "hover:border-[var(--ui-border-strong)] hover:shadow-md",
          isOpen ? "z-50 relative border-[var(--ui-accent)] ring-2 ring-[var(--ui-accent-glow)]" : ""
        )}
        style={{
          background: 'var(--ui-panel)',
          color: 'var(--ui-text)',
          borderColor: isOpen ? 'var(--ui-accent)' : 'var(--ui-border)',
        }}
        onClick={() => !disabled && setIsOpen(!isOpen)}
        onKeyDown={(e) => {
          if (!disabled && (e.key === 'Enter' || e.key === ' ')) {
            e.preventDefault();
            setIsOpen(!isOpen);
          }
        }}
      >
        <span className="truncate pr-2">{value || 'Выбрать стратегию'}</span>
        <svg 
          className={cn("w-4 h-4 flex-shrink-0 transition-transform duration-200", isOpen && "rotate-180")}
          style={{ color: 'var(--ui-text-muted)' }}
          viewBox="0 0 24 24" 
          fill="none" 
          stroke="currentColor" 
          strokeWidth="2" 
          strokeLinecap="round" 
          strokeLinejoin="round"
        >
          <polyline points="6 9 12 15 18 9" />
        </svg>
      </div>
      
      {isOpen && (
        <ul className={cn(
          "absolute left-0 w-full z-[100] max-h-56 overflow-y-auto py-1.5 rounded-xl border shadow-xl backdrop-blur-md transition-all duration-150",
          up ? "bottom-[calc(100%+6px)] animate-in slide-in-from-bottom-2 fade-in" : "top-[calc(100%+6px)] animate-in slide-in-from-top-2 fade-in"
        )}
        style={{
          background: 'var(--ui-panel-transparent)',
          borderColor: 'var(--ui-border-strong)',
        }}>
          {options.map((opt) => (
            <li 
              key={opt}
              className={cn(
                "px-3.5 py-2 mx-1 rounded-lg cursor-pointer truncate text-sm transition-all duration-150 flex items-center justify-between",
                value === opt ? "font-semibold" : "font-normal"
              )}
              style={{
                color: value === opt ? 'var(--ui-accent)' : 'var(--ui-text)',
                background: value === opt ? 'color-mix(in srgb, var(--ui-accent) 12%, transparent)' : 'transparent',
              }}
              onMouseEnter={e => {
                if (value !== opt) {
                  (e.currentTarget as HTMLElement).style.background = 'color-mix(in srgb, var(--ui-text) 5%, transparent)';
                }
              }}
              onMouseLeave={e => {
                if (value !== opt) {
                  (e.currentTarget as HTMLElement).style.background = 'transparent';
                }
              }}
              onClick={() => {
                onChange(opt);
                setIsOpen(false);
              }}
            >
              <span>{opt}</span>
              {value === opt && (
                <svg className="w-3.5 h-3.5" style={{ color: 'var(--ui-accent)' }} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5">
                  <polyline points="20 6 9 17 4 12" />
                </svg>
              )}
            </li>
          ))}
        </ul>
      )}
    </div>
  );
};


