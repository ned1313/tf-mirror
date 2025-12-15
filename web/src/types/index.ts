// API Response types

export interface ApiError {
  error: string
  message: string
}

// Auth types
export interface LoginRequest {
  username: string
  password: string
}

export interface LoginResponse {
  token: string
  expires_at: string
  user: {
    id: number
    username: string
  }
}

// Provider types - Go struct uses PascalCase, JSON uses PascalCase too
export interface Provider {
  ID: number
  Namespace: string
  Type: string
  Version: string
  Platform: string
  Filename: string
  DownloadURL: string
  Shasum: string
  SigningKeys: string | null
  S3Key: string
  SizeBytes: number
  Deprecated: boolean
  Blocked: boolean
  CreatedAt: string
  UpdatedAt: string
}

export interface ProviderListResponse {
  providers: Provider[]
  count: number
}

export interface UpdateProviderRequest {
  deprecated?: boolean
  blocked?: boolean
}

// Job types - Uses snake_case from json tags
export interface JobItem {
  id: number
  namespace: string
  type: string
  version: string
  platform: string
  status: string
  error_message?: string
}

export interface Job {
  id: number
  source_type: string
  status: 'pending' | 'running' | 'completed' | 'failed' | 'cancelled'
  progress: number
  total_items: number
  completed_items: number
  failed_items: number
  error_message?: string
  created_at: string
  started_at?: string
  completed_at?: string
  items?: JobItem[]
}

export interface JobListResponse {
  jobs: Job[]
  total: number
  limit: number
  offset: number
}

// Storage stats types
export interface StorageStats {
  total_providers: number
  total_size_bytes: number
  total_size_human: string
  unique_namespaces: number
  unique_types: number
  unique_versions: number
  deprecated_count: number
  blocked_count: number
}

// Audit log types - uses created_at not timestamp
export interface AuditLogEntry {
  id: number
  user_id?: number
  action: string
  resource_type: string
  resource_id?: string
  ip_address?: string
  success: boolean
  error_message?: string
  created_at: string
}

export interface AuditLogResponse {
  logs: AuditLogEntry[]
  total: number
  limit: number
  offset: number
}

// Config types - nested structure from Go
export interface SanitizedConfig {
  server: {
    port: number
    tls_enabled: boolean
    behind_proxy: boolean
  }
  storage: {
    type: string
    bucket: string
    region: string
    endpoint?: string
    force_path_style: boolean
  }
  database: {
    path: string
    backup_enabled: boolean
    backup_interval_hours: number
    backup_to_s3: boolean
  }
  cache: {
    memory_size_mb: number
    disk_path: string
    disk_size_gb: number
    ttl_seconds: number
  }
  features: {
    auto_download_providers: boolean
    auto_download_modules: boolean
    max_download_size_mb: number
  }
  processor: {
    polling_interval_seconds: number
    max_concurrent_jobs: number
    retry_attempts: number
    retry_delay_seconds: number
  }
  logging: {
    level: string
    format: string
    output: string
  }
  telemetry: {
    enabled: boolean
    otel_enabled: boolean
    export_traces: boolean
    export_metrics: boolean
  }
}

// Backup types
export interface BackupResponse {
  message: string
  backup_path?: string
  s3_key?: string
  size_bytes: number
  created_at: string
}

// Processor status types
export interface ProcessorStatus {
  running: boolean
  active_jobs: number
  processed_total: number
  failed_total: number
  last_poll_at?: string
  started_at?: string
}

// Provider loading types
export interface LoadProvidersResponse {
  job_id: number
  message: string
  total_providers: number
}

// Aggregated provider for UI display (grouped by namespace/type)
export interface AggregatedProvider {
  id: string // namespace/type as unique ID
  namespace: string
  name: string // same as type
  versions: string[]
  deprecated: boolean
  blocked: boolean
  last_synced?: string
  created_at: string
  updated_at: string
}

// Module types - matching backend ModuleResponse
export interface Module {
  id: number
  namespace: string
  name: string
  system: string
  version: string
  s3_key: string
  filename: string
  size_bytes: number
  original_source_url?: string
  deprecated: boolean
  blocked: boolean
  created_at: string
  updated_at: string
}

export interface ModuleListResponse {
  modules: Module[]
  total: number
  page: number
  page_size: number
  total_pages: number
}

export interface UpdateModuleRequest {
  deprecated?: boolean
  blocked?: boolean
}

export interface LoadModulesResponse {
  job_id: number
  message: string
  total_modules: number
}

// Aggregated module for UI display (grouped by namespace/name/system)
export interface AggregatedModule {
  id: string // namespace/name/system as unique ID
  namespace: string
  name: string
  system: string
  versions: string[]
  deprecated: boolean
  blocked: boolean
  created_at: string
  updated_at: string
}
