import { useState, useEffect } from 'react';
import { backendService } from '../services/backend';
import { main } from '../../wailsjs/go/models';

export function usePingPolling(status: string) {
  const [livePingData, setLivePingData] = useState<{
    active: boolean;
    latency: number;
    status: string;
    services?: Record<string, number>;
  }>({ active: false, latency: 0, status: 'stopped' });

  const [pingHistory, setPingHistory] = useState<number[]>([]);

  useEffect(() => {
    let isMounted = true;
    backendService
      .loadPingHistory()
      .then((records: main.PingRecord[]) => {
        if (isMounted && records && records.length > 0) {
          const recent = records
            .slice(-15)
            .map((r: main.PingRecord) => r.lat || 0)
            .filter((l: number) => l > 0);
          if (recent.length > 0) setPingHistory(recent);
        }
      })
      .catch(() => {});

    return () => {
      isMounted = false;
    };
  }, []);

  useEffect(() => {
    if (status !== 'Running') {
      setLivePingData({ active: false, latency: 0, status: 'stopped' });
      setPingHistory([]);
      return;
    }

    const interval = setInterval(() => {
      backendService
        .getLivePing()
        .then((data: any) => {
          const lat = data?.latency || 0;
          const stat = data?.status || 'stopped';
          const serv = data?.services || {};
          setLivePingData({ active: data?.active || false, latency: lat, status: stat, services: serv });
          if (stat === 'ok') {
            setPingHistory((prev) => [...prev.slice(-14), lat]);
          }
          backendService.savePingHistory(lat, stat).catch(() => {});
        })
        .catch(() => setLivePingData({ active: false, latency: 0, status: 'error' }));
    }, 5000);

    return () => clearInterval(interval);
  }, [status]);

  return { livePingData, pingHistory };
}
