import axios from 'axios'
import type { AxiosInstance, AxiosError } from 'axios'
import type {
  LoginRequest,
  LoginResponse,
  Provider,
  ProviderListResponse,
  UpdateProviderRequest,
  Job,
  JobListResponse,
  StorageStats,
  AuditLogResponse,
  SanitizedConfig,
  BackupResponse,
  ProcessorStatus,
  LoadProvidersResponse,
  ApiError
} from '@/types'

// Create axios instance with base configuration
const api: AxiosInstance = axios.create({
  baseURL: import.meta.env.VITE_API_URL || '/admin/api',
  timeout: 30000,
  headers: {
    'Content-Type': 'application/json'
  }
})

// Request interceptor to add auth token
api.interceptors.request.use(
  (config) => {
    const token = localStorage.getItem('auth_token')
    if (token) {
      config.headers.Authorization = `Bearer ${token}`
    }
    return config
  },
  (error) => Promise.reject(error)
)

// Response interceptor for error handling
api.interceptors.response.use(
  (response) => response,
  (error: AxiosError<ApiError>) => {
    // Don't redirect on 401 for login endpoint - let the login form handle it
    const isLoginRequest = error.config?.url === '/login'
    
    if (error.response?.status === 401 && !isLoginRequest) {
      // Clear token and redirect to login
      localStorage.removeItem('auth_token')
      localStorage.removeItem('auth_user')
      window.location.href = '/login'
    }
    return Promise.reject(error)
  }
)

// Auth API
export const authApi = {
  login: async (credentials: LoginRequest): Promise<LoginResponse> => {
    const response = await api.post<LoginResponse>('/login', credentials)
    return response.data
  },

  logout: async (): Promise<void> => {
    await api.post('/logout')
  }
}

// Providers API
export const providersApi = {
  list: async (params?: { namespace?: string; type?: string }): Promise<ProviderListResponse> => {
    const response = await api.get<ProviderListResponse>('/providers', { params })
    return response.data
  },

  get: async (id: number): Promise<Provider> => {
    const response = await api.get<Provider>(`/providers/${id}`)
    return response.data
  },

  update: async (id: number, data: UpdateProviderRequest): Promise<Provider> => {
    const response = await api.put<Provider>(`/providers/${id}`, data)
    return response.data
  },

  delete: async (id: number): Promise<void> => {
    await api.delete(`/providers/${id}`)
  },

  load: async (file: File): Promise<LoadProvidersResponse> => {
    const formData = new FormData()
    formData.append('file', file)
    const response = await api.post<LoadProvidersResponse>('/providers/load', formData, {
      headers: {
        'Content-Type': 'multipart/form-data'
      }
    })
    return response.data
  }
}

// Jobs API
export const jobsApi = {
  list: async (params?: { limit?: number; offset?: number }): Promise<JobListResponse> => {
    const response = await api.get<JobListResponse>('/jobs', { params })
    return response.data
  },

  get: async (id: number): Promise<Job> => {
    const response = await api.get<Job>(`/jobs/${id}`)
    return response.data
  },

  retry: async (id: number): Promise<{ message: string; reset_count: number; job_id: number }> => {
    const response = await api.post(`/jobs/${id}/retry`)
    return response.data
  }
}

// Stats API
export const statsApi = {
  storage: async (): Promise<StorageStats> => {
    const response = await api.get<StorageStats>('/stats/storage')
    return response.data
  },

  audit: async (params?: {
    limit?: number
    offset?: number
    action?: string
    resource_type?: string
    resource_id?: string
  }): Promise<AuditLogResponse> => {
    const response = await api.get<AuditLogResponse>('/stats/audit', { params })
    return response.data
  }
}

// Config API
export const configApi = {
  get: async (): Promise<SanitizedConfig> => {
    const response = await api.get<SanitizedConfig>('/config')
    return response.data
  }
}

// Backup API
export const backupApi = {
  trigger: async (): Promise<BackupResponse> => {
    const response = await api.post<BackupResponse>('/backup')
    return response.data
  }
}

// Processor API
export const processorApi = {
  status: async (): Promise<ProcessorStatus> => {
    const response = await api.get<ProcessorStatus>('/processor/status')
    return response.data
  }
}

export default api
