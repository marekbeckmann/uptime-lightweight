import React, { useState } from 'react';
import { Activity, CheckCircle2, AlertTriangle, Wrench, ShieldCheck } from 'lucide-react';
import { DashboardData } from '../services/api';
import { ScheduledMaintenanceBadge } from './ScheduledMaintenanceBadge';

interface HeaderProps {
  data: DashboardData | null;
}

export const Header: React.FC<HeaderProps> = ({ data }) => {
  const [iconError, setIconError] = useState(false);

  const counts = data?.counts || {
    hosts_up: 0,
    hosts_down: 0,
    in_maintenance: 0,
    scheduled_maintenance_count: 0,
    uptime_percentage: 100,
  };

  const branding = data?.branding || {
    title: 'uptime-mon',
    subtitle: 'Continuous GitOps Uptime & Protocol Monitoring',
    footer: 'uptime-mon • Single-Binary Declarative Uptime Monitor',
    icon: '',
    favicon: '',
    tab_title: 'uptime-mon | Status Dashboard',
  };

  const status = data?.system_status || 'PENDING';

  return (
    <header className="border-b border-cardBorder bg-card/60 backdrop-blur-md px-6 py-6 sticky top-0 z-40">
      <div className="max-w-7xl mx-auto flex flex-col md:flex-row md:items-center justify-between gap-4">
        <div className="flex items-center gap-3">
          <div className="w-11 h-11 rounded-xl bg-emerald-500/10 border border-emerald-500/20 text-emerald-400 flex items-center justify-center p-2 shrink-0">
            {branding.icon && !iconError ? (
              <img
                src={branding.icon}
                alt={branding.title}
                className="w-full h-full object-contain"
                onError={() => setIconError(true)}
              />
            ) : (
              <Activity className="w-6 h-6" />
            )}
          </div>
          <div>
            <div className="flex items-center gap-2">
              <h1 className="text-xl font-bold tracking-tight text-white">{branding.title}</h1>
              <span className={`px-2 py-0.5 text-xs font-semibold rounded-full border ${
                status === 'UP'
                  ? 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20'
                  : status === 'DOWN'
                  ? 'bg-rose-500/10 text-rose-400 border-rose-500/20'
                  : 'bg-amber-500/10 text-amber-400 border-amber-500/20'
              }`}>
                {status === 'UP' ? 'All Systems Operational' : status === 'DOWN' ? 'Service Disruption' : 'Maintenance'}
              </span>
            </div>
            <p className="text-xs text-slate-400 mt-0.5">{branding.subtitle}</p>
          </div>
        </div>

        {/* Global Summary Statistics */}
        <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-5 gap-3 text-center items-center">
          <div className="bg-background/80 border border-cardBorder rounded-lg px-4 py-2">
            <div className="text-xs text-slate-400 flex items-center justify-center gap-1">
              <CheckCircle2 className="w-3.5 h-3.5 text-emerald-400" />
              <span>Hosts Up</span>
            </div>
            <div className="text-lg font-bold text-emerald-400">{counts.hosts_up}</div>
          </div>

          <div className="bg-background/80 border border-cardBorder rounded-lg px-4 py-2">
            <div className="text-xs text-slate-400 flex items-center justify-center gap-1">
              <AlertTriangle className="w-3.5 h-3.5 text-rose-400" />
              <span>Hosts Down</span>
            </div>
            <div className={`text-lg font-bold ${counts.hosts_down > 0 ? 'text-rose-400' : 'text-slate-200'}`}>
              {counts.hosts_down}
            </div>
          </div>

          <div className="bg-background/80 border border-cardBorder rounded-lg px-4 py-2">
            <div className="text-xs text-slate-400 flex items-center justify-center gap-1">
              <Wrench className="w-3.5 h-3.5 text-amber-400" />
              <span>Maintenance</span>
            </div>
            <div className="text-lg font-bold text-amber-400">{counts.in_maintenance}</div>
          </div>

          <ScheduledMaintenanceBadge
            count={counts.scheduled_maintenance_count ?? (data?.scheduled_maintenances?.length || 0)}
            maintenances={data?.scheduled_maintenances || []}
          />

          <div className="bg-background/80 border border-cardBorder rounded-lg px-4 py-2">
            <div className="text-xs text-slate-400 flex items-center justify-center gap-1">
              <ShieldCheck className="w-3.5 h-3.5 text-emerald-400" />
              <span>24h Uptime</span>
            </div>
            <div className="text-lg font-bold text-white">{counts.uptime_percentage.toFixed(1)}%</div>
          </div>
        </div>
      </div>
    </header>
  );
};
