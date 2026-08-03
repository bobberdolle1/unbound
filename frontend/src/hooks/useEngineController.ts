import { useState, useEffect, useCallback } from 'react';
import { backendService } from '../services/backend';

export function useEngineController(initialStatus: string = 'Stopped') {
  const [engines, setEngines] = useState<string[]>([]);
  const [selectedEngine, setSelectedEngine] = useState<string>('');
  const [profiles, setProfiles] = useState<string[]>([]);
  const [selectedProfile, setSelectedProfile] = useState<string>('');
  const [favoriteProfiles, setFavoriteProfiles] = useState<string[]>([]);
  const [status, setStatus] = useState<string>(initialStatus);

  // AutoTune States
  const [isScanning, setIsScanning] = useState<boolean>(false);
  const [scanProgress, setScanProgress] = useState<string>('');
  const [autotuneProgress, setAutotuneProgress] = useState<{ msg?: string; progress?: number } | null>(null);

  // Load Engines on mount
  useEffect(() => {
    const loadEngines = async (retryCount = 0) => {
      try {
        const e = await backendService.getEngineNames();
        if (e && e.length > 0) {
          setEngines(e);
          setSelectedEngine((prev) => (prev && e.includes(prev) ? prev : e[0]));
        } else if (retryCount < 3) {
          setTimeout(() => loadEngines(retryCount + 1), [300, 800, 1500][retryCount]);
        }
      } catch (err) {
        if (retryCount < 3) {
          setTimeout(() => loadEngines(retryCount + 1), [300, 800, 1500][retryCount]);
        }
      }
    };
    loadEngines();

    backendService.getFavoriteProfiles().then((favs: string[]) => {
      setFavoriteProfiles(favs || []);
    }).catch(() => {});
  }, []);

  // Fetch Profiles when selectedEngine changes
  useEffect(() => {
    let engineToUse = selectedEngine;
    if ((!engineToUse || (engines.length > 0 && !engines.includes(engineToUse))) && engines.length > 0) {
      engineToUse = engines[0];
      setSelectedEngine(engineToUse);
    }
    if (engineToUse) {
      backendService.getProfiles(engineToUse).then((p: string[]) => {
        const list = p || [];
        setProfiles(list);
        if (list.length > 0) {
          setSelectedProfile((prev) => (prev && list.includes(prev) ? prev : list[0]));
        }
      }).catch(() => {});
    }
  }, [selectedEngine, engines]);

  const toggleConnection = useCallback(async () => {
    if (status === 'Running') {
      try {
        await backendService.stopEngine();
        setStatus('Stopped');
      } catch (err) {
        console.error('StopEngine failed:', err);
      }
    } else {
      try {
        setStatus('Starting');
        await backendService.startEngine(selectedEngine, selectedProfile);
        setStatus('Running');
      } catch (err) {
        console.error('StartEngine failed:', err);
        setStatus('Stopped');
      }
    }
  }, [status, selectedEngine, selectedProfile]);

  const handleToggleFavorite = useCallback(async () => {
    if (!selectedProfile) return;
    try {
      const favs = await backendService.toggleFavoriteProfile(selectedProfile);
      setFavoriteProfiles(favs || []);
    } catch (err) {
      console.error('ToggleFavorite failed:', err);
    }
  }, [selectedProfile]);

  const handleAutoTune = useCallback(async () => {
    if (!selectedEngine) return;
    setIsScanning(true);
    setScanProgress('Инициализация автоподбора...');
    setAutotuneProgress(null);
    try {
      await backendService.autoTune();
    } catch (err) {
      console.error('AutoTune error:', err);
      setIsScanning(false);
    }
  }, [selectedEngine]);

  const cancelAutoTune = useCallback(async () => {
    try {
      await backendService.cancelAutoTune();
    } catch (err) {
      console.error('CancelAutoTune error:', err);
    } finally {
      setIsScanning(false);
    }
  }, []);

  const sortedProfiles = [...profiles].sort((a, b) => {
    const aFav = favoriteProfiles.includes(a) ? -1 : 0;
    const bFav = favoriteProfiles.includes(b) ? -1 : 0;
    return aFav - bFav;
  });

  return {
    state: {
      engines,
      selectedEngine,
      profiles,
      selectedProfile,
      sortedProfiles,
      favoriteProfiles,
      status,
      isScanning,
      scanProgress,
      autotuneProgress,
    },
    actions: {
      setSelectedEngine,
      setSelectedProfile,
      setStatus,
      setEngines,
      setProfiles,
      setIsScanning,
      setScanProgress,
      setAutotuneProgress,
      toggleConnection,
      handleToggleFavorite,
      handleAutoTune,
      cancelAutoTune,
    },
  };
}
