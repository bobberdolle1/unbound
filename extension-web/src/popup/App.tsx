import { useState, useEffect, useCallback } from 'react';
import { type ExtensionState, type Theme, type ExtensionMode, type ConnectionStatus } from '../types';
import { getState, setState, applyTheme } from '../utils/storage';
import { ConnectToggle } from '../components/ConnectToggle';
import { ModeSelector } from '../components/ModeSelector';
import { DomainList } from '../components/DomainList';
import { ThemeSwitcher } from '../components/ThemeSwitcher';
import { ProxyConfigPanel } from '../components/ProxyConfigPanel';

export default function App() {
  const [state, setStateLocal] = useState<ExtensionState>({
    mode: 'companion',
    status: 'disconnected',
    theme: 'modern-dark',
    bypassDomains: [],
  });
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    // Load initial state
    getState().then((savedState) => {
      setStateLocal(savedState);
      applyTheme(savedState.theme);
      setIsLoading(false);
    });

    // Listen for state changes from background
    const handleStorageChange = (changes: Record<string, any>, namespace: string) => {
      if (namespace !== 'local') return;
      
      getState().then((updatedState) => {
        setStateLocal(updatedState);
        applyTheme(updatedState.theme);
      });
    };

    chrome.storage.onChanged.addListener(handleStorageChange);
    return () => chrome.storage.onChanged.removeListener(handleStorageChange);
  }, []);

  const handleToggle = useCallback(async (connected: boolean) => {
    if (connected) {
      setStateLocal(prev => ({ ...prev, status: 'connecting' }));
      await setState({ status: 'connecting' });
      
      // Send command to background
      chrome.runtime.sendMessage({ 
        action: 'connect', 
        mode: state.mode,
        domains: state.bypassDomains,
      });
    } else {
      setStateLocal(prev => ({ ...prev, status: 'disconnected' }));
      await setState({ status: 'disconnected' });
      
      chrome.runtime.sendMessage({ action: 'disconnect' });
    }
  }, [state.mode, state.bypassDomains]);

  const handleModeChange = useCallback(async (mode: ExtensionMode) => {
    setStateLocal(prev => ({ ...prev, mode }));
    await setState({ mode });
    
    // If connected, restart with new mode
    if (state.status === 'connected') {
      chrome.runtime.sendMessage({ action: 'restart', mode });
    }
  }, [state.status]);

  const handleThemeChange = useCallback(async (theme: Theme) => {
    setStateLocal(prev => ({ ...prev, theme }));
    await setState({ theme });
    applyTheme(theme);
  }, []);

  const handleDomainsChange = useCallback(async (domains: string[]) => {
    setStateLocal(prev => ({ ...prev, bypassDomains: domains }));
    await setState({ bypassDomains: domains });
    
    // If connected in standalone mode, update PAC
    if (state.status === 'connected' && state.mode === 'standalone') {
      chrome.runtime.sendMessage({ 
        action: 'update_domains', 
        domains,
      });
    }
  }, [state.status, state.mode]);

  if (isLoading) {
    return (
      <div className="w-80 h-96 flex items-center justify-center bg-doodle-bg dark:bg-dark-bg">
        <div className="animate-spin rounded-full h-8 w-8 border-2 border-doodle-primary dark:border-dark-accent border-t-transparent"></div>
      </div>
    );
  }

  return (
    <div className="w-80 min-h-96 bg-[var(--bg-primary)] transition-colors duration-300">
      <div className="p-4 space-y-4">
        {/* Header */}
        <header className="flex items-center justify-between">
          <h1 className="text-lg font-bold text-[var(--text-primary)]">
            <span className="text-[var(--color-accent)]">Unbound</span> Web
          </h1>
          <ThemeSwitcher currentTheme={state.theme} onThemeChange={handleThemeChange} />
        </header>

        {/* Main Toggle */}
        <ConnectToggle 
          status={state.status} 
          onToggle={handleToggle} 
        />

        {/* Mode Selector */}
        <ModeSelector 
          mode={state.mode} 
          onModeChange={handleModeChange}
          disabled={state.status === 'connecting'}
        />

        {/* Proxy Config (only in standalone mode) */}
        {state.mode === 'standalone' && (
          <ProxyConfigPanel />
        )}

        {/* Domain List */}
        <DomainList 
          domains={state.bypassDomains}
          onDomainsChange={handleDomainsChange}
        />

        {/* Status Footer */}
        <footer className="pt-2 border-t border-[var(--border-color)]">
          <p className="text-xs text-[var(--text-muted)] text-center">
            {state.status === 'connected' ? '✓ Active' : 
             state.status === 'connecting' ? '⟳ Connecting...' :
             state.status === 'error' ? '✗ Error' : 'Disconnected'}
            {state.mode === 'companion' ? ' • Companion Mode' : ' • Standalone'}
          </p>
        </footer>
      </div>
    </div>
  );
}
