import { cn } from '../lib/cn';
import { SketchyCheck } from './icons';

export const DoodleCheckbox = ({ checked, onChange, id, label, desc }: { checked: boolean, onChange: () => void, id?: string, label: string, desc: string }) => (
  <div id={id} role="checkbox" aria-checked={checked} tabIndex={0} className="flex items-start gap-4 p-3 sketch-box cursor-pointer hover:bg-white hover:shadow-[2px_2px_0_rgba(0,0,0,0.6)] transition-all duration-150 hover:scale-[1.01]" onClick={onChange} onKeyDown={(e) => { if (e.key === ' ' || e.key === 'Enter') { e.preventDefault(); onChange(); } }}>
    <div className={cn(
      "w-7 h-7 flex-shrink-0 sketch-input flex items-center justify-center transition-all duration-200 bg-white",
      checked ? "text-green-600 scale-110" : "text-transparent scale-100"
    )}>
      {checked && <SketchyCheck className="w-5 h-5 animate-in zoom-in duration-200" />}
    </div>
    <div className="flex flex-col pt-0.5">
      <span className="text-[17px] font-bold text-gray-900 leading-none">{label}</span>
      <span className="text-sm text-gray-600 mt-1 leading-snug">{desc}</span>
    </div>
  </div>
);
