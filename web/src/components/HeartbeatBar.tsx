import React from 'react';
import { HeartbeatItem } from '../services/api';

interface HeartbeatBarProps {
  heartbeats: HeartbeatItem[];
}

export const HeartbeatBar: React.FC<HeartbeatBarProps> = ({ heartbeats }) => {
  // Ensure we display up to 60 slots
  const totalSlots = 50;
  const items = heartbeats.slice(-totalSlots);
  const emptySlots = Math.max(0, totalSlots - items.length);

  return (
    <div className="flex items-center gap-[3px] py-1">
      {/* Ghost slots if newly started */}
      {Array.from({ length: emptySlots }).map((_, i) => (
        <div
          key={`empty-${i}`}
          className="h-7 flex-1 min-w-[3px] max-w-[6px] rounded-[1px] bg-slate-800/60"
          title="No data"
        />
      ))}

      {/* Actual check results */}
      {items.map((hb, i) => {
        let color = 'bg-emerald-500 hover:bg-emerald-400';
        if (hb.status === 'DOWN') {
          color = 'bg-rose-500 hover:bg-rose-400';
        } else if (hb.status === 'MAINTENANCE') {
          color = 'bg-amber-500 hover:bg-amber-400';
        } else if (hb.status === 'PENDING') {
          color = 'bg-slate-600';
        }

        const dateStr = new Date(hb.timestamp).toLocaleTimeString();
        const tooltip = `${hb.status} (${hb.latency_ms}ms) at ${dateStr}${hb.error ? ` - ${hb.error}` : ''}`;

        return (
          <div
            key={hb.id || i}
            className={`h-7 flex-1 min-w-[3px] max-w-[6px] rounded-[1px] transition-all duration-150 cursor-pointer ${color}`}
            title={tooltip}
          />
        );
      })}
    </div>
  );
};
