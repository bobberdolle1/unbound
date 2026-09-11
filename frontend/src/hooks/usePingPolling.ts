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

    const poll = () => {
      backendService
        .getLivePing()
        .then((data: Record<string, unknown>) => {
          const lat = typeof data?.latency === 'number' ? data.latency : 0;
          const stat = typeof data?.status === 'string' ? data.status : 'stopped';
          const serv: Record<string, number> = {};
          if (data?.services && typeof data.services === 'object') {
            for (const [service, latency] of Object.entries(data.services)) {
              if (typeof latency === 'number') serv[service] = latency;
            }
          }
          setLivePingData({ active: Boolean(data?.active), latency: lat, status: stat, services: serv });
          if (stat === 'ok' && lat > 0) {
            setPingHistory((prev) => [...prev.slice(-14), lat]);
          }
          backendService.savePingHistory(lat, stat).catch(() => {});
        })
        .catch(() => setLivePingData({ active: false, latency: 0, status: 'error' }));
    };

    poll();
    const interval = setInterval(poll, 4000);
    return () => clearInterval(interval);
  }, [status]);

  return { livePingData, pingHistory };
}
