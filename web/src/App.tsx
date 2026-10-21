import { useEffect, useState } from 'react';
import { Header } from './components/Header';
import { MaintenanceBannerBar } from './components/MaintenanceBannerBar';
import { ZoneGroup } from './components/ZoneGroup';
import { fetchDashboard, subscribeToEvents, DashboardData } from './services/api';

export function App() {
  const [data, setData] = useState<DashboardData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [isStreaming, setIsStreaming] = useState(true);

  const loadData = async () => {
    try {
      setLoading(true);
      const res = await fetchDashboard();
      setData(res);
      setError(null);
    } catch (err: unknown) {
      if (err instanceof Error) {
        setError(err.message);
      } else {
        setError(String(err));
      }
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadData();

    // Subscribe to real-time events via Server-Sent Events (SSE)
    const unsubscribe = subscribeToEvents((heartbeat) => {
      setIsStreaming(true);
      setData((prev) => {
        if (!prev) return prev;

        const updatedZones = prev.zones.map((zone) => {
          const updatedEndpoints = zone.endpoints.map((ep) => {
            if (ep.id !== heartbeat.endpoint_id) return ep;

            const updatedHeartbeats = [
              ...ep.heartbeats,
              {
                id: Date.now(),
                timestamp: heartbeat.timestamp,
                status: heartbeat.status as 'UP' | 'DOWN' | 'MAINTENANCE' | 'PENDING',
                latency_ms: heartbeat.latency_ms,
              },
            ];

            return {
              ...ep,
              status: heartbeat.status as 'UP' | 'DOWN' | 'MAINTENANCE' | 'PENDING',
              latency_ms: heartbeat.latency_ms,
              heartbeats: updatedHeartbeats.slice(-60),
            };
          });

          return {
            ...zone,
            endpoints: updatedEndpoints,
          };
        });

        return {
          ...prev,
          zones: updatedZones,
        };
      });
    });

    return () => {
      unsubscribe();
    };
  }, []);

  return (
    <div className="min-h-screen bg-background text-zinc-100 flex flex-col font-sans selection:bg-emerald-500/20 selection:text-emerald-300">
      <Header data={data} isStreaming={isStreaming} />

      {data && <MaintenanceBannerBar maintenances={data.one_off_maintenances} />}

      <main className="max-w-7xl mx-auto px-4 sm:px-6 py-6 flex-1 w-full space-y-6">
        {loading && !data && (
          <div className="flex flex-col items-center justify-center py-20 text-zinc-500 gap-2">
            <span className="w-4 h-4 border border-zinc-700 border-t-zinc-400 rounded-full animate-spin"></span>
            <p className="text-xs font-mono">Connecting to monitoring daemon...</p>
          </div>
        )}

        {error && (
          <div className="p-3 rounded-md bg-rose-500/10 border border-rose-500/20 text-rose-400 text-xs font-mono">
            Error loading status: {error}
          </div>
        )}

        {data && (
          <div className="space-y-6">
            {data.zones.map((zone) => (
              <ZoneGroup key={zone.id} zone={zone} />
            ))}
          </div>
        )}
      </main>

      <footer className="border-t border-zinc-800/80 py-4 text-center text-xs text-zinc-600 font-mono">
        <p>uptime-mon • Single-Binary Declarative Uptime Monitor</p>
      </footer>
    </div>
  );
}

export default App;
