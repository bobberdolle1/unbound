import { cn } from '../lib/cn';

/**
 * Hand-drawn icon set. Inline SVG rather than an icon package: the app ships
 * six glyphs, and pulling in a library for them cost more bundle than the
 * whole set.
 */
export type IconProps = { className?: string };

export const SketchySpinner = ({ className }: IconProps) => (
  <svg className={cn(className, "animate-spin")} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
    <path d="M21 12a9 9 0 1 1-6.219-8.56" />
  </svg>
);

export const SketchyX = ({ className }: IconProps) => (
  <svg className={className} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="3" strokeLinecap="round" strokeLinejoin="round">
    <path d="M18 6L6 18M6 6l12 12" />
  </svg>
);

export const SketchyStar = ({ className }: IconProps) => (
  <svg className={className} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <path d="M13 2L15.09 8.26L22 9.27L17 14.14L18.18 21.02L12 17.77L5.82 21.02L7 14.14L2 9.27L8.91 8.26L11 2Z" transform="translate(0.5, 0.5) rotate(2)"/>
    <path d="M13 2L15.09 8.26L22 9.27L17 14.14L18.18 21.02L12 17.77L5.82 21.02L7 14.14L2 9.27L8.91 8.26L11 2Z" transform="translate(-0.5, -0.5) rotate(-2)" opacity="0.4"/>
  </svg>
);

export const SketchyGear = ({ className }: IconProps) => (
  <svg className={className} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
    <circle cx="12" cy="12" r="3.5" />
    <path d="M19.5 15.5c.2.6.4 1.2.8 1.8l-1.5 2.5-3-1.5c-.6.2-1.2.4-1.8.6v3.5h-4v-3.5c-.6-.2-1.2-.4-1.8-.6l-3 1.5-1.5-2.5c.4-.6.6-1.2.8-1.8H2.5v-4h3.5c-.2-.6-.4-1.2-.6-1.8l-1.5-2.5 2.5-1.5c.6.4 1.2.6 1.8.8V2.5h4v3.5c.6-.2 1.2-.4 1.8-.6l2.5-1.5 1.5 2.5c-.2.6-.4 1.2-.6 1.8h3.5v4h-3.5z" />
  </svg>
);

export const SketchyTerminal = ({ className }: IconProps) => (
  <svg className={className} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
    <path d="M4.5 17.5l6-6-6-6" />
    <path d="M12.5 18.5h7" />
  </svg>
);

export const SketchyCheck = ({ className }: IconProps) => (
  <svg className={className} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="3" strokeLinecap="round" strokeLinejoin="round">
    <path d="M20 6.5l-11 11-5-5" />
  </svg>
);
