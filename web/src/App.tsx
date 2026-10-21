import { useEffect, useState } from 'react';
import { Header } from './components/Header';
import { MaintenanceBannerBar } from './components/MaintenanceBannerBar';
import { ZoneGroup } from './components/ZoneGroup';
import { fetchDashboard, subscribeToEvents, DashboardData } from './services/api';
import { RefreshCw } from 'lucide-react';

export function App() {
  const [data, setData] = useState<DashboardData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

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
    <div className="min-h-screen bg-background text-slate-100 flex flex-col font-sans">
      <Header data={data} />

      {data && <MaintenanceBannerBar maintenances={data.one_off_maintenances} />}

      <main className="max-w-7xl mx-auto px-6 py-8 flex-1 w-full space-y-8">
        {loading && !data && (
          <div className="flex flex-col items-center justify-center py-20 text-slate-400 gap-3">
            <RefreshCw className="w-8 h-8 animate-spin text-emerald-400" />
            <p className="text-sm">Connecting to monitoring daemon...</p>
          </div>
        )}

        {error && (
          <div className="p-4 rounded-xl bg-rose-500/10 border border-rose-500/20 text-rose-300 text-sm">
            Failed to load status dashboard: {error}
          </div>
        )}

        {data && (
          <div className="space-y-8">
            {data.zones.map((zone) => (
              <ZoneGroup key={zone.id} zone={zone} />
            ))}
          </div>
        )}
      </main>

      <footer className="border-t border-cardBorder py-6 text-center text-xs text-slate-500">
        <p>uptime-mon • Single-Binary Declarative Uptime Monitor</p>
      </footer>
    </div>
  );
}

export default App;
