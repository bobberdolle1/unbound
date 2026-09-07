import React from 'react';
import { cn } from '../../lib/cn';
import { UIGear, UIStar, UIZap } from '../icons';

interface ProfilesViewProps {
  sortedProfiles: string[];
  selectedProfile: string;
  setSelectedProfile: (profile: string) => void;
  favoriteProfiles: string[];
  isConnected: boolean;
  openLuaEditor: () => void;
  openStrategyLab?: () => void;
}
export const ProfilesView: React.FC<ProfilesViewProps> = ({
  sortedProfiles,
  selectedProfile,
  setSelectedProfile,
  favoriteProfiles,
  isConnected,
  openLuaEditor,
  openStrategyLab,
}) => {
  return (
    <div className="flex-1 flex flex-col gap-4">
      <div className="bg-[var(--ui-surface-elevated)] border border-[var(--ui-border)] rounded-[var(--ui-radius)] p-4 space-y-3">
        <h3 className="text-sm font-semibold text-[var(--ui-text)]">Управление профилями обхода</h3>
        <p className="text-xs text-[var(--ui-text-muted)]">
          Выбирайте готовые профили или настраивайте собственные кастомные LUA-скрипты.
        </p>

        <div className="space-y-2 pt-2">
          {sortedProfiles.map((prof, idx) => (
            <div
              key={idx}
              onClick={() => !isConnected && setSelectedProfile(prof)}
              className={cn(
                'p-3 rounded-lg border text-xs font-medium flex items-center justify-between cursor-pointer transition-all',
                selectedProfile === prof
                  ? 'border-[var(--ui-border-strong)] bg-[var(--ui-panel)] text-[var(--ui-text)]'
                  : 'border-[var(--ui-border)] text-[var(--ui-text-muted)] hover:text-[var(--ui-text)]'
              )}
            >
              <div className="flex items-center gap-2">
                <span>{prof}</span>
                {prof.includes('Adaptive') && (
                  <span className="text-[9px] px-1.5 py-0.5 rounded font-mono font-semibold bg-purple-500/20 text-purple-400 border border-purple-500/30">
                    EXPERIMENTAL
                  </span>
                )}
              </div>
              {favoriteProfiles.includes(prof) && <UIStar className="w-3.5 h-3.5 fill-current text-amber-400" />}
            </div>
          ))}
        </div>

        <div className="grid grid-cols-1 sm:grid-cols-2 gap-2 mt-3">
          <button
            onClick={openStrategyLab}
            className="btn-ui-primary flex items-center justify-center gap-1.5 py-2 text-xs"
          >
            <UIZap className="w-4 h-4 text-emerald-400" />
            <span>Strategy Lab</span>
          </button>
          <button
            onClick={openLuaEditor}
            className="btn-ui-secondary flex items-center justify-center gap-1.5 py-2 text-xs"
          >
            <UIGear className="w-4 h-4" />
            <span>Конструктор LUA</span>
          </button>
        </div>
      </div>
    </div>
  );
};
