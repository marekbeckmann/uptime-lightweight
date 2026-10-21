import React from 'react';
import { Layers } from 'lucide-react';
import { ZoneView } from '../services/api';
import { EndpointCard } from './EndpointCard';

interface ZoneGroupProps {
  zone: ZoneView;
}

export const ZoneGroup: React.FC<ZoneGroupProps> = ({ zone }) => {
  return (
    <section className="space-y-3">
      {/* Zone Header */}
      <div className="flex items-center justify-between pb-2 border-b border-cardBorder">
        <div className="flex items-center gap-2">
          <Layers className="w-4 h-4 text-emerald-400" />
          <h2 className="text-lg font-bold text-white tracking-tight">{zone.display_name}</h2>
          {zone.description && (
            <span className="text-xs text-slate-400 hidden sm:inline ml-2">• {zone.description}</span>
          )}
        </div>
        <span className="text-xs font-semibold px-2 py-0.5 rounded bg-cardBorder text-slate-300">
          {zone.endpoints.length} {zone.endpoints.length === 1 ? 'monitor' : 'monitors'}
        </span>
      </div>

      {/* Endpoints List */}
      <div className="grid grid-cols-1 gap-3">
        {zone.endpoints.map((ep) => (
          <EndpointCard key={ep.id} endpoint={ep} />
        ))}
      </div>
    </section>
  );
};
