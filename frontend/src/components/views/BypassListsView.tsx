import React from 'react';
import { UISelect } from '../UISelect';
import { UICheck, UISpinner } from '../icons';

interface BypassListsViewProps {
  selectedList: string;
  hostlists: string[];
  handleSelectHostlist: (name: string) => void;
  hostlistContent: string;
  setHostlistContent: (content: string) => void;
  handleSaveHostlist: () => void;
  isSavingHostlist: boolean;
}

export const BypassListsView: React.FC<BypassListsViewProps> = ({
  selectedList,
  hostlists,
  handleSelectHostlist,
  hostlistContent,
  setHostlistContent,
  handleSaveHostlist,
  isSavingHostlist,
}) => {
  return (
    <div className="flex-1 flex flex-col gap-3">
      <div className="bg-[var(--ui-surface-elevated)] border border-[var(--ui-border)] rounded-[var(--ui-radius)] p-4 flex flex-col gap-3">
        <div className="flex items-center justify-between">
          <span className="text-xs font-semibold text-[var(--ui-text-muted)] uppercase">Файл списка:</span>
          <UISelect
            value={selectedList}
            options={hostlists}
            onChange={handleSelectHostlist}
          />
        </div>

        <div className="flex flex-wrap gap-1">
          <button
            type="button"
            onClick={() => {
              const preset = ['youtube.com', 'googlevideo.com', 'ytimg.com', 'youtu.be'];
              const existing = new Set(hostlistContent.split('\n').map((l) => l.trim().toLowerCase()));
              const toAdd = preset.filter((d) => !existing.has(d));
              if (toAdd.length > 0) {
                setHostlistContent(
                  hostlistContent.trim() ? hostlistContent.trim() + '\n' + toAdd.join('\n') : toAdd.join('\n')
                );
              }
            }}
            className="px-2 py-1 text-[11px] font-semibold rounded bg-[var(--ui-panel)] border border-[var(--ui-border)] text-[var(--ui-text-muted)] hover:text-[var(--ui-text)]"
          >
            + YouTube
          </button>
          <button
            type="button"
            onClick={() => {
              const preset = ['discord.com', 'discord.gg', 'discord.media', 'discordapp.com'];
              const existing = new Set(hostlistContent.split('\n').map((l) => l.trim().toLowerCase()));
              const toAdd = preset.filter((d) => !existing.has(d));
              if (toAdd.length > 0) {
                setHostlistContent(
                  hostlistContent.trim() ? hostlistContent.trim() + '\n' + toAdd.join('\n') : toAdd.join('\n')
                );
              }
            }}
            className="px-2 py-1 text-[11px] font-semibold rounded bg-[var(--ui-panel)] border border-[var(--ui-border)] text-[var(--ui-text-muted)] hover:text-[var(--ui-text)]"
          >
            + Discord
          </button>
        </div>

        <textarea
          value={hostlistContent}
          onChange={(e) => setHostlistContent(e.target.value)}
          className="w-full h-48 p-3 font-mono text-xs border rounded-lg bg-[var(--ui-panel)] text-[var(--ui-text)] border-[var(--ui-border)] focus:outline-none resize-none"
          placeholder="domain.com"
          spellCheck={false}
        />

        <button
          onClick={handleSaveHostlist}
          disabled={isSavingHostlist}
          className="btn-ui-primary w-full"
        >
          {isSavingHostlist ? <UISpinner className="w-4 h-4 animate-spin" /> : <UICheck className="w-4 h-4" />}
          <span>Сохранить список</span>
        </button>
      </div>
    </div>
  );
};
