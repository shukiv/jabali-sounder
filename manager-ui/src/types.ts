export interface Server {
  id: string;
  name: string;
  base_url: string;
  token_id: string;
  scopes: string[];
  tags: string[];
  insecure_skip_verify: boolean;
  version: string;
  cert_expires_at?: string;
  status: "active" | "disabled" | "unreachable";
  credential_status: "valid" | "invalid" | "unknown";
  created_at: string;
  updated_at: string;
}

export interface DashboardEntry {
  id: string;
  name: string;
  base_url: string;
  status: "active" | "disabled" | "unreachable";
  credential_status: "valid" | "invalid" | "unknown";
  version: string;
  healthy: boolean;
}

export interface ListEnvelope<T> {
  data: T[];
  total: number;
  page: number;
  page_size: number;
}

export interface CheckResult {
  reachable: boolean;
  healthy: boolean;
  credential_valid: boolean;
  version: string;
  health_code: number;
  status_code: number;
  health_error?: string;
  status_error?: string;
}

export interface MonitorServerRef {
  id: string;
  name: string;
  base_url: string;
  status: "active" | "disabled" | "unreachable";
  credential_status: "valid" | "invalid" | "unknown";
  version: string;
}

export interface MonitorLiveEntry {
  server: MonitorServerRef;
  available: boolean;
  as_of?: string;
  cpu_percent?: number;
  ram_used_bytes?: number;
  ram_total_bytes?: number;
  ram_percent?: number;
  io_wait_percent?: number;
  io_read_bps?: number;
  io_write_bps?: number;
  load1?: number;
  load5?: number;
  load15?: number;
  warming_up: boolean;
  error?: string;
}

export interface MonitorSummaryEntry {
  server: MonitorServerRef;
  available: boolean;
  as_of?: string;
  disk_used_bytes?: number;
  disk_total_bytes?: number;
  disk_percent?: number;
  accounts_total?: number;
  domains_total?: number;
  applications_total?: number;
  error?: string;
}

export interface Mailbox {
  id: string;
  domain_id: string;
  email: string;
  display_name: string;
  quota_bytes: number;
  is_disabled: boolean;
  last_usage_bytes: number;
  last_usage_at?: string;
  created_at: string;
  updated_at: string;
  domain_name: string;
  owner_user_id: string;
  user_username: string;
}

export interface MailGroup {
  id: string;
  domain_id: string;
  local_part: string;
  email: string;
  display_name: string;
  description: string;
  group_kind: string;
  has_mailbox: boolean;
  has_calendar: boolean;
  has_addressbook: boolean;
  has_files: boolean;
  internal_only: boolean;
  created_at: string;
  updated_at: string;
  domain_name: string;
  owner_user_id: string;
  user_username: string;
  member_count: number;
}

export interface MailForwarder {
  id: string;
  mailbox_id: string;
  mailbox_email: string;
  domain_id: string;
  domain_name: string;
  type: string;
  local_part?: string;
  target: string;
  keep_copy: boolean;
  enabled: boolean;
  created_at: string;
}

export interface DomainForwarder {
  id: string;
  domain_id: string;
  domain_name: string;
  type: string;
  local_part: string;
  target: string;
  enabled: boolean;
  managed_by: string;
  created_at: string;
}

export interface MailAutoresponder {
  mailbox_id: string;
  mailbox_email?: string;
  domain_id?: string;
  domain_name?: string;
  enabled: boolean;
  from_date?: string | null;
  to_date?: string | null;
  subject?: string | null;
  text_body?: string | null;
  html_body?: string | null;
  updated_at: string;
}

export interface MailSnapshotEntry {
  server: MonitorServerRef;
  available: boolean;
  mailboxes: Mailbox[];
  groups: MailGroup[];
  forwarders: MailForwarder[];
  domain_forwarders: DomainForwarder[];
  autoresponders: MailAutoresponder[];
  error?: string;
}

export interface Heartbeat {
  id: string;
  server_id: string;
  healthy: boolean;
  version: string;
  checked_at: string;
}

export interface HeartbeatHistory {
  data: Heartbeat[];
  total: number;
  uptime: { healthy: number; total: number; ratio: number };
}

export interface MetricSample {
  id: string;
  server_id: string;
  cpu_percent?: number;
  ram_percent?: number;
  disk_percent?: number;
  load1?: number;
  sampled_at: string;
}

export interface MetricHistory {
  data: MetricSample[];
  total: number;
}

export interface Admin {
  id: string;
  username: string;
  role: "viewer" | "operator" | "owner";
  created_at: string;
}
