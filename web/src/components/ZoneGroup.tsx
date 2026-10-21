import React from 'react';
import { ZoneView } from '../services/api';
import { EndpointCard } from './EndpointCard';

interface ZoneGroupProps {
  zone: ZoneView;
}

export const ZoneGroup: React.FC<ZoneGroupProps> = ({ zone }) => {
  const onlineCount = zone.endpoints.filter((ep) => ep.status === 'UP').length;
  const totalCount = zone.endpoints.length;

  return (
    <section className="bg-[#111113] border border-zinc-800/80 rounded-md overflow-hidden shadow-sm">
      {/* Zone Header */}
      <div className="px-4 py-2.5 bg-zinc-900/40 border-b border-zinc-800/60 flex items-center justify-between">
        <div className="flex items-center gap-2">
          <h2 className="text-sm font-semibold tracking-tight text-zinc-200">
            {zone.display_name}
          </h2>
          {zone.description && (
            <span className="text-xs text-zinc-500 hidden sm:inline">
              • {zone.description}
            </span>
          )}
        </div>

        <div className="flex items-center gap-2">
          <span className="text-xs font-mono tabular-nums text-zinc-400 bg-zinc-800/40 px-2 py-0.5 rounded border border-zinc-800/60">
            {onlineCount}/{totalCount} UP
          </span>
        </div>
      </div>

      {/* Endpoints List */}
      <div className="divide-y divide-zinc-800/60">
        {zone.endpoints.length === 0 ? (
          <div className="px-4 py-6 text-center text-xs font-mono text-zinc-500">
            No endpoints configured in this zone
          </div>
        ) : (
          zone.endpoints.map((ep) => (
            <EndpointCard key={ep.id} endpoint={ep} />
          ))
        )}
      </div>
    </section>
  );
};
