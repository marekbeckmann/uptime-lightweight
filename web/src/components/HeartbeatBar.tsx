import React, { useState } from 'react';
import { HeartbeatItem } from '../services/api';

interface HeartbeatBarProps {
  heartbeats: HeartbeatItem[];
}

export const HeartbeatBar: React.FC<HeartbeatBarProps> = ({ heartbeats }) => {
  const [hoveredIdx, setHoveredIdx] = useState<number | null>(null);

  // Exactly 45 slots
  const totalSlots = 45;
  const items = heartbeats.slice(-totalSlots);
  const emptySlots = Math.max(0, totalSlots - items.length);

  return (
    <div
      className="flex items-center gap-[2px] py-1 relative"
      onMouseLeave={() => setHoveredIdx(null)}
    >
      {/* Ghost slots for newly started endpoints */}
      {Array.from({ length: emptySlots }).map((_, i) => (
        <div
          key={`empty-${i}`}
          className="h-4 w-[3px] rounded-[1px] bg-zinc-800/80 shrink-0"
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
          color = 'bg-zinc-600';
        }

        const dateStr = new Date(hb.timestamp).toLocaleTimeString();
        const isHovered = hoveredIdx === i;

        return (
          <div
            key={hb.id || i}
            onMouseEnter={() => setHoveredIdx(i)}
            className={`h-4 w-[3px] rounded-[1px] transition-colors duration-75 cursor-pointer shrink-0 relative ${color}`}
          >
            {isHovered && (
              <div className="pointer-events-none absolute bottom-full mb-1.5 left-1/2 -translate-x-1/2 bg-zinc-900 border border-zinc-700/80 text-[10px] font-mono text-zinc-200 px-1.5 py-0.5 rounded shadow-lg whitespace-nowrap z-30 tabular-nums">
                <span
                  className={
                    hb.status === 'UP'
                      ? 'text-emerald-400'
                      : hb.status === 'DOWN'
                      ? 'text-rose-400'
                      : 'text-amber-400'
                  }
                >
                  {hb.status}
                </span>{' '}
                · {hb.latency_ms}ms · {dateStr}
                {hb.error ? ` · ${hb.error}` : ''}
              </div>
            )}
          </div>
        );
      })}
    </div>
  );
};
