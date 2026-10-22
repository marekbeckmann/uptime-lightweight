import React, { useState, useEffect } from 'react';
import { Layers, ChevronDown } from 'lucide-react';
import { ZoneView, UIConfig } from '../services/api';
import { EndpointCard } from './EndpointCard';

interface ZoneGroupProps {
  zone: ZoneView;
  uiConfig?: UIConfig;
}

export const ZoneGroup: React.FC<ZoneGroupProps> = ({ zone, uiConfig }) => {
  const disableCollapse = uiConfig?.disable_collapse ?? false;
  const initiallyCollapsed = !disableCollapse && (uiConfig?.initially_collapsed ?? false);

  const storageKey = `uptime-mon:zone-collapsed:${zone.id}`;

  const [isCollapsed, setIsCollapsed] = useState<boolean>(() => {
    if (disableCollapse) return false;
    try {
      const saved = localStorage.getItem(storageKey);
      if (saved !== null) {
        return saved === 'true';
      }
    } catch {
      // Ignore localStorage access errors
    }
    return initiallyCollapsed;
  });

  // Keep state synchronized if disableCollapse changes via hot-reload
  useEffect(() => {
    if (disableCollapse) {
      setIsCollapsed(false);
    }
  }, [disableCollapse]);

  const toggleCollapse = () => {
    if (disableCollapse) return;
    setIsCollapsed((prev) => {
      const next = !prev;
      try {
        localStorage.setItem(storageKey, String(next));
      } catch {
        // Ignore localStorage write errors
      }
      return next;
    });
  };

  // Compute zone endpoint health summary
  const downCount = zone.endpoints.filter((ep) => ep.status === 'DOWN').length;
  const maintCount = zone.endpoints.filter((ep) => ep.status === 'MAINTENANCE').length;
  const upCount = zone.endpoints.filter((ep) => ep.status === 'UP').length;

  return (
    <section className="space-y-3">
      {/* Zone Header */}
      <div
        onClick={toggleCollapse}
        onKeyDown={(e) => {
          if (!disableCollapse && (e.key === 'Enter' || e.key === ' ')) {
            e.preventDefault();
            toggleCollapse();
          }
        }}
        role={disableCollapse ? undefined : 'button'}
        tabIndex={disableCollapse ? undefined : 0}
        aria-expanded={disableCollapse ? undefined : !isCollapsed}
        aria-controls={`zone-endpoints-${zone.id}`}
        className={`flex items-center justify-between pb-2 border-b border-cardBorder transition-colors select-none ${
          disableCollapse
            ? 'cursor-default'
            : 'cursor-pointer hover:border-slate-600 focus:outline-none focus-visible:ring-1 focus-visible:ring-emerald-400'
        }`}
      >
        <div className="flex items-center gap-2">
          {!disableCollapse && (
            <ChevronDown
              className={`w-4 h-4 text-slate-400 transition-transform duration-200 ${
                isCollapsed ? '-rotate-90' : 'rotate-0'
              }`}
            />
          )}
          <Layers className="w-4 h-4 text-emerald-400" />
          <h2 className="text-lg font-bold text-white tracking-tight">{zone.display_name}</h2>
          {zone.description && (
            <span className="text-xs text-slate-400 hidden sm:inline ml-2">• {zone.description}</span>
          )}
        </div>

        <div className="flex items-center gap-2">
          {/* Incident / Status Badges */}
          {downCount > 0 && (
            <span className="text-xs font-semibold px-2 py-0.5 rounded bg-rose-500/20 border border-rose-500/30 text-rose-300 animate-pulse">
              {downCount} {downCount === 1 ? 'down' : 'down'}
            </span>
          )}
          {downCount === 0 && maintCount > 0 && (
            <span className="text-xs font-semibold px-2 py-0.5 rounded bg-amber-500/20 border border-amber-500/30 text-amber-300">
              {maintCount} maintenance
            </span>
          )}
          {isCollapsed && downCount === 0 && maintCount === 0 && upCount > 0 && (
            <span className="text-xs font-semibold px-2 py-0.5 rounded bg-emerald-500/10 border border-emerald-500/20 text-emerald-400">
              all up
            </span>
          )}

          <span className="text-xs font-semibold px-2 py-0.5 rounded bg-cardBorder text-slate-300">
            {zone.endpoints.length} {zone.endpoints.length === 1 ? 'monitor' : 'monitors'}
          </span>
        </div>
      </div>

      {/* Endpoints List */}
      {!isCollapsed && (
        <div id={`zone-endpoints-${zone.id}`} className="grid grid-cols-1 gap-3 pt-1">
          {zone.endpoints.map((ep) => (
            <EndpointCard key={ep.id} endpoint={ep} />
          ))}
        </div>
      )}
    </section>
  );
};
