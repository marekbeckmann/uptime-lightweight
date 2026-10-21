import React, { useEffect, useState } from 'react';
import { Wrench, Clock, Layers, Server } from 'lucide-react';
import { OneOffMaintenanceView } from '../services/api';

interface MaintenanceBannerBarProps {
  maintenances: OneOffMaintenanceView[];
}

export const MaintenanceBannerBar: React.FC<MaintenanceBannerBarProps> = ({ maintenances }) => {
  // 1-second client-side reactive timestamp so expired maintenances disappear immediately
  const [now, setNow] = useState(() => Date.now());

  useEffect(() => {
    const timer = setInterval(() => {
      setNow(Date.now());
    }, 1000);
    return () => clearInterval(timer);
  }, []);

  // Filter only items where end_at is in the future
  const visibleItems = (maintenances || []).filter((m) => {
    const endMs = new Date(m.end_at).getTime();
    return endMs > now;
  });

  if (visibleItems.length === 0) {
    return null;
  }

  const formatDateTime = (isoStr: string) => {
    try {
      const d = new Date(isoStr);
      return d.toLocaleString(undefined, {
        month: 'short',
        day: 'numeric',
        hour: '2-digit',
        minute: '2-digit',
      });
    } catch {
      return isoStr;
    }
  };

  return (
    <aside aria-label="Maintenance Announcements" className="w-full flex flex-col gap-2">
      {visibleItems.map((m) => {
        const startMs = new Date(m.start_at).getTime();
        const endMs = new Date(m.end_at).getTime();
        const isCurrentlyActive = now >= startMs && now < endMs;

        return (
          <div
            key={m.id}
            className="w-full border-b border-amber-500/30 bg-amber-950/40 text-amber-100 px-4 py-3 shadow-sm flex flex-col md:flex-row md:items-center justify-between gap-3 text-sm backdrop-blur-sm"
          >
            {/* Left: Icon, Badge, Title, Message */}
            <div className="flex items-start md:items-center gap-3">
              <div className="p-1.5 rounded-lg bg-amber-500/20 text-amber-400 shrink-0 mt-0.5 md:mt-0">
                <Wrench className="w-4 h-4" />
              </div>

              <div className="flex flex-col sm:flex-row sm:items-center gap-2">
                <span
                  className={`px-2 py-0.5 text-xs font-semibold rounded uppercase tracking-wider shrink-0 ${
                    isCurrentlyActive
                      ? 'bg-amber-500 text-slate-950 font-bold animate-pulse'
                      : 'bg-amber-500/20 text-amber-300 border border-amber-500/40'
                  }`}
                >
                  {isCurrentlyActive ? 'Active Maintenance' : 'Upcoming Maintenance'}
                </span>

                <span className="font-bold text-white text-sm">{m.title || m.id}</span>
                {m.message && (
                  <span className="text-amber-200/90 text-xs sm:text-sm sm:before:content-['•'] sm:before:mr-2 sm:before:text-amber-500">
                    {m.message}
                  </span>
                )}
              </div>
            </div>

            {/* Right: Timing & Scope */}
            <div className="flex flex-wrap items-center gap-3 text-xs text-amber-300 shrink-0">
              <div className="flex items-center gap-1.5 bg-background/50 px-2.5 py-1 rounded border border-amber-500/20">
                <Clock className="w-3.5 h-3.5 text-amber-400" />
                <span>
                  {formatDateTime(m.start_at)} → {formatDateTime(m.end_at)}
                </span>
              </div>

              {/* Affected Zones */}
              {m.affected_zones && m.affected_zones.length > 0 && (
                <div className="flex items-center gap-1 bg-background/50 px-2.5 py-1 rounded border border-amber-500/20">
                  <Layers className="w-3 h-3 text-amber-400" />
                  <span>{m.affected_zones.map((z) => z.display_name).join(', ')}</span>
                </div>
              )}

              {/* Affected Hosts */}
              <div className="flex items-center gap-1 bg-background/50 px-2.5 py-1 rounded border border-amber-500/20">
                <Server className="w-3 h-3 text-amber-400" />
                <span>
                  {m.all_hosts
                    ? 'All hosts'
                    : m.affected_hosts && m.affected_hosts.length > 0
                    ? m.affected_hosts.map((h) => h.display_name).join(', ')
                    : 'All hosts'}
                </span>
              </div>
            </div>
          </div>
        );
      })}
    </aside>
  );
};
