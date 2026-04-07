import browser from 'webextension-polyfill';

/**
 * Detect the current browser and return browser-specific utilities
 */
export function getBrowserInfo() {
  // Firefox detection
  if (typeof browser !== 'undefined' && browser.runtime.getBrowserInfo) {
    return {
      name: 'firefox',
      supportsMV3: true,
      supportsNativeMessaging: true,
      supportsProxyAPI: true,
    };
  }
  
  // Chrome/Edge detection
  if (typeof chrome !== 'undefined') {
    const isEdge = navigator.userAgent.includes('Edg/');
    return {
      name: isEdge ? 'edge' : 'chrome',
      supportsMV3: true,
      supportsNativeMessaging: true,
      supportsProxyAPI: true,
    };
  }
  
  // Fallback
  return {
    name: 'unknown',
    supportsMV3: false,
    supportsNativeMessaging: false,
    supportsProxyAPI: false,
  };
}

/**
 * Wrap chrome.* API to be more promise-based like browser.* API
 */
export function promisifyChromeAPI() {
  const promisified = {
    runtime: {
      sendMessage: (message: any) => chrome.runtime.sendMessage(message),
      connectNative: (name: string) => chrome.runtime.connectNative(name),
      onMessage: {
        addListener: (callback: (message: any, sender: any, sendResponse: any) => void) => {
          chrome.runtime.onMessage.addListener(callback);
        },
        removeListener: (callback: any) => {
          chrome.runtime.onMessage.removeListener(callback);
        },
      },
    },
    storage: {
      local: {
        get: (keys?: string | string[] | Record<string, any> | null) => 
          new Promise((resolve) => chrome.storage.local.get(keys, resolve)),
        set: (items: Record<string, any>) => 
          new Promise((resolve) => chrome.storage.local.set(items, resolve)),
      },
      onChanged: chrome.storage.onChanged,
    },
    proxy: {
      settings: {
        set: (details: any) => 
          new Promise((resolve) => chrome.proxy.settings.set(details, resolve)),
      },
    },
    action: {
      setIcon: (details: any) => 
        new Promise((resolve) => chrome.action.setIcon(details, resolve)),
    },
    alarms: {
      create: (name: string, alarmInfo: any) => chrome.alarms.create(name, alarmInfo),
      onAlarm: chrome.alarms.onAlarm,
    },
  };

  return promisified;
}
