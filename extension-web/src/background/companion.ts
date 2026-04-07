// Companion mode - handles native messaging integration
// This module communicates with the Unbound Desktop daemon

import browser from 'webextension-polyfill';
import { getState, setState } from '../utils/storage';

let nativePort: browser.Runtime.Port | null = null;

/**
 * Initialize connection to native messaging host
 */
export async function connectToNativeHost(): Promise<browser.Runtime.Port | null> {
  try {
    nativePort = browser.runtime.connectNative('com.unbound.desktop');

    nativePort.onMessage.addListener((message: any) => {
      console.log('[Companion] Native message received:', message);
      handleNativeMessage(message);
    });

    nativePort.onDisconnect.addListener(() => {
      console.log('[Companion] Native port disconnected');
      const error = browser.runtime.lastError;
      if (error) {
        console.error('[Companion] Disconnect reason:', error.message);
      }
      nativePort = null;
    });

    // Wait a bit for connection to establish
    await new Promise(resolve => setTimeout(resolve, 200));
    
    return nativePort;
  } catch (error) {
    console.error('[Companion] Failed to connect to native host:', error);
    return null;
  }
}

/**
 * Send start command to native host
 */
export async function sendStartCommand(domains: string[]): Promise<boolean> {
  if (!nativePort) {
    console.warn('[Companion] Native port not available, attempting to connect...');
    nativePort = await connectToNativeHost();
    
    if (!nativePort) {
      return false;
    }
  }

  try {
    nativePort.postMessage({
      command: 'start',
      domains: domains,
    });
    return true;
  } catch (error) {
    console.error('[Companion] Failed to send start command:', error);
    nativePort = null;
    return false;
  }
}

/**
 * Send stop command to native host
 */
export async function sendStopCommand(): Promise<void> {
  if (!nativePort) {
    console.warn('[Companion] Native port not available');
    return;
  }

  try {
    nativePort.postMessage({ command: 'stop' });
  } catch (error) {
    console.error('[Companion] Failed to send stop command:', error);
  }
}

/**
 * Send status query to native host
 */
export async function sendStatusQuery(): Promise<void> {
  if (!nativePort) {
    return;
  }

  try {
    nativePort.postMessage({ command: 'status' });
  } catch (error) {
    console.error('[Companion] Failed to send status query:', error);
  }
}

/**
 * Update domains on native host
 */
export async function sendDomainUpdate(domains: string[]): Promise<void> {
  if (!nativePort) {
    console.warn('[Companion] Native port not available');
    return;
  }

  try {
    nativePort.postMessage({
      command: 'update_domains',
      domains: domains,
    });
  } catch (error) {
    console.error('[Companion] Failed to update domains:', error);
  }
}

/**
 * Disconnect from native host
 */
export function disconnectNative(): void {
  if (nativePort) {
    try {
      nativePort.disconnect();
    } catch (error) {
      console.error('[Companion] Error disconnecting:', error);
    }
    nativePort = null;
  }
}

/**
 * Handle incoming messages from native host
 */
function handleNativeMessage(message: any): void {
  console.log('[Companion] Processing native message:', message);

  switch (message.status) {
    case 'running':
      setState({ 
        status: 'connected',
      });
      // Notify popup
      browser.runtime.sendMessage({
        type: 'NATIVE_STATUS_UPDATE',
        payload: { status: 'connected', version: message.version },
      });
      break;

    case 'stopped':
      setState({ 
        status: 'disconnected',
      });
      browser.runtime.sendMessage({
        type: 'NATIVE_STATUS_UPDATE',
        payload: { status: 'disconnected' },
      });
      break;

    case 'error':
      setState({ 
        status: 'error',
      });
      browser.runtime.sendMessage({
        type: 'NATIVE_STATUS_UPDATE',
        payload: { status: 'error', message: message.message },
      });
      break;

    default:
      console.warn('[Companion] Unknown native message:', message);
  }
}

/**
 * Check if native host is installed and accessible
 */
export async function checkNativeHostAvailability(): Promise<boolean> {
  try {
    const port = browser.runtime.connectNative('com.unbound.desktop');
    
    // If we get here without immediate error, host might be available
    // We'll know for sure when we try to send a message
    port.disconnect();
    return true;
  } catch (error) {
    console.warn('[Companion] Native host not available:', error);
    return false;
  }
}
