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
          "w-full sketch-input px-4 py-3 text-gray-900 font-bold text-base flex items-center justify-between transition-all duration-200 bg-white/80",
          disabled ? "opacity-60 cursor-not-allowed" : "cursor-pointer hover:bg-white hover:shadow-[3px_3px_0_rgba(0,0,0,0.8)] hover:scale-[1.01]",
          isOpen && "bg-white z-50 relative shadow-[3px_3px_0_rgba(0,0,0,0.8)]"
        )}
        onClick={() => !disabled && setIsOpen(!isOpen)}
      >
        <span className="truncate">{value || 'Выбрать стратегию'}</span>
        <span className={cn("font-marker font-black text-xl transition-transform duration-200", isOpen && "rotate-180")}>{isOpen ? 'x' : 'v'}</span>
      </div>
      
      {isOpen && (
        <ul className={cn(
          "absolute left-0 w-full z-[100] sketch-box max-h-48 overflow-y-auto py-2 shadow-[4px_4px_0_rgba(0,0,0,0.8)] animate-in slide-in-from-top-2 fade-in duration-200",
          up ? "bottom-[calc(100%+8px)]" : "top-[calc(100%+8px)]"
        )}>
          {options.map((opt) => (
            <li 
              key={opt}
              className={cn(
                "px-4 py-2 hover:bg-yellow-100 hover:text-yellow-900 cursor-pointer truncate font-bold text-base transition-all duration-150 hover:translate-x-1",
                value === opt ? "bg-yellow-200 text-yellow-900" : "text-gray-800"
              )}
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
