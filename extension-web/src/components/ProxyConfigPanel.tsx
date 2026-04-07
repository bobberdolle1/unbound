import { useState, useEffect } from 'react';
import { type ProxyConfig } from '../types';
import { getState, setState } from '../utils/storage';

export function ProxyConfigPanel() {
  const [config, setConfig] = useState<ProxyConfig>({
    protocol: 'https',
    host: '',
    port: 443,
  });
  const [isEditing, setIsEditing] = useState(false);
  const [tempConfig, setTempConfig] = useState(config);

  useEffect(() => {
    getState().then((state) => {
      if (state.proxyConfig) {
        setConfig(state.proxyConfig);
        setTempConfig(state.proxyConfig);
      }
    });
  }, []);

  const saveConfig = async () => {
    if (!tempConfig.host || tempConfig.port < 1 || tempConfig.port > 65535) {
      return;
    }

    setConfig(tempConfig);
    await setState({ proxyConfig: tempConfig });
    setIsEditing(false);
  };

  const cancelEdit = () => {
    setTempConfig(config);
    setIsEditing(false);
  };

  if (!isEditing) {
    return (
      <div className="space-y-2">
        <div className="flex items-center justify-between">
          <label className="text-xs font-medium text-[var(--text-muted)] uppercase tracking-wide">
            Proxy Server
          </label>
          <button
            onClick={() => setIsEditing(true)}
            className="text-xs text-[var(--color-accent)] hover:underline"
          >
            Edit
          </button>
        </div>
        <div className="px-3 py-2 rounded-lg bg-[var(--bg-surface)] border border-[var(--border-color)]">
          {config.host ? (
            <div className="flex items-center gap-2">
              <span className="text-xs font-mono text-[var(--text-primary)]">
                {config.protocol}://{config.host}:{config.port}
              </span>
              <span className="ml-auto w-2 h-2 rounded-full bg-green-500"></span>
            </div>
          ) : (
            <p className="text-xs text-[var(--text-muted)] italic">
              No proxy configured
            </p>
          )}
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-3 p-3 rounded-lg bg-[var(--bg-surface)] border border-[var(--border-color)]">
      <label className="text-xs font-medium text-[var(--text-muted)] uppercase tracking-wide block">
        Proxy Configuration
      </label>

      {/* Protocol selector */}
      <div className="space-y-1">
        <label className="text-xs text-[var(--text-muted)]">Protocol</label>
        <div className="grid grid-cols-2 gap-2">
          {(['https', 'socks5'] as const).map((proto) => (
            <button
              key={proto}
              onClick={() => setTempConfig(prev => ({ ...prev, protocol: proto }))}
              className={`
                px-3 py-1.5 rounded-md text-xs font-medium transition-all
                ${tempConfig.protocol === proto
                  ? 'bg-[var(--color-primary)] text-white'
                  : 'bg-[var(--border-color)] text-[var(--text-muted)]'
                }
              `}
            >
              {proto.toUpperCase()}
            </button>
          ))}
        </div>
      </div>

      {/* Host input */}
      <div className="space-y-1">
        <label className="text-xs text-[var(--text-muted)]">Host</label>
        <input
          type="text"
          value={tempConfig.host}
          onChange={(e) => setTempConfig(prev => ({ ...prev, host: e.target.value }))}
          placeholder="proxy.example.com"
          className="w-full px-3 py-2 text-sm rounded-md border border-[var(--border-color)] 
                     bg-[var(--bg-primary)] text-[var(--text-primary)]
                     placeholder-[var(--text-muted)]
                     focus:outline-none focus:ring-2 focus:ring-[var(--color-accent)]"
        />
      </div>

      {/* Port input */}
      <div className="space-y-1">
        <label className="text-xs text-[var(--text-muted)]">Port</label>
        <input
          type="number"
          value={tempConfig.port}
          onChange={(e) => setTempConfig(prev => ({ ...prev, port: parseInt(e.target.value) || 0 }))}
          min="1"
          max="65535"
          className="w-full px-3 py-2 text-sm rounded-md border border-[var(--border-color)] 
                     bg-[var(--bg-primary)] text-[var(--text-primary)]
                     placeholder-[var(--text-muted)]
                     focus:outline-none focus:ring-2 focus:ring-[var(--color-accent)]"
        />
      </div>

      {/* Actions */}
      <div className="flex gap-2 pt-2">
        <button
          onClick={saveConfig}
          disabled={!tempConfig.host || tempConfig.port < 1 || tempConfig.port > 65535}
          className="flex-1 px-3 py-2 rounded-md bg-[var(--color-primary)] text-white 
                     hover:bg-[var(--color-primary-hover)] transition-colors disabled:opacity-50 disabled:cursor-not-allowed
                     text-sm font-medium"
        >
          Save
        </button>
        <button
          onClick={cancelEdit}
          className="px-3 py-2 rounded-md bg-[var(--border-color)] text-[var(--text-muted)] 
                     hover:text-[var(--text-primary)] transition-colors text-sm"
        >
          Cancel
        </button>
      </div>
    </div>
  );
}
