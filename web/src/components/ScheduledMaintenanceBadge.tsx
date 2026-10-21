import React, { useState, useEffect, useRef } from 'react';
import { ChevronDown } from 'lucide-react';
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
      {/* Interactive Trigger Button inside Telemetry Strip */}
      <button
        type="button"
        onClick={() => setIsOpen(!isOpen)}
        aria-expanded={isOpen}
        aria-haspopup="dialog"
        className={`px-3 py-1.5 flex items-center gap-1.5 text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800/40 cursor-pointer transition-colors focus:outline-none ${
          isOpen ? 'bg-zinc-800/60 text-zinc-100' : ''
        }`}
      >
        <span className="text-zinc-500 text-[10px] uppercase font-semibold">SCHEDULED</span>
        <span className="text-amber-400 font-semibold tabular-nums">{count}</span>
        <ChevronDown
          className={`w-3 h-3 text-zinc-500 transition-transform duration-150 ${
            isOpen ? 'rotate-180 text-amber-400' : ''
          }`}
        />
      </button>

      {/* Popover Quick List */}
      {isOpen && (
        <div
          role="dialog"
          aria-label="Scheduled Maintenances"
          className="absolute right-0 top-full mt-2 w-80 sm:w-96 bg-[#111113] border border-zinc-800/80 rounded-md shadow-2xl p-4 z-50 text-zinc-100 divide-y divide-zinc-800/60"
        >
          <div className="flex items-center justify-between pb-2.5">
            <h3 className="font-semibold text-xs text-zinc-200 uppercase tracking-wider">
              Scheduled Maintenances
            </h3>
            <span className="text-[11px] text-zinc-400 bg-zinc-800/60 px-2 py-0.5 rounded font-mono tabular-nums">
              {count} {count === 1 ? 'rule' : 'rules'}
            </span>
          </div>

          <div className="py-2 max-h-80 overflow-y-auto space-y-3 pr-1">
            {maintenances.length === 0 ? (
              <div className="text-center py-6 text-xs text-zinc-500 font-mono">
                No scheduled maintenances configured
              </div>
            ) : (
              maintenances.map((sm) => (
                <div key={sm.id} className="pt-2.5 first:pt-0 space-y-1.5 text-xs font-mono">
                  <div className="flex items-center justify-between gap-2">
                    <span className="font-medium text-zinc-200">{sm.title || sm.id}</span>
                    <span className="px-1.5 py-0.5 rounded bg-zinc-800 text-zinc-300 border border-zinc-700/60 text-[10px] tabular-nums">
                      {sm.schedule}
                    </span>
                  </div>

                  {sm.message && (
                    <p className="text-zinc-400 text-[11px] font-sans">{sm.message}</p>
                  )}

                  <div className="bg-zinc-950/60 p-2 rounded border border-zinc-800/60 space-y-1 text-[11px] text-zinc-400 tabular-nums">
                    <div>
                      Next: <strong className="text-zinc-200">{formatNextRun(sm.next_run_at)}</strong> ({sm.duration})
                    </div>

                    {sm.affected_zones && sm.affected_zones.length > 0 && (
                      <div>
                        Zones: <span className="text-zinc-300">{sm.affected_zones.map((z) => z.display_name).join(', ')}</span>
                      </div>
                    )}

                    <div>
                      Hosts:{' '}
                      <span className="text-zinc-300">
                        {sm.all_hosts
                          ? 'All hosts'
                          : sm.affected_hosts && sm.affected_hosts.length > 0
                          ? sm.affected_hosts.map((h) => h.display_name).join(', ')
                          : 'All hosts'}
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
