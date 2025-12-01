<template>
  <AdminLayout>
    <!-- Page header -->
    <div class="mb-6">
      <h1 class="text-2xl font-bold text-gray-900">Audit Logs</h1>
      <p class="mt-1 text-sm text-gray-600">View all system activity and changes</p>
    </div>

    <!-- Filters -->
    <div class="bg-white rounded-lg shadow mb-6">
      <div class="p-4 border-b border-gray-200">
        <div class="flex flex-wrap items-center gap-4">
          <div class="flex-1 min-w-48">
            <input
              v-model="searchQuery"
              type="text"
              placeholder="Search by action, provider, or user..."
              class="w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 sm:text-sm"
            />
          </div>
          <div>
            <select
              v-model="actionFilter"
              class="rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 sm:text-sm"
            >
              <option value="">All actions</option>
              <option v-for="action in uniqueActions" :key="action" :value="action">{{ action }}</option>
            </select>
          </div>
          <div>
            <input
              v-model="dateFrom"
              type="date"
              class="rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 sm:text-sm"
              placeholder="From date"
            />
          </div>
          <div>
            <input
              v-model="dateTo"
              type="date"
              class="rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 sm:text-sm"
              placeholder="To date"
            />
          </div>
          <button
            @click="clearFilters"
            class="text-sm text-gray-500 hover:text-gray-700"
          >
            Clear filters
          </button>
        </div>
      </div>
    </div>

    <!-- Logs table -->
    <div class="bg-white rounded-lg shadow overflow-hidden">
      <div v-if="statsStore.auditLoading" class="p-8 text-center text-gray-500">
        <svg class="animate-spin h-8 w-8 mx-auto text-indigo-600" fill="none" viewBox="0 0 24 24">
          <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
          <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
        </svg>
        <p class="mt-2">Loading audit logs...</p>
      </div>

      <div v-else-if="filteredLogs.length === 0" class="p-8 text-center text-gray-500">
        <svg class="mx-auto h-12 w-12 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
        </svg>
        <h3 class="mt-2 text-sm font-medium text-gray-900">No logs found</h3>
        <p class="mt-1 text-sm text-gray-500">
          {{ hasFilters ? 'Try adjusting your filters' : 'Activity will appear here as you use the system' }}
        </p>
      </div>

      <table v-else class="min-w-full divide-y divide-gray-200">
        <thead class="bg-gray-50">
          <tr>
            <th scope="col" class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
              Timestamp
            </th>
            <th scope="col" class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
              Action
            </th>
            <th scope="col" class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
              Resource
            </th>
            <th scope="col" class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
              Status
            </th>
            <th scope="col" class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
              IP Address
            </th>
          </tr>
        </thead>
        <tbody class="bg-white divide-y divide-gray-200">
          <tr v-for="log in filteredLogs" :key="log.id" class="hover:bg-gray-50">
            <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
              {{ formatDate(log.created_at) }}
            </td>
            <td class="px-6 py-4 whitespace-nowrap">
              <span :class="getActionBadgeClass(log.action)">
                {{ log.action }}
              </span>
            </td>
            <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-900">
              <span class="text-xs text-gray-500">{{ log.resource_type }}</span>
              <span v-if="log.resource_id" class="ml-1">{{ log.resource_id }}</span>
            </td>
            <td class="px-6 py-4 whitespace-nowrap">
              <span v-if="log.success" class="text-green-600 text-sm">Success</span>
              <span v-else class="text-red-600 text-sm">Failed</span>
            </td>
            <td class="px-6 py-4 text-sm text-gray-500">
              {{ log.ip_address || '-' }}
            </td>
          </tr>
        </tbody>
      </table>

      <!-- Load more -->
      <div v-if="filteredLogs.length > 0 && hasMoreLogs" class="bg-gray-50 px-4 py-3 border-t border-gray-200 text-center">
        <button
          @click="loadMore"
          :disabled="loadingMore"
          class="text-sm text-indigo-600 hover:text-indigo-800 disabled:opacity-50"
        >
          {{ loadingMore ? 'Loading...' : 'Load more' }}
        </button>
      </div>
    </div>

    <!-- Details Modal -->
    <Teleport to="body">
      <div v-if="showDetailsModal && selectedLog" class="fixed inset-0 z-50 overflow-y-auto">
        <div class="flex items-center justify-center min-h-screen pt-4 px-4 pb-20 text-center sm:p-0">
          <div class="fixed inset-0 bg-gray-500 bg-opacity-75" @click="showDetailsModal = false" />
          
          <div class="relative bg-white rounded-lg text-left overflow-hidden shadow-xl transform sm:my-8 sm:max-w-lg sm:w-full">
            <div class="bg-white px-4 pt-5 pb-4 sm:p-6">
              <div class="flex items-start justify-between mb-4">
                <h3 class="text-lg font-medium text-gray-900">Log Details</h3>
                <button @click="showDetailsModal = false" class="text-gray-400 hover:text-gray-600">
                  <svg class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                  </svg>
                </button>
              </div>

              <dl class="space-y-4">
                <div>
                  <dt class="text-sm font-medium text-gray-500">Timestamp</dt>
                  <dd class="mt-1 text-sm text-gray-900">{{ formatDate(selectedLog.created_at) }}</dd>
                </div>
                <div>
                  <dt class="text-sm font-medium text-gray-500">Action</dt>
                  <dd class="mt-1">
                    <span :class="getActionBadgeClass(selectedLog.action)">{{ selectedLog.action }}</span>
                  </dd>
                </div>
                <div>
                  <dt class="text-sm font-medium text-gray-500">Resource</dt>
                  <dd class="mt-1 text-sm text-gray-900">
                    {{ selectedLog.resource_type }}
                    <span v-if="selectedLog.resource_id" class="ml-1">({{ selectedLog.resource_id }})</span>
                  </dd>
                </div>
                <div>
                  <dt class="text-sm font-medium text-gray-500">Status</dt>
                  <dd class="mt-1 text-sm">
                    <span v-if="selectedLog.success" class="text-green-600">Success</span>
                    <span v-else class="text-red-600">Failed</span>
                  </dd>
                </div>
                <div v-if="selectedLog.ip_address">
                  <dt class="text-sm font-medium text-gray-500">IP Address</dt>
                  <dd class="mt-1 text-sm text-gray-900">{{ selectedLog.ip_address }}</dd>
                </div>
                <div v-if="selectedLog.error_message">
                  <dt class="text-sm font-medium text-gray-500">Error</dt>
                  <dd class="mt-1">
                    <pre class="bg-red-50 rounded-md p-3 text-xs text-red-800 overflow-auto max-h-48">{{ selectedLog.error_message }}</pre>
                  </dd>
                </div>
              </dl>
            </div>
            <div class="bg-gray-50 px-4 py-3 sm:px-6 sm:flex sm:flex-row-reverse">
              <button
                @click="showDetailsModal = false"
                class="w-full inline-flex justify-center rounded-md border border-gray-300 shadow-sm px-4 py-2 bg-white text-base font-medium text-gray-700 hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500 sm:w-auto sm:text-sm"
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
import { ref, computed, onMounted, watch } from 'vue'
import AdminLayout from '@/layouts/AdminLayout.vue'
import { useStatsStore } from '@/stores'
import type { AuditLogEntry } from '@/types'

const statsStore = useStatsStore()

const searchQuery = ref('')
const actionFilter = ref('')
const dateFrom = ref('')
const dateTo = ref('')
const showDetailsModal = ref(false)
const selectedLog = ref<AuditLogEntry | null>(null)
const loadingMore = ref(false)
const currentLimit = ref(50)

const hasFilters = computed(() => {
  return searchQuery.value || actionFilter.value || dateFrom.value || dateTo.value
})

const uniqueActions = computed(() => {
  const actions = new Set(statsStore.auditLogs.map(l => l.action))
  return Array.from(actions).sort()
})

const filteredLogs = computed(() => {
  let result = statsStore.auditLogs

  if (searchQuery.value) {
    const query = searchQuery.value.toLowerCase()
    result = result.filter(l =>
      l.action.toLowerCase().includes(query) ||
      l.resource_id?.toLowerCase().includes(query) ||
      l.resource_type.toLowerCase().includes(query)
    )
  }

  if (actionFilter.value) {
    result = result.filter(l => l.action === actionFilter.value)
  }

  if (dateFrom.value) {
    const from = new Date(dateFrom.value)
    result = result.filter(l => new Date(l.created_at) >= from)
  }

  if (dateTo.value) {
    const to = new Date(dateTo.value)
    to.setHours(23, 59, 59, 999)
    result = result.filter(l => new Date(l.created_at) <= to)
  }

  return result
})

const hasMoreLogs = computed(() => {
  return statsStore.auditLogs.length >= currentLimit.value
})

function clearFilters() {
  searchQuery.value = ''
  actionFilter.value = ''
  dateFrom.value = ''
  dateTo.value = ''
}

function formatDate(dateString: string): string {
  return new Date(dateString).toLocaleString('en-US', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit'
  })
}

function getActionBadgeClass(action: string): string {
  if (action.includes('create') || action.includes('add')) {
    return 'inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-green-100 text-green-800'
  }
  if (action.includes('delete') || action.includes('remove')) {
    return 'inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-red-100 text-red-800'
  }
  if (action.includes('update') || action.includes('modify') || action.includes('deprecate') || action.includes('block')) {
    return 'inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-yellow-100 text-yellow-800'
  }
  if (action.includes('login') || action.includes('logout')) {
    return 'inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-purple-100 text-purple-800'
  }
  return 'inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-gray-100 text-gray-800'
}

async function loadMore() {
  loadingMore.value = true
  try {
    currentLimit.value += 50
    await statsStore.fetchAuditLogs({ limit: currentLimit.value })
  } finally {
    loadingMore.value = false
  }
}

// Refresh when filters change (with debounce for search)
let searchTimeout: ReturnType<typeof setTimeout>
watch(searchQuery, () => {
  clearTimeout(searchTimeout)
  searchTimeout = setTimeout(() => {
    // Re-filter happens automatically via computed
  }, 300)
})

onMounted(() => {
  statsStore.fetchAuditLogs({ limit: currentLimit.value })
})
</script>
