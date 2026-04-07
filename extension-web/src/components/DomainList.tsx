import { useState } from 'react';
import { isValidDomain } from '../utils/proxy';

interface DomainListProps {
  domains: string[];
  onDomainsChange: (domains: string[]) => void;
}

export function DomainList({ domains, onDomainsChange }: DomainListProps) {
  const [inputValue, setInputValue] = useState('');
  const [error, setError] = useState<string | null>(null);

  const addDomain = () => {
    const domain = inputValue.trim().toLowerCase();
    
    if (!domain) {
      setError('Please enter a domain');
      return;
    }

    if (!isValidDomain(domain)) {
      setError('Invalid domain format');
      return;
    }

    if (domains.includes(domain)) {
      setError('Domain already exists');
      return;
    }

    onDomainsChange([...domains, domain]);
    setInputValue('');
    setError(null);
  };

  const removeDomain = (index: number) => {
    onDomainsChange(domains.filter((_, i) => i !== index));
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter') {
      e.preventDefault();
      addDomain();
    }
  };

  return (
    <div className="space-y-2">
      <label className="text-xs font-medium text-[var(--text-muted)] uppercase tracking-wide">
        Bypass Domains
      </label>
      
      {/* Input */}
      <div className="flex gap-2">
        <input
          type="text"
          value={inputValue}
          onChange={(e) => {
            setInputValue(e.target.value);
            setError(null);
          }}
          onKeyDown={handleKeyDown}
          placeholder="e.g., *.youtube.com"
          className="flex-1 px-3 py-2 text-sm rounded-lg border border-[var(--border-color)] 
                     bg-[var(--bg-surface)] text-[var(--text-primary)] 
                     placeholder-[var(--text-muted)]
                     focus:outline-none focus:ring-2 focus:ring-[var(--color-accent)] focus:border-transparent"
        />
        <button
          onClick={addDomain}
          className="px-3 py-2 rounded-lg bg-[var(--color-primary)] text-white 
                     hover:bg-[var(--color-primary-hover)] transition-colors
                     focus:outline-none focus:ring-2 focus:ring-[var(--color-accent)]"
          aria-label="Add domain"
        >
          <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
          </svg>
        </button>
      </div>

      {/* Error message */}
      {error && (
        <p className="text-xs text-red-500">{error}</p>
      )}

      {/* Domain list */}
      <div className="max-h-32 overflow-y-auto space-y-1">
        {domains.length === 0 ? (
          <p className="text-xs text-[var(--text-muted)] italic py-2 text-center">
            No domains added
          </p>
        ) : (
          domains.map((domain, index) => (
            <div
              key={domain}
              className="flex items-center justify-between px-3 py-2 rounded-lg 
                         bg-[var(--bg-surface)] border border-[var(--border-color)]
                         group hover:border-[var(--color-accent)] transition-colors"
            >
              <span className="text-sm text-[var(--text-primary)] font-mono">
                {domain}
              </span>
              <button
                onClick={() => removeDomain(index)}
                className="opacity-0 group-hover:opacity-100 text-[var(--text-muted)] 
                           hover:text-red-500 transition-all"
                aria-label={`Remove ${domain}`}
              >
                <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                </svg>
              </button>
            </div>
          ))
        )}
      </div>
    </div>
  );
}
