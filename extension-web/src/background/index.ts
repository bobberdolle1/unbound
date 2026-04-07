import browser from 'webextension-polyfill';
import { type ExtensionMode, type ProxyConfig } from '../types';
import { getState, setState } from '../utils/storage';
import * as Companion from './companion';
import * as Standalone from './standalone';

// State
let isActive = false;

// Initialize
async function init() {
  console.log('[Unbound] Background service worker initialized');
  
  // Setup native messaging listeners
  setupNativeMessagingListeners();
  
  // Restore state from storage
  const state = await getState();
  
  // If was connected, attempt to reconnect
  if (state.status === 'connected' || state.status === 'connecting') {
    await handleConnect(state.mode, state.bypassDomains, state.proxyConfig);
  }
}

// Setup native messaging listeners
function setupNativeMessagingListeners() {
  // Listen for messages from companion module
  browser.runtime.onMessage.addListener((message: any) => {
    if (message.type === 'NATIVE_STATUS_UPDATE') {
      console.log('[Unbound] Native status update from companion:', message.payload);
      // Update state based on native host response
      if (message.payload.status) {
        setState({ status: message.payload.status });
        updateIcon(message.payload.status === 'connected');
      }
    }
  });
}

// Handle connect command
async function handleConnect(
  mode: ExtensionMode, 
  domains: string[], 
  proxyConfig?: ProxyConfig
) {
  if (isActive) {
    console.warn('[Unbound] Already active, ignoring connect');
    return;
  }

  isActive = true;
  await setState({ status: 'connecting' });

  try {
    if (mode === 'companion') {
      const success = await Companion.sendStartCommand(domains);
      if (!success) {
        throw new Error('Failed to connect to native host');
      }
    } else {
      if (!proxyConfig) {
        throw new Error('Proxy configuration required for standalone mode');
      }
      await Standalone.enableProxy(domains, proxyConfig);
    }

    await setState({ status: 'connected' });
    updateIcon(true);
    console.log('[Unbound] Connected successfully');
  } catch (error) {
    console.error('[Unbound] Connection failed:', error);
    await setState({ status: 'error' });
    updateIcon(false);
    isActive = false;
  }
}

// Handle disconnect command
async function handleDisconnect() {
  if (!isActive) {
    console.warn('[Unbound] Not active, ignoring disconnect');
    return;
  }

  isActive = false;

  try {
    await Companion.sendStopCommand();
    await Standalone.disableProxy();
    Companion.disconnectNative();
    await setState({ status: 'disconnected' });
    updateIcon(false);
    console.log('[Unbound] Disconnected');
  } catch (error) {
    console.error('[Unbound] Disconnect error:', error);
  }
}

// Handle domain update
async function handleDomainUpdate(domains: string[]) {
  const state = await getState();
  
  if (state.mode === 'standalone' && isActive && state.proxyConfig) {
    await Standalone.updateDomains(domains);
  }
  
  if (state.mode === 'companion' && isActive) {
    await Companion.sendDomainUpdate(domains);
  }
  
  await setState({ bypassDomains: domains });
}

// Update extension icon based on status
function updateIcon(connected: boolean) {
  const iconPath = connected ? 'icons/icon128-active.png' : 'icons/icon128.png';
  
  browser.action.setIcon({
    path: {
      '16': iconPath,
      '32': iconPath,
      '48': iconPath,
      '128': iconPath,
    }
  });
}

// Message listener
browser.runtime.onMessage.addListener((message: any, _sender: browser.Runtime.MessageSender) => {
  console.log('[Unbound] Message received:', message);

  switch (message.action) {
    case 'connect':
      return handleConnect(message.mode, message.domains || [], message.proxyConfig);
    
    case 'disconnect':
      return handleDisconnect();
    
    case 'restart':
      return handleDisconnect().then(() => 
        handleConnect(message.mode, message.domains || [], message.proxyConfig)
      );
    
    case 'update_domains':
      return handleDomainUpdate(message.domains);
    
    default:
      console.warn('[Unbound] Unknown action:', message.action);
  }
});

// Initialize on service worker startup
init();

// Keep service worker alive with periodic heartbeat (Manifest V3 requirement)
browser.alarms.create('unbound-heartbeat', { periodInMinutes: 5 });
browser.alarms.onAlarm.addListener(async (alarm) => {
  if (alarm.name === 'unbound-heartbeat' && isActive) {
    const state = await getState();
    
    if (state.mode === 'companion') {
      await Companion.sendStatusQuery();
    }
  }
});

// Export for Vite module bundling
export {};
