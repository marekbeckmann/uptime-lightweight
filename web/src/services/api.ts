export interface HeartbeatItem {
  id: number;
  timestamp: string;
  status: 'UP' | 'DOWN' | 'MAINTENANCE' | 'PENDING';
  latency_ms: number;
  error?: string;
}

export interface EndpointView {
  id: string;
  display_name: string;
  zone: string;
  icon?: string;
  status: 'UP' | 'DOWN' | 'MAINTENANCE' | 'PENDING';
  latency_ms: number;
  uptime_24h: number;
  show_target: boolean;
  target?: string;
  heartbeats: HeartbeatItem[];
}

export interface ZoneView {
  id: string;
  display_name: string;
  description?: string;
  order: number;
  icon?: string;
  endpoints: EndpointView[];
}

export interface MaintenanceTargetRef {
  id: string;
  display_name: string;
}

export interface OneOffMaintenanceView {
  id: string;
  title: string;
  message: string;
  start_at: string;
  end_at: string;
  is_active: boolean;
  affected_zones: MaintenanceTargetRef[];
  affected_hosts: MaintenanceTargetRef[];
  all_hosts: boolean;
}

export interface ScheduledMaintenanceView {
  id: string;
  title: string;
  message: string;
  schedule: string;
  duration: string;
  start_at?: string;
  end_at?: string;
  next_run_at: string;
  is_currently_active: boolean;
  affected_zones: MaintenanceTargetRef[];
  affected_hosts: MaintenanceTargetRef[];
  all_hosts: boolean;
}

// Backward-compatible banner definition
export interface MaintenanceBanner {
  id: string;
  message: string;
  start_time: string;
  end_time: string;
  is_active: boolean;
}

export interface BrandingView {
  title: string;
  subtitle: string;
  footer: string;
  icon: string;
  favicon: string;
  tab_title: string;
}

export interface UIConfig {
  disable_collapse: boolean;
  initially_collapsed: boolean;
}

export interface DashboardData {
  system_status: 'UP' | 'DOWN' | 'MAINTENANCE' | 'PENDING';
  counts: {
    hosts_up: number;
    hosts_down: number;
    in_maintenance: number;
    scheduled_maintenance_count: number;
    uptime_percentage: number;
  };
  branding: BrandingView;
  ui?: UIConfig;
  one_off_maintenances: OneOffMaintenanceView[];
  scheduled_maintenances: ScheduledMaintenanceView[];
  maintenance_banners: MaintenanceBanner[];
  zones: ZoneView[];
}

export async function fetchDashboard(): Promise<DashboardData> {
  const res = await fetch('/api/v1/dashboard');
  if (!res.ok) {
    throw new Error(`Failed to fetch dashboard: ${res.statusText}`);
  }
  return res.json();
}

export function subscribeToEvents(
  onHeartbeat: (data: { endpoint_id: string; status: string; latency_ms: number; timestamp: string }) => void
): () => void {
  const eventSource = new EventSource('/api/v1/events');

  eventSource.addEventListener('heartbeat', (e) => {
    try {
      const data = JSON.parse(e.data);
      onHeartbeat(data);
    } catch (err) {
      console.error('Error parsing heartbeat event:', err);
    }
  });

  return () => {
    eventSource.close();
  };
}
