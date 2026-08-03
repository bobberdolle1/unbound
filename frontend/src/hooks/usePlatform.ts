import { useState, useEffect } from 'react';
import { backendService } from '../services/backend';

export function usePlatform() {
  const [platform, setPlatform] = useState<string>('');
  const [appVersion, setAppVersion] = useState<string>('');

  useEffect(() => {
    backendService.getAppVersion().then(setAppVersion).catch(() => setAppVersion(''));
    backendService.getOSPlatform().then(setPlatform).catch(() => setPlatform(''));
  }, []);

  return { platform, appVersion };
}
