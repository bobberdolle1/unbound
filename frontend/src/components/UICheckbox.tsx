import { UICheck } from './icons';
import { cn } from '../lib/cn';

export const UICheckbox = ({ checked, onChange, id, label, desc }: {
  checked: boolean;
  onChange: () => void;
  id?: string;
  label: string;
  desc: string;
}) => (
  <div
    id={id}
    role="checkbox"
    aria-checked={checked}
    tabIndex={0}
    className={cn(
      "flex items-start gap-3.5 p-3 rounded-xl border cursor-pointer transition-all duration-150 app-no-drag group",
      "hover:border-[var(--ui-border-strong)]"
    )}
    style={{
      background: checked
        ? 'color-mix(in srgb, var(--ui-accent) 8%, var(--ui-panel))'
        : 'var(--ui-panel)',
      borderColor: checked ? 'var(--ui-accent)' : 'var(--ui-border)',
      color: 'var(--ui-text)',
    }}
    onClick={onChange}
    onKeyDown={(e) => {
      if (e.key === ' ' || e.key === 'Enter') { e.preventDefault(); onChange(); }
    }}
  >
    {/* Checkbox indicator */}
    <div
      className="w-5 h-5 mt-0.5 flex-shrink-0 rounded-md flex items-center justify-center transition-all duration-200 border"
      style={{
        background: checked ? 'var(--ui-accent)' : 'transparent',
        borderColor: checked ? 'var(--ui-accent)' : 'var(--ui-border-strong)',
        color: '#ffffff',
        transform: checked ? 'scale(1.05)' : 'scale(1)',
      }}
    >
      {checked && <UICheck className="w-3.5 h-3.5" />}
    </div>

    {/* Label + description */}
    <div className="flex flex-col min-w-0">
      <span className="text-sm font-semibold leading-snug" style={{ color: 'var(--ui-text)' }}>
        {label}
      </span>
      <span className="text-xs mt-0.5 leading-snug" style={{ color: 'var(--ui-text-muted)' }}>
        {desc}
      </span>
    </div>
  </div>
);


