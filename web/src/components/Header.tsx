import React from 'react';
import { DashboardData } from '../services/api';
import { ScheduledMaintenanceBadge } from './ScheduledMaintenanceBadge';

interface HeaderProps {
  data: DashboardData | null;
  isStreaming?: boolean;
}

export const Header: React.FC<HeaderProps> = ({ data, isStreaming = true }) => {
  const counts = data?.counts || {
    hosts_up: 0,
    hosts_down: 0,
    in_maintenance: 0,
    scheduled_maintenance_count: 0,
    uptime_percentage: 100,
  };

  const status = data?.system_status || 'PENDING';

  return (
    <header className="h-14 sticky top-0 z-40 w-full border-b border-zinc-800/80 bg-[#09090b]/80 backdrop-blur-md px-4 sm:px-6">
      <div className="max-w-7xl mx-auto flex items-center justify-between h-full">
        {/* Left: Brand & Status Dot */}
        <div className="flex items-center gap-3">
          <div className="flex items-center gap-2">
            <span className="relative flex h-2 w-2">
              {isStreaming ? (
                <>
                  <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75"></span>
                  <span className="relative inline-flex rounded-full h-2 w-2 bg-emerald-500"></span>
                </>
              ) : (
                <span className="relative inline-flex rounded-full h-2 w-2 bg-amber-500"></span>
              )}
            </span>
            <span className="font-mono text-sm font-semibold tracking-tight text-zinc-100">
              uptime-mon
            </span>
          </div>

          <span
            className={`hidden md:inline-flex items-center px-2 py-0.5 text-[11px] font-mono rounded border ${
              status === 'UP'
                ? 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20'
                : status === 'DOWN'
                ? 'bg-rose-500/10 text-rose-400 border-rose-500/20'
                : 'bg-amber-500/10 text-amber-400 border-amber-500/20'
            }`}
          >
            {status === 'UP' ? 'OPERATIONAL' : status === 'DOWN' ? 'DISRUPTED' : 'MAINTENANCE'}
          </span>
        </div>

        {/* Right: Inline Segmented Telemetry Strip */}
        <div className="inline-flex items-center divide-x divide-zinc-800 border border-zinc-800/80 rounded-md bg-zinc-900/60 text-xs font-mono">
          {/* UP Count */}
          <div className="px-3 py-1.5 flex items-center gap-1.5 text-zinc-400">
            <span className="text-zinc-500 text-[10px] uppercase font-semibold">UP</span>
            <span className="text-emerald-400 font-semibold tabular-nums">{counts.hosts_up}</span>
          </div>

          {/* DOWN Count */}
          <div className="px-3 py-1.5 flex items-center gap-1.5 text-zinc-400">
            <span className="text-zinc-500 text-[10px] uppercase font-semibold">DOWN</span>
            <span
              className={`font-semibold tabular-nums ${
                counts.hosts_down > 0 ? 'text-rose-400' : 'text-zinc-500'
              }`}
            >
              {counts.hosts_down}
            </span>
          </div>

          {/* Aggregate Uptime Percentage */}
          <div className="hidden sm:flex items-center gap-1.5 px-3 py-1.5 text-zinc-400">
            <span className="text-zinc-500 text-[10px] uppercase font-semibold">UPTIME</span>
            <span className="text-zinc-200 tabular-nums">
              {counts.uptime_percentage.toFixed(2)}%
            </span>
          </div>

          {/* Scheduled Maintenance Popover Trigger */}
          <ScheduledMaintenanceBadge
            count={counts.scheduled_maintenance_count}
            maintenances={data?.scheduled_maintenances || []}
          />
        </div>
      </div>
    </header>
  );
};
