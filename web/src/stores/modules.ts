import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { modulesApi } from '@/services/api'
import type { Module, UpdateModuleRequest, LoadModulesResponse, AggregatedModule } from '@/types'

export const useModulesStore = defineStore('modules', () => {
  // State
  const modules = ref<Module[]>([])
  const currentModule = ref<Module | null>(null)
  const loading = ref(false)
  const error = ref<string | null>(null)
  const pagination = ref({
    page: 1,
    pageSize: 20,
    total: 0,
    totalPages: 0
  })
  const filters = ref({
    namespace: '',
    name: '',
    system: '',
    search: ''
  })

  // Getters - aggregate modules by namespace/name/system
  const aggregatedModules = computed((): AggregatedModule[] => {
    const groups = new Map<string, AggregatedModule>()
    
    for (const m of modules.value) {
      const key = `${m.namespace}/${m.name}/${m.system}`
      
      if (!groups.has(key)) {
        groups.set(key, {
          id: key,
          namespace: m.namespace,
          name: m.name,
          system: m.system,
          versions: [],
          deprecated: m.deprecated,
          blocked: m.blocked,
          created_at: m.created_at,
          updated_at: m.updated_at
        })
      }
      
      const group = groups.get(key)!
      if (!group.versions.includes(m.version)) {
        group.versions.push(m.version)
      }
      // Use latest timestamps
      if (m.updated_at > group.updated_at) {
        group.updated_at = m.updated_at
      }
      // Mark as deprecated/blocked if any version is
      if (m.deprecated) group.deprecated = true
      if (m.blocked) group.blocked = true
    }
    
    // Sort versions descending
    for (const group of groups.values()) {
      group.versions.sort((a, b) => b.localeCompare(a, undefined, { numeric: true }))
    }
    
    return Array.from(groups.values())
  })

  const filteredModules = computed(() => {
    let result = aggregatedModules.value

    if (filters.value.namespace) {
      result = result.filter(m => m.namespace === filters.value.namespace)
    }
    if (filters.value.name) {
      result = result.filter(m => m.name === filters.value.name)
    }
    if (filters.value.system) {
      result = result.filter(m => m.system === filters.value.system)
    }
    if (filters.value.search) {
      const search = filters.value.search.toLowerCase()
      result = result.filter(m => 
        m.namespace.toLowerCase().includes(search) ||
        m.name.toLowerCase().includes(search) ||
        m.system.toLowerCase().includes(search)
      )
    }

    return result
  })

  const uniqueNamespaces = computed(() => 
    [...new Set(modules.value.map(m => m.namespace))].sort()
  )

  const uniqueNames = computed(() => 
    [...new Set(modules.value.map(m => m.name))].sort()
  )

  const uniqueSystems = computed(() => 
    [...new Set(modules.value.map(m => m.system))].sort()
  )

  const totalCount = computed(() => pagination.value.total)
  const deprecatedCount = computed(() => modules.value.filter(m => m.deprecated).length)
  const blockedCount = computed(() => modules.value.filter(m => m.blocked).length)

  // Actions
  async function fetchModules(params?: { 
    namespace?: string
    name?: string
    system?: string
    page?: number
    page_size?: number
  }) {
    loading.value = true
    error.value = null
    
    try {
      const response = await modulesApi.list({
        page: params?.page || pagination.value.page,
        page_size: params?.page_size || pagination.value.pageSize,
        ...params
      })
      modules.value = response.modules || []
      pagination.value = {
        page: response.page,
        pageSize: response.page_size,
        total: response.total,
        totalPages: response.total_pages
      }
    } catch (err: unknown) {
      const axiosError = err as { response?: { data?: { message?: string } } }
      error.value = axiosError.response?.data?.message || 'Failed to fetch modules'
    } finally {
      loading.value = false
    }
  }

  async function fetchModule(id: number) {
    loading.value = true
    error.value = null
    
    try {
      currentModule.value = await modulesApi.get(id)
    } catch (err: unknown) {
      const axiosError = err as { response?: { data?: { message?: string } } }
      error.value = axiosError.response?.data?.message || 'Failed to fetch module'
      currentModule.value = null
    } finally {
      loading.value = false
    }
  }

  async function updateModule(id: number, data: UpdateModuleRequest): Promise<boolean> {
    loading.value = true
    error.value = null
    
    try {
      const updated = await modulesApi.update(id, data)
      
      // Update in list
      const index = modules.value.findIndex(m => m.id === id)
      if (index !== -1) {
        modules.value[index] = updated
      }
      
      // Update current if same
      if (currentModule.value?.id === id) {
        currentModule.value = updated
      }
      
      return true
    } catch (err: unknown) {
      const axiosError = err as { response?: { data?: { message?: string } } }
      error.value = axiosError.response?.data?.message || 'Failed to update module'
      return false
    } finally {
      loading.value = false
    }
  }

  async function deleteModule(id: number): Promise<boolean> {
    loading.value = true
    error.value = null
    
    try {
      await modulesApi.delete(id)
      
      // Remove from list
      modules.value = modules.value.filter(m => m.id !== id)
      
      // Clear current if same
      if (currentModule.value?.id === id) {
        currentModule.value = null
      }
      
      return true
    } catch (err: unknown) {
      const axiosError = err as { response?: { data?: { message?: string } } }
      error.value = axiosError.response?.data?.message || 'Failed to delete module'
      return false
    } finally {
      loading.value = false
    }
  }

  async function loadModules(file: File): Promise<LoadModulesResponse | null> {
    loading.value = true
    error.value = null
    
    try {
      const response = await modulesApi.load(file)
      return response
    } catch (err: unknown) {
      const axiosError = err as { response?: { data?: { message?: string } } }
      error.value = axiosError.response?.data?.message || 'Failed to load modules'
      return null
    } finally {
      loading.value = false
    }
  }

  function setFilters(newFilters: Partial<typeof filters.value>) {
    filters.value = { ...filters.value, ...newFilters }
  }

  function clearFilters() {
    filters.value = { namespace: '', name: '', system: '', search: '' }
  }

  function setPage(page: number) {
    pagination.value.page = page
  }

  return {
    // State
    modules,
    currentModule,
    loading,
    error,
    pagination,
    filters,
    // Getters
    aggregatedModules,
    filteredModules,
    uniqueNamespaces,
    uniqueNames,
    uniqueSystems,
    totalCount,
    deprecatedCount,
    blockedCount,
    // Actions
    fetchModules,
    fetchModule,
    updateModule,
    deleteModule,
    loadModules,
    setFilters,
    clearFilters,
    setPage
  }
})
