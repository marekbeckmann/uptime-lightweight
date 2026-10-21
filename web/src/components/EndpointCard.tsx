import React, { useState } from 'react';
import { Globe, Server, CheckCircle2, XCircle, Wrench, Clock } from 'lucide-react';
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

  const getStatusBadge = () => {
    switch (endpoint.status) {
      case 'UP':
        return (
          <span className="flex items-center gap-1.5 px-2.5 py-1 text-xs font-semibold rounded-full bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">
            <CheckCircle2 className="w-3.5 h-3.5" />
            <span>UP</span>
          </span>
        );
      case 'DOWN':
        return (
          <span className="flex items-center gap-1.5 px-2.5 py-1 text-xs font-semibold rounded-full bg-rose-500/10 text-rose-400 border border-rose-500/20">
            <XCircle className="w-3.5 h-3.5" />
            <span>DOWN</span>
          </span>
        );
      case 'MAINTENANCE':
        return (
          <span className="flex items-center gap-1.5 px-2.5 py-1 text-xs font-semibold rounded-full bg-amber-500/10 text-amber-400 border border-amber-500/20">
            <Wrench className="w-3.5 h-3.5" />
            <span>MAINT</span>
          </span>
        );
      default:
        return (
          <span className="flex items-center gap-1.5 px-2.5 py-1 text-xs font-semibold rounded-full bg-slate-500/10 text-slate-400 border border-slate-500/20">
            <Clock className="w-3.5 h-3.5" />
            <span>PENDING</span>
          </span>
        );
    }
  };

  return (
    <div className="bg-card border border-cardBorder rounded-xl p-4 hover:border-slate-700 transition-colors shadow-sm">
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 mb-3">
        {/* Endpoint Icon & Title */}
        <div className="flex items-center gap-3">
          <div className="w-10 h-10 rounded-lg bg-background border border-cardBorder flex items-center justify-center p-2 text-slate-400 shrink-0">
            {iconUrl && !imgError ? (
              <img
                src={iconUrl}
                alt={endpoint.display_name}
                className="w-full h-full object-contain"
                onError={() => setImgError(true)}
              />
            ) : (
              <Server className="w-5 h-5 text-slate-400" />
            )}
          </div>

          <div>
            <div className="flex items-center gap-2">
              <h3 className="font-semibold text-white text-base leading-snug">{endpoint.display_name}</h3>
              {getStatusBadge()}
            </div>
            {endpoint.target && (
              <p className="text-xs text-slate-400 flex items-center gap-1 mt-0.5">
                <Globe className="w-3 h-3 text-slate-500" />
                <span className="font-mono text-slate-400">{endpoint.target}</span>
              </p>
            )}
          </div>
        </div>

        {/* Latency and Uptime Metrics */}
        <div className="flex items-center gap-4 text-right shrink-0">
          <div>
            <div className="text-xs text-slate-400">Response</div>
            <div className="text-sm font-semibold text-slate-200">
              {endpoint.latency_ms > 0 ? `${endpoint.latency_ms} ms` : '—'}
            </div>
          </div>
          <div className="border-l border-cardBorder pl-4">
            <div className="text-xs text-slate-400">24h Uptime</div>
            <div className="text-sm font-semibold text-emerald-400">
              {endpoint.uptime_24h.toFixed(1)}%
            </div>
          </div>
        </div>
      </div>

      {/* Heartbeat Bar */}
      <div className="mt-2 pt-2 border-t border-cardBorder/50">
        <HeartbeatBar heartbeats={endpoint.heartbeats} />
      </div>
    </div>
  );
};
