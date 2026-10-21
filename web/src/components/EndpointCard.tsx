import React, { useState } from 'react';
import { Server } from 'lucide-react';
import { EndpointView } from '../services/api';
import { HeartbeatBar } from './HeartbeatBar';

interface EndpointCardProps {
  endpoint: EndpointView;
}

export const EndpointCard: React.FC<EndpointCardProps> = ({ endpoint }) => {
  const [imgError, setImgError] = useState(false);

  // Icon URL resolver: if slug, use cdn.jsdelivr.net selfh.st/icons; if full URL, use as is
  let iconUrl = '';
  if (endpoint.icon) {
    if (endpoint.icon.startsWith('http://') || endpoint.icon.startsWith('https://')) {
      iconUrl = endpoint.icon;
    } else {
      iconUrl = `https://cdn.jsdelivr.net/gh/selfhst/icons/svg/${endpoint.icon}.svg`;
    }
  }

  const getStatusDot = () => {
    switch (endpoint.status) {
      case 'UP':
        return <span className="w-2 h-2 rounded-full bg-emerald-500 shrink-0" />;
      case 'DOWN':
        return <span className="w-2 h-2 rounded-full bg-rose-500 shrink-0" />;
      case 'MAINTENANCE':
        return <span className="w-2 h-2 rounded-full bg-amber-500 shrink-0" />;
      default:
        return <span className="w-2 h-2 rounded-full bg-zinc-600 shrink-0" />;
    }
  };

  const getStatusBadge = () => {
    switch (endpoint.status) {
      case 'UP':
        return (
          <span className="px-2 py-0.5 text-[10px] font-mono rounded border bg-emerald-500/10 text-emerald-400 border-emerald-500/20">
            UP
          </span>
        );
      case 'DOWN':
        return (
          <span className="px-2 py-0.5 text-[10px] font-mono rounded border bg-rose-500/10 text-rose-400 border-rose-500/20">
            DOWN
          </span>
        );
      case 'MAINTENANCE':
        return (
          <span className="px-2 py-0.5 text-[10px] font-mono rounded border bg-amber-500/10 text-amber-400 border-amber-500/20">
            MAINT
          </span>
        );
      default:
        return (
          <span className="px-2 py-0.5 text-[10px] font-mono rounded border bg-zinc-800 text-zinc-400 border-zinc-700">
            PENDING
          </span>
        );
    }
  };

  return (
    <div className="px-4 py-2.5 flex items-center justify-between gap-3 sm:gap-4 hover:bg-zinc-800/30 transition-colors duration-100">
      {/* Left: Status Dot, Icon, Title, and Target */}
      <div className="flex items-center gap-2.5 min-w-0 flex-1 sm:flex-initial sm:w-64 md:w-72 lg:w-80">
        {getStatusDot()}

        <div className="w-4 h-4 flex items-center justify-center shrink-0 text-zinc-400">
          {iconUrl && !imgError ? (
            <img
              src={iconUrl}
              alt={endpoint.display_name}
              className="w-4 h-4 object-contain"
              onError={() => setImgError(true)}
            />
          ) : (
            <Server className="w-3.5 h-3.5 text-zinc-400" />
          )}
        </div>

        <div className="min-w-0 flex items-center gap-2">
          <span className="font-medium text-sm text-zinc-200 truncate">
            {endpoint.display_name}
          </span>
          {endpoint.target && (
            <span className="font-mono text-xs text-zinc-500 truncate hidden xl:inline-block">
              {endpoint.target}
            </span>
          )}
        </div>
      </div>

      {/* Center: Heartbeat Bar */}
      <div className="hidden sm:flex flex-1 items-center justify-center max-w-xs md:max-w-md lg:max-w-lg px-2">
        <HeartbeatBar heartbeats={endpoint.heartbeats} />
      </div>

      {/* Right: Latency, Uptime %, and Status Badge */}
      <div className="flex items-center gap-3 sm:gap-4 shrink-0 text-right">
        {/* Latency */}
        <div className="w-14 text-right font-mono text-xs tabular-nums text-zinc-400">
          {endpoint.latency_ms > 0 ? `${endpoint.latency_ms}ms` : '—'}
        </div>

        {/* 24h Uptime */}
        <div className="w-14 text-right font-mono text-xs tabular-nums text-emerald-400 hidden xs:block">
          {endpoint.uptime_24h.toFixed(1)}%
        </div>

        {/* Status Badge */}
        <div className="w-14 flex justify-end shrink-0">
          {getStatusBadge()}
        </div>
      </div>
    </div>
  );
};
