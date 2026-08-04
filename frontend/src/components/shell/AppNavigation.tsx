import React from 'react';
import { cn } from '../../lib/cn';

export type TabType = 'main' | 'profiles' | 'lists' | 'settings';

interface AppNavigationProps {
  activeTab: TabType;
  onTabChange: (tab: TabType) => void;
}

export const AppNavigation: React.FC<AppNavigationProps> = ({ activeTab, onTabChange }) => {
  const tabs: { id: TabType; label: string; compactLabel: string }[] = [
    { id: 'main', label: 'Главная', compactLabel: 'Главная' },
    { id: 'profiles', label: 'Профили & LUA', compactLabel: 'Профили' },
    { id: 'lists', label: 'Списки обхода', compactLabel: 'Списки' },
    { id: 'settings', label: 'Настройки', compactLabel: 'Настройки' },
  ];

  return (
    <div className="flex bg-[var(--ui-surface-elevated)] border border-[var(--ui-border)] rounded-[var(--ui-radius)] p-1 gap-1 shrink-0">
      {tabs.map((tab) => {
        const isActive = activeTab === tab.id;
        return (
          <button
            key={tab.id}
            onClick={() => onTabChange(tab.id)}
            className={cn(
              'flex-1 min-w-0 text-center text-xs font-semibold py-1.5 px-2.5 rounded-md transition-all whitespace-nowrap overflow-hidden',
              isActive
                ? 'bg-[var(--ui-panel)] text-[var(--ui-text)] shadow-sm'
                : 'text-[var(--ui-text-muted)] hover:text-[var(--ui-text)]'
            )}
          >
            <span className="hidden sm:inline">{tab.label}</span>
            <span className="sm:hidden">{tab.compactLabel}</span>
          </button>
        );
      })}
    </div>
  );
};
