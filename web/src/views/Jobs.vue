<template>
  <AdminLayout>
    <!-- Page header -->
    <div class="mb-6">
      <h1 class="text-2xl font-bold text-gray-900">Jobs</h1>
      <p class="mt-1 text-sm text-gray-600">Monitor and manage sync jobs</p>
    </div>

    <!-- Filter tabs -->
    <div class="bg-white rounded-lg shadow mb-6">
      <div class="border-b border-gray-200">
        <nav class="flex -mb-px">
          <button
            v-for="tab in tabs"
            :key="tab.value"
            @click="statusFilter = tab.value"
            :class="[
              statusFilter === tab.value
                ? 'border-indigo-500 text-indigo-600'
                : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300',
              'whitespace-nowrap py-4 px-6 border-b-2 font-medium text-sm'
            ]"
          >
            {{ tab.label }}
            <span
              v-if="tab.count > 0"
              :class="[
                statusFilter === tab.value ? 'bg-indigo-100 text-indigo-600' : 'bg-gray-100 text-gray-900',
                'ml-2 py-0.5 px-2.5 rounded-full text-xs font-medium'
              ]"
            >
              {{ tab.count }}
            </span>
          </button>
        </nav>
      </div>
    </div>

    <!-- Jobs list -->
    <div class="bg-white rounded-lg shadow overflow-hidden">
      <div v-if="jobsStore.loading" class="p-8 text-center text-gray-500">
        <svg class="animate-spin h-8 w-8 mx-auto text-indigo-600" fill="none" viewBox="0 0 24 24">
          <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
          <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
        </svg>
        <p class="mt-2">Loading jobs...</p>
      </div>

      <div v-else-if="filteredJobs.length === 0" class="p-8 text-center text-gray-500">
        <svg class="mx-auto h-12 w-12 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2" />
        </svg>
        <h3 class="mt-2 text-sm font-medium text-gray-900">No jobs found</h3>
        <p class="mt-1 text-sm text-gray-500">
          {{ statusFilter ? 'No jobs with this status' : 'Jobs will appear here when providers are synced' }}
        </p>
      </div>

      <div v-else class="divide-y divide-gray-200">
        <div
          v-for="job in filteredJobs"
          :key="job.id"
          class="p-6 hover:bg-gray-50"
        >
          <div class="flex items-start justify-between">
            <div class="flex-1">
              <div class="flex items-center space-x-3">
                <span :class="getStatusBadgeClass(job.status)">
                  {{ job.status }}
                </span>
                <h3 class="text-sm font-medium text-gray-900">{{ job.source_type }}</h3>
                <span class="text-xs text-gray-500">#{{ job.id }}</span>
              </div>

              <div class="mt-2 flex items-center space-x-4 text-sm text-gray-500">
                <span>
                  <svg class="inline-block h-4 w-4 mr-1" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
                  </svg>
                  Started: {{ job.started_at ? formatDate(job.started_at) : 'Not started' }}
                </span>
                <span v-if="job.completed_at">
                  Completed: {{ formatDate(job.completed_at) }}
                </span>
                <span v-if="job.completed_at && job.started_at">
                  Duration: {{ calculateDuration(job.started_at, job.completed_at) }}
                </span>
              </div>

              <!-- Progress bar for running jobs -->
              <div v-if="job.status === 'running' && job.total_items > 0" class="mt-3">
                <div class="flex items-center justify-between text-xs text-gray-500 mb-1">
                  <span>Progress</span>
                  <span>{{ job.completed_items }}/{{ job.total_items }} ({{ job.failed_items }} failed)</span>
                </div>
                <div class="w-full bg-gray-200 rounded-full h-2">
                  <div
                    class="bg-indigo-600 h-2 rounded-full transition-all duration-300"
                    :style="{ width: `${(job.completed_items / job.total_items) * 100}%` }"
                  />
                </div>
              </div>

              <!-- Error message for failed jobs -->
              <div v-if="job.status === 'failed' && job.error_message" class="mt-3">
                <div class="bg-red-50 border border-red-200 rounded-md p-3">
                  <div class="flex">
                    <svg class="h-5 w-5 text-red-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                    </svg>
                    <p class="ml-2 text-sm text-red-700">{{ job.error_message }}</p>
                  </div>
                </div>
              </div>
            </div>

            <div class="ml-4 flex-shrink-0 flex space-x-2">
              <button
                v-if="job.status === 'pending' || job.status === 'running'"
                @click="handleCancel(job)"
                :disabled="cancelling === job.id"
                class="inline-flex items-center px-3 py-1.5 border border-red-300 shadow-sm text-xs font-medium rounded text-red-700 bg-white hover:bg-red-50 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-red-500 disabled:opacity-50"
              >
                <svg class="mr-1.5 h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                </svg>
                {{ cancelling === job.id ? 'Cancelling...' : 'Cancel' }}
              </button>
              <button
                v-if="job.status === 'failed'"
                @click="handleRetry(job)"
                :disabled="retrying === job.id"
                class="inline-flex items-center px-3 py-1.5 border border-gray-300 shadow-sm text-xs font-medium rounded text-gray-700 bg-white hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500 disabled:opacity-50"
              >
                <svg class="mr-1.5 h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                </svg>
                {{ retrying === job.id ? 'Retrying...' : 'Retry' }}
              </button>
              <button
                @click="selectedJob = job; showDetailsModal = true"
                class="inline-flex items-center px-3 py-1.5 text-xs font-medium text-indigo-600 hover:text-indigo-800"
              >
                View Details
              </button>
            </div>
          </div>
        </div>
      </div>

      <!-- Pagination -->
      <div v-if="jobsStore.totalPages > 1" class="bg-white px-4 py-3 border-t border-gray-200 sm:px-6">
        <div class="flex items-center justify-between">
          <div class="text-sm text-gray-700">
            Showing page {{ jobsStore.currentPage }} of {{ jobsStore.totalPages }}
          </div>
          <div class="flex space-x-2">
            <button
              @click="jobsStore.prevPage()"
              :disabled="jobsStore.currentPage === 1"
              class="px-3 py-1 border rounded-md text-sm disabled:opacity-50 disabled:cursor-not-allowed hover:bg-gray-50"
            >
              Previous
            </button>
            <button
              @click="jobsStore.nextPage()"
              :disabled="jobsStore.currentPage === jobsStore.totalPages"
              class="px-3 py-1 border rounded-md text-sm disabled:opacity-50 disabled:cursor-not-allowed hover:bg-gray-50"
            >
              Next
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- Job Details Modal -->
    <Teleport to="body">
      <div v-if="showDetailsModal && selectedJob" class="fixed inset-0 z-50 overflow-y-auto">
        <div class="flex items-center justify-center min-h-screen pt-4 px-4 pb-20 text-center sm:p-0">
          <div class="fixed inset-0 bg-gray-500 bg-opacity-75" @click="showDetailsModal = false" />
          
          <div class="relative bg-white rounded-lg text-left overflow-hidden shadow-xl transform sm:my-8 sm:max-w-3xl sm:w-full">
            <div class="bg-white px-4 pt-5 pb-4 sm:p-6">
              <div class="flex items-start justify-between mb-4">
                <div>
                  <h3 class="text-lg font-medium text-gray-900">Job #{{ selectedJob.id }}</h3>
                  <p class="text-sm text-gray-500">{{ selectedJob.source_type }}</p>
                </div>
                <button @click="showDetailsModal = false" class="text-gray-400 hover:text-gray-600">
                  <svg class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                  </svg>
                </button>
              </div>

              <dl class="grid grid-cols-2 gap-4 mb-6">
                <div>
                  <dt class="text-sm font-medium text-gray-500">Status</dt>
                  <dd class="mt-1">
                    <span :class="getStatusBadgeClass(selectedJob.status)">
                      {{ selectedJob.status }}
                    </span>
                  </dd>
                </div>
                <div>
                  <dt class="text-sm font-medium text-gray-500">Type</dt>
                  <dd class="mt-1 text-sm text-gray-900">{{ selectedJob.source_type }}</dd>
                </div>
                <div>
                  <dt class="text-sm font-medium text-gray-500">Started At</dt>
                  <dd class="mt-1 text-sm text-gray-900">{{ selectedJob.started_at ? formatDate(selectedJob.started_at) : 'Not started' }}</dd>
                </div>
                <div>
                  <dt class="text-sm font-medium text-gray-500">Completed At</dt>
                  <dd class="mt-1 text-sm text-gray-900">
                    {{ selectedJob.completed_at ? formatDate(selectedJob.completed_at) : 'In progress' }}
                  </dd>
                </div>
              </dl>

              <!-- Progress -->
              <div v-if="selectedJob.total_items > 0" class="mb-6">
                <h4 class="text-sm font-medium text-gray-900 mb-2">Progress</h4>
                <div class="bg-gray-100 rounded-lg p-4">
                  <div class="grid grid-cols-3 gap-4 text-center">
                    <div>
                      <div class="text-2xl font-bold text-gray-900">{{ selectedJob.total_items }}</div>
                      <div class="text-xs text-gray-500">Total</div>
                    </div>
                    <div>
                      <div class="text-2xl font-bold text-green-600">{{ selectedJob.completed_items }}</div>
                      <div class="text-xs text-gray-500">Completed</div>
                    </div>
                    <div>
                      <div class="text-2xl font-bold text-red-600">{{ selectedJob.failed_items }}</div>
                      <div class="text-xs text-gray-500">Failed</div>
                    </div>
                  </div>
                </div>
              </div>

              <!-- Error -->
              <div v-if="selectedJob.error_message" class="mb-6">
                <h4 class="text-sm font-medium text-gray-900 mb-2">Error</h4>
                <div class="bg-red-50 border border-red-200 rounded-md p-4">
                  <p class="text-sm text-red-700 font-mono">{{ selectedJob.error_message }}</p>
                </div>
              </div>

              <!-- Job Items -->
              <div v-if="selectedJob.items?.length">
                <h4 class="text-sm font-medium text-gray-900 mb-2">Job Items</h4>
                <div class="max-h-64 overflow-y-auto border border-gray-200 rounded-md">
                  <table class="min-w-full divide-y divide-gray-200">
                    <thead class="bg-gray-50 sticky top-0">
                      <tr>
                        <th class="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase">Item</th>
                        <th class="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase">Status</th>
                        <th class="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase">Error</th>
                      </tr>
                    </thead>
                    <tbody class="divide-y divide-gray-200">
                      <tr v-for="item in selectedJob.items" :key="item.id" class="hover:bg-gray-50">
                        <td class="px-4 py-2 text-sm text-gray-900">{{ item.namespace }}/{{ item.type }}@{{ item.version }}</td>
                        <td class="px-4 py-2">
                          <span :class="getItemStatusClass(item.status)">{{ item.status }}</span>
                        </td>
                        <td class="px-4 py-2 text-sm text-red-600 truncate max-w-xs">{{ item.error_message || '-' }}</td>
                      </tr>
                    </tbody>
                  </table>
                </div>
              </div>
            </div>
            <div class="bg-gray-50 px-4 py-3 sm:px-6 sm:flex sm:flex-row-reverse">
              <button
                v-if="selectedJob.status === 'failed'"
                @click="handleRetry(selectedJob); showDetailsModal = false"
                class="w-full inline-flex justify-center rounded-md border border-transparent shadow-sm px-4 py-2 bg-indigo-600 text-base font-medium text-white hover:bg-indigo-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500 sm:ml-3 sm:w-auto sm:text-sm"
              >
                Retry Job
              </button>
              <button
                @click="showDetailsModal = false"
                class="mt-3 w-full inline-flex justify-center rounded-md border border-gray-300 shadow-sm px-4 py-2 bg-white text-base font-medium text-gray-700 hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500 sm:mt-0 sm:w-auto sm:text-sm"
              >
                Close
              </button>
            </div>
          </div>
        </div>
      </div>
    </Teleport>
  </AdminLayout>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import AdminLayout from '@/layouts/AdminLayout.vue'
import { useJobsStore } from '@/stores'
import type { Job } from '@/types'

const jobsStore = useJobsStore()

const statusFilter = ref('')
const showDetailsModal = ref(false)
const selectedJob = ref<Job | null>(null)
const retrying = ref<number | null>(null)
const cancelling = ref<number | null>(null)

// Auto-refresh interval
let refreshInterval: ReturnType<typeof setInterval> | null = null
const REFRESH_INTERVAL_ACTIVE = 2000  // 2 seconds when jobs are running
const REFRESH_INTERVAL_IDLE = 10000   // 10 seconds when no active jobs

// Check if there are any active (running/pending) jobs
const hasActiveJobs = computed(() => {
  return jobsStore.jobs.some(j => j.status === 'running' || j.status === 'pending')
})

// Start/update the refresh interval based on active jobs
function updateRefreshInterval() {
  // Clear existing interval
  if (refreshInterval) {
    clearInterval(refreshInterval)
  }
  
  // Set new interval based on whether there are active jobs
  const interval = hasActiveJobs.value ? REFRESH_INTERVAL_ACTIVE : REFRESH_INTERVAL_IDLE
  refreshInterval = setInterval(() => {
    jobsStore.fetchJobs()
  }, interval)
}

// Watch for changes in active jobs to adjust refresh rate
watch(hasActiveJobs, () => {
  updateRefreshInterval()
})

const tabs = computed(() => [
  { label: 'All', value: '', count: jobsStore.jobs.length },
  { label: 'Running', value: 'running', count: jobsStore.jobs.filter(j => j.status === 'running').length },
  { label: 'Pending', value: 'pending', count: jobsStore.jobs.filter(j => j.status === 'pending').length },
  { label: 'Completed', value: 'completed', count: jobsStore.jobs.filter(j => j.status === 'completed').length },
  { label: 'Failed', value: 'failed', count: jobsStore.jobs.filter(j => j.status === 'failed').length },
  { label: 'Cancelled', value: 'cancelled', count: jobsStore.jobs.filter(j => j.status === 'cancelled').length },
])

const filteredJobs = computed(() => {
  if (!statusFilter.value) return jobsStore.jobs
  return jobsStore.jobs.filter(j => j.status === statusFilter.value)
})

function getStatusBadgeClass(status: Job['status']): string {
  const classes: Record<string, string> = {
    pending: 'inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-yellow-100 text-yellow-800',
    running: 'inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-blue-100 text-blue-800',
    completed: 'inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-green-100 text-green-800',
    failed: 'inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-red-100 text-red-800',
    cancelled: 'inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-gray-100 text-gray-800',
  }
  return classes[status] || 'inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-gray-100 text-gray-800'
}

function getItemStatusClass(status: string): string {
  const classes: Record<string, string> = {
    pending: 'text-xs text-yellow-600',
    completed: 'text-xs text-green-600',
    failed: 'text-xs text-red-600',
    cancelled: 'text-xs text-gray-600',
  }
  return classes[status] || 'text-xs text-gray-600'
}

function formatDate(dateString: string): string {
  return new Date(dateString).toLocaleString('en-US', {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit'
  })
}

function calculateDuration(start: string, end: string): string {
  const startDate = new Date(start)
  const endDate = new Date(end)
  const diff = endDate.getTime() - startDate.getTime()
  
  const seconds = Math.floor(diff / 1000)
  const minutes = Math.floor(seconds / 60)
  const hours = Math.floor(minutes / 60)
  
  if (hours > 0) {
    return `${hours}h ${minutes % 60}m`
  }
  if (minutes > 0) {
    return `${minutes}m ${seconds % 60}s`
  }
  return `${seconds}s`
}

async function handleRetry(job: Job) {
  retrying.value = job.id
  try {
    await jobsStore.retryJob(job.id)
  } finally {
    retrying.value = null
  }
}

async function handleCancel(job: Job) {
  cancelling.value = job.id
  try {
    await jobsStore.cancelJob(job.id)
  } finally {
    cancelling.value = null
  }
}

onMounted(() => {
  jobsStore.fetchJobs()
  updateRefreshInterval()
})

onUnmounted(() => {
  if (refreshInterval) {
    clearInterval(refreshInterval)
  }
})
</script>
