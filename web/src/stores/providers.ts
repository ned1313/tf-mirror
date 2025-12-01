import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { providersApi } from '@/services/api'
import type { Provider, UpdateProviderRequest, LoadProvidersResponse } from '@/types'

export const useProvidersStore = defineStore('providers', () => {
  // State
  const providers = ref<Provider[]>([])
  const currentProvider = ref<Provider | null>(null)
  const loading = ref(false)
  const error = ref<string | null>(null)
  const filters = ref({
    namespace: '',
    type: '',
    search: ''
  })

  // Getters
  const filteredProviders = computed(() => {
    let result = providers.value

    if (filters.value.namespace) {
      result = result.filter(p => p.Namespace === filters.value.namespace)
    }
    if (filters.value.type) {
      result = result.filter(p => p.Type === filters.value.type)
    }
    if (filters.value.search) {
      const search = filters.value.search.toLowerCase()
      result = result.filter(p => 
        p.Namespace.toLowerCase().includes(search) ||
        p.Type.toLowerCase().includes(search) ||
        p.Version.toLowerCase().includes(search)
      )
    }

    return result
  })

  const uniqueNamespaces = computed(() => 
    [...new Set(providers.value.map(p => p.Namespace))].sort()
  )

  const uniqueTypes = computed(() => 
    [...new Set(providers.value.map(p => p.Type))].sort()
  )

  const totalCount = computed(() => providers.value.length)
  const deprecatedCount = computed(() => providers.value.filter(p => p.Deprecated).length)
  const blockedCount = computed(() => providers.value.filter(p => p.Blocked).length)

  // Actions
  async function fetchProviders(params?: { namespace?: string; type?: string }) {
    loading.value = true
    error.value = null
    
    try {
      const response = await providersApi.list(params)
      providers.value = response.providers || []
    } catch (err: unknown) {
      const axiosError = err as { response?: { data?: { message?: string } } }
      error.value = axiosError.response?.data?.message || 'Failed to fetch providers'
    } finally {
      loading.value = false
    }
  }

  async function fetchProvider(id: number) {
    loading.value = true
    error.value = null
    
    try {
      currentProvider.value = await providersApi.get(id)
    } catch (err: unknown) {
      const axiosError = err as { response?: { data?: { message?: string } } }
      error.value = axiosError.response?.data?.message || 'Failed to fetch provider'
      currentProvider.value = null
    } finally {
      loading.value = false
    }
  }

  async function updateProvider(id: number, data: UpdateProviderRequest): Promise<boolean> {
    loading.value = true
    error.value = null
    
    try {
      const updated = await providersApi.update(id, data)
      
      // Update in list
      const index = providers.value.findIndex(p => p.ID === id)
      if (index !== -1) {
        providers.value[index] = updated
      }
      
      // Update current if same
      if (currentProvider.value?.ID === id) {
        currentProvider.value = updated
      }
      
      return true
    } catch (err: unknown) {
      const axiosError = err as { response?: { data?: { message?: string } } }
      error.value = axiosError.response?.data?.message || 'Failed to update provider'
      return false
    } finally {
      loading.value = false
    }
  }

  async function deleteProvider(id: number): Promise<boolean> {
    loading.value = true
    error.value = null
    
    try {
      await providersApi.delete(id)
      
      // Remove from list
      providers.value = providers.value.filter(p => p.ID !== id)
      
      // Clear current if same
      if (currentProvider.value?.ID === id) {
        currentProvider.value = null
      }
      
      return true
    } catch (err: unknown) {
      const axiosError = err as { response?: { data?: { message?: string } } }
      error.value = axiosError.response?.data?.message || 'Failed to delete provider'
      return false
    } finally {
      loading.value = false
    }
  }

  async function loadProviders(file: File): Promise<LoadProvidersResponse | null> {
    loading.value = true
    error.value = null
    
    try {
      const response = await providersApi.load(file)
      return response
    } catch (err: unknown) {
      const axiosError = err as { response?: { data?: { message?: string } } }
      error.value = axiosError.response?.data?.message || 'Failed to load providers'
      return null
    } finally {
      loading.value = false
    }
  }

  function setFilter(key: keyof typeof filters.value, value: string) {
    filters.value[key] = value
  }

  function clearFilters() {
    filters.value = { namespace: '', type: '', search: '' }
  }

  function clearError() {
    error.value = null
  }

  return {
    // State
    providers,
    currentProvider,
    loading,
    error,
    filters,
    // Getters
    filteredProviders,
    uniqueNamespaces,
    uniqueTypes,
    totalCount,
    deprecatedCount,
    blockedCount,
    // Actions
    fetchProviders,
    fetchProvider,
    updateProvider,
    deleteProvider,
    loadProviders,
    setFilter,
    clearFilters,
    clearError
  }
})
