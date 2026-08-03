import React from 'react';
import { cn } from '../../lib/cn';

export type TabType = 'main' | 'profiles' | 'lists' | 'settings';

interface AppNavigationProps {
  activeTab: TabType;
  onTabChange: (tab: TabType) => void;
}

export const AppNavigation: React.FC<AppNavigationProps> = ({ activeTab, onTabChange }) => {
  const tabs: { id: TabType; label: string }[] = [
    { id: 'main', label: 'Главная' },
    { id: 'profiles', label: 'Профили & LUA' },
    { id: 'lists', label: 'Списки обхода' },
    { id: 'settings', label: 'Настройки' },
  ];

  return (
    <div className="flex bg-[var(--ui-surface-elevated)] border border-[var(--ui-border)] rounded-[var(--ui-radius)] p-1 gap-1 flex-wrap shrink-0">
      {tabs.map((tab) => {
        const isActive = activeTab === tab.id;
        return (
          <button
            key={tab.id}
            onClick={() => onTabChange(tab.id)}
            className={cn(
              'flex-1 min-w-0 text-center text-xs font-semibold py-1.5 px-2 rounded-md transition-all truncate',
              isActive
                ? 'bg-[var(--ui-panel)] text-[var(--ui-text)] shadow-sm'
                : 'text-[var(--ui-text-muted)] hover:text-[var(--ui-text)]'
            )}
          >
            {tab.label}
          </button>
        );
      })}
    </div>
  );
};
