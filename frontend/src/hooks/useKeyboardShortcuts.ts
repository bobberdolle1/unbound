import { useEffect } from 'react';

export function useKeyboardShortcuts(onEscape: () => void, onToggleLogs?: () => void) {
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        onEscape();
        return;
      }
      if (
        onToggleLogs &&
        (e.ctrlKey || e.metaKey) &&
        !e.altKey &&
        (e.key.toLowerCase() === 'j' || e.key === '`' || e.code === 'Backquote')
      ) {
        e.preventDefault();
        onToggleLogs();
      }
    };
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [onEscape, onToggleLogs]);
}
