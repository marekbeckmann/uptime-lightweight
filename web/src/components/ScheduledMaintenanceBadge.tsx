import React, { useState, useEffect, useRef } from 'react';
import { Calendar, ChevronDown, Clock, Layers, Server } from 'lucide-react';
import { ScheduledMaintenanceView } from '../services/api';

interface ScheduledMaintenanceBadgeProps {
  count: number;
  maintenances: ScheduledMaintenanceView[];
}

export const ScheduledMaintenanceBadge: React.FC<ScheduledMaintenanceBadgeProps> = ({
  count,
  maintenances = [],
}) => {
  const [isOpen, setIsOpen] = useState(false);
  const popoverRef = useRef<HTMLDivElement>(null);

  // Close on Escape key
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        setIsOpen(false);
      }
    };
    if (isOpen) {
      document.addEventListener('keydown', handleKeyDown);
    }
    return () => {
      document.removeEventListener('keydown', handleKeyDown);
    };
  }, [isOpen]);

  // Close on click outside
  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (popoverRef.current && !popoverRef.current.contains(e.target as Node)) {
        setIsOpen(false);
      }
    };
    if (isOpen) {
      document.addEventListener('mousedown', handleClickOutside);
    }
    return () => {
      document.removeEventListener('mousedown', handleClickOutside);
    };
  }, [isOpen]);

  const formatNextRun = (isoStr: string) => {
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
    <div className="relative inline-block" ref={popoverRef}>
      {/* Interactive Header Badge Button */}
      <button
        type="button"
        onClick={() => setIsOpen(!isOpen)}
        aria-expanded={isOpen}
        aria-haspopup="dialog"
        className={`bg-background/80 hover:bg-slate-800/80 border transition-all rounded-lg px-4 py-2 flex flex-col items-center justify-center min-w-[90px] focus:outline-none focus:ring-2 focus:ring-amber-500/50 ${
          isOpen ? 'border-amber-500/60 ring-2 ring-amber-500/20' : 'border-cardBorder'
        }`}
      >
        <div className="text-xs text-slate-400 flex items-center justify-center gap-1.5">
          <Calendar className="w-3.5 h-3.5 text-amber-400" />
          <span>Scheduled</span>
          <ChevronDown
            className={`w-3 h-3 text-slate-400 transition-transform duration-200 ${
              isOpen ? 'rotate-180 text-amber-400' : ''
            }`}
          />
        </div>
        <div className="text-lg font-bold text-amber-400">{count}</div>
      </button>

      {/* Popover Quick List Drawer */}
      {isOpen && (
        <div
          role="dialog"
          aria-label="Scheduled Maintenances"
          className="absolute right-0 mt-2 w-80 sm:w-96 bg-slate-900 border border-slate-700 shadow-2xl rounded-xl p-4 z-50 text-slate-100 divide-y divide-slate-800"
        >
          <div className="flex items-center justify-between pb-3">
            <div className="flex items-center gap-2">
              <Calendar className="w-4 h-4 text-amber-400" />
              <h3 className="font-semibold text-sm text-white">Scheduled Maintenances</h3>
            </div>
            <span className="text-xs text-slate-400 bg-slate-800 px-2 py-0.5 rounded font-mono">
              {count} {count === 1 ? 'rule' : 'rules'}
            </span>
          </div>

          <div className="py-2 max-h-80 overflow-y-auto space-y-3.5 pr-1">
            {maintenances.length === 0 ? (
              <div className="text-center py-6 text-xs text-slate-500">
                No scheduled maintenances configured
              </div>
            ) : (
              maintenances.map((sm) => (
                <div key={sm.id} className="pt-2 first:pt-0 space-y-1.5 text-xs">
                  <div className="flex items-center justify-between gap-2">
                    <span className="font-bold text-slate-200 text-sm">{sm.title || sm.id}</span>
                    <span className="px-1.5 py-0.5 rounded bg-amber-500/10 text-amber-300 border border-amber-500/20 font-mono text-[10px]">
                      {sm.schedule}
                    </span>
                  </div>

                  {sm.message && <p className="text-slate-400 text-xs">{sm.message}</p>}

                  <div className="bg-slate-950/60 p-2 rounded-md border border-slate-800/80 space-y-1 text-slate-300">
                    <div className="flex items-center gap-1.5 text-slate-400">
                      <Clock className="w-3 h-3 text-amber-400 shrink-0" />
                      <span>Next: <strong className="text-slate-200">{formatNextRun(sm.next_run_at)}</strong> ({sm.duration})</span>
                    </div>

                    {/* Affected Zones */}
                    {sm.affected_zones && sm.affected_zones.length > 0 && (
                      <div className="flex items-center gap-1.5 text-slate-400">
                        <Layers className="w-3 h-3 text-amber-400 shrink-0" />
                        <span>Zones: <span className="text-slate-200">{sm.affected_zones.map((z) => z.display_name).join(', ')}</span></span>
                      </div>
                    )}

                    {/* Affected Hosts */}
                    <div className="flex items-center gap-1.5 text-slate-400">
                      <Server className="w-3 h-3 text-amber-400 shrink-0" />
                      <span>
                        Hosts:{' '}
                        <span className="text-slate-200">
                          {sm.all_hosts
                            ? 'All hosts'
                            : sm.affected_hosts && sm.affected_hosts.length > 0
                            ? sm.affected_hosts.map((h) => h.display_name).join(', ')
                            : 'All hosts'}
                        </span>
                      </span>
                    </div>
                  </div>
                </div>
              ))
            )}
          </div>
        </div>
      )}
    </div>
  );
};
