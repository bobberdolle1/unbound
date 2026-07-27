import { SketchyCheck } from './icons';

export const DoodleCheckbox = ({ checked, onChange, id, label, desc }: {
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
    className="flex items-start gap-4 p-3 rounded-xl border-2 cursor-pointer transition-all duration-150 hover:scale-[1.01]"
    style={{
      background: checked
        ? 'color-mix(in srgb, var(--ui-accent) 10%, var(--ui-panel))'
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
      className="w-6 h-6 flex-shrink-0 rounded-md flex items-center justify-center transition-all duration-200 border-2"
      style={{
        background: checked ? 'var(--ui-accent)' : 'transparent',
        borderColor: checked ? 'var(--ui-accent)' : 'var(--ui-border)',
        color: '#ffffff',
        transform: checked ? 'scale(1.1)' : 'scale(1)',
      }}
    >
      {checked && <SketchyCheck className="w-4 h-4" />}
    </div>

    {/* Label + description */}
    <div className="flex flex-col pt-0.5 min-w-0">
      <span className="text-[15px] font-bold leading-snug" style={{ color: 'var(--ui-text)' }}>
        {label}
      </span>
      <span className="text-sm mt-0.5 leading-snug" style={{ color: 'var(--ui-text-muted)' }}>
        {desc}
      </span>
    </div>
  </div>
);
