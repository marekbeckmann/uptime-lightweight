import React, { useEffect, useState } from 'react';
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
    <aside aria-label="Maintenance Announcements" className="w-full flex flex-col">
      {visibleItems.map((m) => {
        const startMs = new Date(m.start_at).getTime();
        const endMs = new Date(m.end_at).getTime();
        const isCurrentlyActive = now >= startMs && now < endMs;

        return (
          <div
            key={m.id}
            className={`w-full px-4 sm:px-6 py-2 flex flex-col sm:flex-row sm:items-center justify-between gap-2 text-xs font-mono border-b ${
              isCurrentlyActive
                ? 'border-amber-500/20 bg-amber-500/[0.04] text-amber-200'
                : 'border-zinc-800 bg-zinc-900/40 text-zinc-300'
            }`}
          >
            {/* Left: Badge, Title, Message */}
            <div className="flex items-center gap-2.5 flex-wrap">
              <span
                className={`px-1.5 py-0.5 text-[10px] font-semibold rounded uppercase tracking-wider ${
                  isCurrentlyActive
                    ? 'bg-amber-500/20 text-amber-300 border border-amber-500/30'
                    : 'bg-zinc-800 text-zinc-400 border border-zinc-700'
                }`}
              >
                {isCurrentlyActive ? 'ACTIVE' : 'UPCOMING'}
              </span>

              <span className="font-semibold text-zinc-100">{m.title || m.id}</span>

              {m.message && (
                <span className="text-zinc-400 text-xs sm:before:content-['•'] sm:before:mr-2 sm:before:text-zinc-600">
                  {m.message}
                </span>
              )}
            </div>

            {/* Right: Timing & Scope */}
            <div className="flex items-center gap-3 text-xs text-zinc-400 flex-wrap tabular-nums">
              <span>
                {formatDateTime(m.start_at)} → {formatDateTime(m.end_at)}
              </span>

              {m.affected_zones && m.affected_zones.length > 0 && (
                <span className="text-zinc-500">
                  [{m.affected_zones.map((z) => z.display_name).join(', ')}]
                </span>
              )}

              <span className="text-zinc-500">
                {m.all_hosts
                  ? 'all hosts'
                  : m.affected_hosts && m.affected_hosts.length > 0
                  ? m.affected_hosts.map((h) => h.display_name).join(', ')
                  : 'all hosts'}
              </span>
            </div>
          </div>
        );
      })}
    </aside>
  );
};
