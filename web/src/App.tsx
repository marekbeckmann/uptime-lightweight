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
    const unsubscribe = subscribeToEvents(
      (heartbeat) => {
        setData((prev) => {
          if (!prev) return prev;

          const updatedZones = prev.zones.map((zone) => {
            const updatedEndpoints = zone.endpoints.map((ep) => {
              if (ep.id !== heartbeat.endpoint_id) return ep;

              const probeStatus = (heartbeat.probe_status || heartbeat.status) as 'UP' | 'DOWN' | 'MAINTENANCE' | 'PENDING';
              const endpointStatus = heartbeat.status as 'UP' | 'DOWN' | 'MAINTENANCE' | 'PENDING';

              const updatedHeartbeats = [
                ...ep.heartbeats,
                {
                  id: Date.now(),
                  timestamp: heartbeat.timestamp,
                  status: probeStatus,
                  latency_ms: heartbeat.latency_ms,
                  error: heartbeat.error,
                },
              ];

              return {
                ...ep,
                status: endpointStatus,
                latency_ms: heartbeat.latency_ms,
                heartbeats: updatedHeartbeats.slice(-60),
              };
            });

            return {
              ...zone,
              endpoints: updatedEndpoints,
            };
          });

          // Recalculate summary status and counts across all zones and endpoints
          let totalUp = 0;
          let totalDown = 0;
          let totalMaint = 0;

          for (const zone of updatedZones) {
            for (const ep of zone.endpoints) {
              if (ep.status === 'UP') totalUp++;
              else if (ep.status === 'DOWN') totalDown++;
              else if (ep.status === 'MAINTENANCE') totalMaint++;
            }
          }

          let sysStatus: 'UP' | 'DOWN' | 'MAINTENANCE' | 'PENDING' = 'UP';
          if (totalDown > 0) {
            sysStatus = 'DOWN';
          } else if (totalMaint > 0 && totalUp === 0) {
            sysStatus = 'MAINTENANCE';
          } else if (totalUp === 0 && totalDown === 0) {
            sysStatus = 'PENDING';
          }

          return {
            ...prev,
            system_status: sysStatus,
            counts: {
              ...prev.counts,
              hosts_up: totalUp,
              hosts_down: totalDown,
              in_maintenance: totalMaint,
            },
            zones: updatedZones,
          };
        });
      },
      () => {
        // Reload full dashboard data on config reload
        loadData();
      }
    );

    return () => {
      unsubscribe();
    };
  }, []);

  // Synchronize document <title> and favicon with branding metadata
  useEffect(() => {
    if (!data?.branding) return;

    if (data.branding.tab_title) {
      document.title = data.branding.tab_title;
    } else if (data.branding.title) {
      document.title = `${data.branding.title} | Status Dashboard`;
    }

    if (data.branding.favicon) {
      let link = document.querySelector("link[rel*='icon']") as HTMLLinkElement | null;
      if (!link) {
        link = document.createElement('link');
        link.rel = 'icon';
        document.head.appendChild(link);
      }
      link.href = data.branding.favicon;
    }
  }, [data?.branding]);

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
              <ZoneGroup key={zone.id} zone={zone} uiConfig={data.ui} />
            ))}
          </div>
        )}
      </main>

      <footer className="border-t border-cardBorder py-6 text-center text-xs text-slate-500">
        <p>{data?.branding?.footer || 'uptime-mon • Single-Binary Declarative Uptime Monitor'}</p>
      </footer>
    </div>
  );
}

export default App;
