import { defineStore } from 'pinia'
import { ref } from 'vue'
import { statsApi, configApi, backupApi, processorApi } from '@/services/api'
import type { StorageStats, AuditLogEntry, SanitizedConfig, ProcessorStatus, BackupResponse } from '@/types'

export const useStatsStore = defineStore('stats', () => {
  // State
  const storageStats = ref<StorageStats | null>(null)
  const auditLogs = ref<AuditLogEntry[]>([])
  const config = ref<SanitizedConfig | null>(null)
  const processorStatus = ref<ProcessorStatus | null>(null)
  const lastBackup = ref<BackupResponse | null>(null)
  const loading = ref(false)
  const auditLoading = ref(false)
  const configLoading = ref(false)
  const error = ref<string | null>(null)
  const auditPagination = ref({
    limit: 50,
    offset: 0,
    total: 0
  })

  // Actions
  async function fetchStorageStats() {
    loading.value = true
    error.value = null
    
    try {
      storageStats.value = await statsApi.storage()
    } catch (err: unknown) {
      const axiosError = err as { response?: { data?: { message?: string } } }
      error.value = axiosError.response?.data?.message || 'Failed to fetch storage stats'
    } finally {
      loading.value = false
    }
  }

  async function fetchAuditLogs(params?: {
    limit?: number
    offset?: number
    action?: string
    resource_type?: string
    resource_id?: string
  }) {
    auditLoading.value = true
    error.value = null
    
    try {
      const response = await statsApi.audit({
        limit: params?.limit || auditPagination.value.limit,
        offset: params?.offset ?? auditPagination.value.offset,
        action: params?.action,
        resource_type: params?.resource_type,
        resource_id: params?.resource_id
      })
      
      auditLogs.value = response.logs || []
      auditPagination.value.total = response.total
      auditPagination.value.limit = response.limit
      auditPagination.value.offset = response.offset
    } catch (err: unknown) {
      const axiosError = err as { response?: { data?: { message?: string } } }
      error.value = axiosError.response?.data?.message || 'Failed to fetch audit logs'
    } finally {
      auditLoading.value = false
    }
  }

  async function fetchConfig() {
    configLoading.value = true
    error.value = null
    
    try {
      config.value = await configApi.get()
    } catch (err: unknown) {
      const axiosError = err as { response?: { data?: { message?: string } } }
      error.value = axiosError.response?.data?.message || 'Failed to fetch config'
    } finally {
      configLoading.value = false
    }
  }

  async function fetchProcessorStatus() {
    loading.value = true
    error.value = null
    
    try {
      processorStatus.value = await processorApi.status()
    } catch (err: unknown) {
      const axiosError = err as { response?: { data?: { message?: string } } }
      error.value = axiosError.response?.data?.message || 'Failed to fetch processor status'
    } finally {
      loading.value = false
    }
  }

  async function triggerBackup(): Promise<BackupResponse> {
    loading.value = true
    error.value = null
    
    try {
      lastBackup.value = await backupApi.trigger()
      return lastBackup.value
    } catch (err: unknown) {
      const axiosError = err as { response?: { data?: { message?: string } } }
      error.value = axiosError.response?.data?.message || 'Failed to trigger backup'
      throw err
    } finally {
      loading.value = false
    }
  }

  function clearError() {
    error.value = null
  }

  return {
    // State
    storageStats,
    auditLogs,
    config,
    processorStatus,
    lastBackup,
    loading,
    auditLoading,
    configLoading,
    error,
    auditPagination,
    // Actions
    fetchStorageStats,
    fetchAuditLogs,
    fetchConfig,
    fetchProcessorStatus,
    triggerBackup,
    clearError
  }
})
