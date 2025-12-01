import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { jobsApi } from '@/services/api'
import type { Job } from '@/types'

export const useJobsStore = defineStore('jobs', () => {
  // State
  const jobs = ref<Job[]>([])
  const currentJob = ref<Job | null>(null)
  const loading = ref(false)
  const error = ref<string | null>(null)
  const pagination = ref({
    limit: 10,
    offset: 0,
    total: 0
  })

  // Getters
  const pendingJobs = computed(() => 
    jobs.value.filter((j: Job) => j.status === 'pending')
  )

  const runningJobs = computed(() => 
    jobs.value.filter((j: Job) => j.status === 'running')
  )

  const completedJobs = computed(() => 
    jobs.value.filter((j: Job) => j.status === 'completed')
  )

  const failedJobs = computed(() => 
    jobs.value.filter((j: Job) => j.status === 'failed')
  )

  const hasMore = computed(() => 
    pagination.value.offset + pagination.value.limit < pagination.value.total
  )

  const currentPage = computed(() => 
    Math.floor(pagination.value.offset / pagination.value.limit) + 1
  )

  const totalPages = computed(() => 
    Math.ceil(pagination.value.total / pagination.value.limit)
  )

  // Actions
  async function fetchJobs(params?: { limit?: number; offset?: number }) {
    loading.value = true
    error.value = null
    
    try {
      const response = await jobsApi.list({
        limit: params?.limit || pagination.value.limit,
        offset: params?.offset ?? pagination.value.offset
      })
      
      jobs.value = response.jobs || []
      pagination.value.total = response.total
      pagination.value.limit = response.limit
      pagination.value.offset = response.offset
    } catch (err: unknown) {
      const axiosError = err as { response?: { data?: { message?: string } } }
      error.value = axiosError.response?.data?.message || 'Failed to fetch jobs'
    } finally {
      loading.value = false
    }
  }

  async function fetchJob(id: number) {
    loading.value = true
    error.value = null
    
    try {
      currentJob.value = await jobsApi.get(id)
    } catch (err: unknown) {
      const axiosError = err as { response?: { data?: { message?: string } } }
      error.value = axiosError.response?.data?.message || 'Failed to fetch job'
      currentJob.value = null
    } finally {
      loading.value = false
    }
  }

  async function retryJob(id: number): Promise<boolean> {
    loading.value = true
    error.value = null
    
    try {
      await jobsApi.retry(id)
      
      // Refresh job details
      await fetchJob(id)
      
      // Refresh list
      await fetchJobs()
      
      return true
    } catch (err: unknown) {
      const axiosError = err as { response?: { data?: { message?: string } } }
      error.value = axiosError.response?.data?.message || 'Failed to retry job'
      return false
    } finally {
      loading.value = false
    }
  }

  async function cancelJob(id: number): Promise<boolean> {
    loading.value = true
    error.value = null
    
    try {
      await jobsApi.cancel(id)
      
      // Refresh job details if viewing this job
      if (currentJob.value?.id === id) {
        await fetchJob(id)
      }
      
      // Refresh list
      await fetchJobs()
      
      return true
    } catch (err: unknown) {
      const axiosError = err as { response?: { data?: { message?: string } } }
      error.value = axiosError.response?.data?.message || 'Failed to cancel job'
      return false
    } finally {
      loading.value = false
    }
  }

  function nextPage() {
    pagination.value.offset += pagination.value.limit
    fetchJobs()
  }

  function prevPage() {
    pagination.value.offset = Math.max(0, pagination.value.offset - pagination.value.limit)
    fetchJobs()
  }

  function setPageSize(size: number) {
    pagination.value.limit = size
    pagination.value.offset = 0
    fetchJobs()
  }

  function clearError() {
    error.value = null
  }

  return {
    // State
    jobs,
    currentJob,
    loading,
    error,
    pagination,
    // Getters
    pendingJobs,
    runningJobs,
    completedJobs,
    failedJobs,
    hasMore,
    currentPage,
    totalPages,
    // Actions
    fetchJobs,
    fetchJob,
    retryJob,
    cancelJob,
    nextPage,
    prevPage,
    setPageSize,
    clearError
  }
})
