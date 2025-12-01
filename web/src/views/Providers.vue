<template>
  <AdminLayout>
    <!-- Page header -->
    <div class="mb-6 flex items-center justify-between">
      <div>
        <h1 class="text-2xl font-bold text-gray-900">Providers</h1>
        <p class="mt-1 text-sm text-gray-600">Manage cached Terraform providers</p>
      </div>
      <div class="flex space-x-3">
        <button
          @click="showUploadModal = true"
          class="inline-flex items-center px-4 py-2 border border-gray-300 rounded-md shadow-sm text-sm font-medium text-gray-700 bg-white hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500"
        >
          <svg class="-ml-1 mr-2 h-5 w-5 text-gray-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M15 13l-3-3m0 0l-3 3m3-3v12" />
          </svg>
          Upload HCL
        </button>
      </div>
    </div>

    <!-- Filters -->
    <div class="bg-white rounded-lg shadow mb-6">
      <div class="p-4 border-b border-gray-200">
        <div class="flex flex-wrap items-center gap-4">
          <div class="flex-1 min-w-64">
            <input
              v-model="searchQuery"
              type="text"
              placeholder="Search providers..."
              class="w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 sm:text-sm"
            />
          </div>
          <div>
            <select
              v-model="statusFilter"
              class="rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 sm:text-sm"
            >
              <option value="">All statuses</option>
              <option value="active">Active</option>
              <option value="deprecated">Deprecated</option>
              <option value="blocked">Blocked</option>
            </select>
          </div>
          <div>
            <select
              v-model="namespaceFilter"
              class="rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 sm:text-sm"
            >
              <option value="">All namespaces</option>
              <option v-for="ns in namespaces" :key="ns" :value="ns">{{ ns }}</option>
            </select>
          </div>
        </div>
      </div>
    </div>

    <!-- Providers table -->
    <div class="bg-white rounded-lg shadow">
      <div v-if="providersStore.loading" class="p-8 text-center text-gray-500">
        <svg class="animate-spin h-8 w-8 mx-auto text-indigo-600" fill="none" viewBox="0 0 24 24">
          <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
          <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
        </svg>
        <p class="mt-2">Loading providers...</p>
      </div>

      <div v-else-if="filteredProviders.length === 0" class="p-8 text-center text-gray-500">
        <svg class="mx-auto h-12 w-12 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10M4 7v10l8 4" />
        </svg>
        <h3 class="mt-2 text-sm font-medium text-gray-900">No providers found</h3>
        <p class="mt-1 text-sm text-gray-500">
          {{ searchQuery || statusFilter || namespaceFilter ? 'Try adjusting your filters' : 'Upload an HCL file to get started' }}
        </p>
      </div>

      <table v-else class="min-w-full divide-y divide-gray-200">
        <thead class="bg-gray-50">
          <tr>
            <th scope="col" class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
              Provider
            </th>
            <th scope="col" class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
              Versions
            </th>
            <th scope="col" class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
              Status
            </th>
            <th scope="col" class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
              Last Synced
            </th>
            <th scope="col" class="relative px-6 py-3">
              <span class="sr-only">Actions</span>
            </th>
          </tr>
        </thead>
        <tbody class="bg-white divide-y divide-gray-200">
          <tr v-for="(provider, index) in filteredProviders" :key="provider.ID" class="hover:bg-gray-50">
            <td class="px-6 py-4 whitespace-nowrap">
              <div class="flex items-center">
                <div>
                  <div class="text-sm font-medium text-gray-900">
                    {{ provider.Namespace }}/{{ provider.Type }}
                  </div>
                  <div class="text-sm text-gray-500">v{{ provider.Version }} - {{ provider.Platform }}</div>
                </div>
              </div>
            </td>
            <td class="px-6 py-4 whitespace-nowrap">
              <div class="text-sm text-gray-900">{{ provider.Version }}</div>
              <div class="text-xs text-gray-500">
                {{ formatBytes(provider.SizeBytes) }}
              </div>
            </td>
            <td class="px-6 py-4 whitespace-nowrap">
              <span :class="getStatusBadgeClass(provider)">
                {{ getStatusLabel(provider) }}
              </span>
            </td>
            <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
              {{ formatDate(provider.UpdatedAt) }}
            </td>
            <td class="px-6 py-4 whitespace-nowrap text-right text-sm font-medium">
              <div class="flex items-center justify-end space-x-2">
                <button
                  @click="selectedProvider = provider; showDetailsModal = true"
                  class="text-indigo-600 hover:text-indigo-900"
                >
                  View
                </button>
                <div class="relative" v-click-outside="() => closeDropdown(String(provider.ID))">
                  <button
                    @click="toggleDropdown(String(provider.ID))"
                    class="text-gray-400 hover:text-gray-600"
                  >
                    <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 5v.01M12 12v.01M12 19v.01M12 6a1 1 0 110-2 1 1 0 010 2zm0 7a1 1 0 110-2 1 1 0 010 2zm0 7a1 1 0 110-2 1 1 0 010 2z" />
                    </svg>
                  </button>
                  <div
                    v-if="openDropdown === String(provider.ID)"
                    :class="[
                      'absolute right-0 w-48 rounded-md shadow-lg bg-white ring-1 ring-black ring-opacity-5 z-50',
                      index >= filteredProviders.length - 3 ? 'bottom-full mb-2' : 'mt-2'
                    ]"
                  >
                    <div class="py-1">
                      <button
                        v-if="!provider.Deprecated"
                        @click="handleDeprecate(provider)"
                        class="block w-full text-left px-4 py-2 text-sm text-gray-700 hover:bg-gray-100"
                      >
                        Mark as Deprecated
                      </button>
                      <button
                        v-else
                        @click="handleUndeprecate(provider)"
                        class="block w-full text-left px-4 py-2 text-sm text-gray-700 hover:bg-gray-100"
                      >
                        Remove Deprecated
                      </button>
                      <button
                        v-if="!provider.Blocked"
                        @click="handleBlock(provider)"
                        class="block w-full text-left px-4 py-2 text-sm text-red-700 hover:bg-gray-100"
                      >
                        Block Provider
                      </button>
                      <button
                        v-else
                        @click="handleUnblock(provider)"
                        class="block w-full text-left px-4 py-2 text-sm text-gray-700 hover:bg-gray-100"
                      >
                        Unblock Provider
                      </button>
                      <button
                        @click="handleDelete(provider)"
                        class="block w-full text-left px-4 py-2 text-sm text-red-700 hover:bg-gray-100"
                      >
                        Delete Provider
                      </button>
                    </div>
                  </div>
                </div>
              </div>
            </td>
          </tr>
        </tbody>
      </table>

      <!-- Pagination -->
      <div v-if="totalPages > 1" class="bg-white px-4 py-3 border-t border-gray-200 sm:px-6">
        <div class="flex items-center justify-between">
          <div class="text-sm text-gray-700">
            Showing {{ (currentPage - 1) * pageSize + 1 }} to {{ Math.min(currentPage * pageSize, totalCount) }} of {{ totalCount }} providers
          </div>
          <div class="flex space-x-2">
            <button
              @click="currentPage = currentPage - 1"
              :disabled="currentPage === 1"
              class="px-3 py-1 border rounded-md text-sm disabled:opacity-50 disabled:cursor-not-allowed hover:bg-gray-50"
            >
              Previous
            </button>
            <button
              @click="currentPage = currentPage + 1"
              :disabled="currentPage === totalPages"
              class="px-3 py-1 border rounded-md text-sm disabled:opacity-50 disabled:cursor-not-allowed hover:bg-gray-50"
            >
              Next
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- Upload Modal -->
    <Teleport to="body">
      <div v-if="showUploadModal" class="fixed inset-0 z-50 overflow-y-auto">
        <div class="flex items-center justify-center min-h-screen pt-4 px-4 pb-20 text-center sm:p-0">
          <div class="fixed inset-0 bg-gray-500 bg-opacity-75" @click="showUploadModal = false" />
          
          <div class="relative bg-white rounded-lg text-left overflow-hidden shadow-xl transform sm:my-8 sm:max-w-lg sm:w-full">
            <div class="bg-white px-4 pt-5 pb-4 sm:p-6 sm:pb-4">
              <h3 class="text-lg font-medium text-gray-900 mb-4">Upload HCL File</h3>
              <div
                @drop.prevent="handleDrop"
                @dragover.prevent
                @dragenter.prevent
                class="border-2 border-dashed border-gray-300 rounded-lg p-8 text-center hover:border-indigo-500 transition-colors"
              >
                <input
                  ref="fileInput"
                  type="file"
                  accept=".hcl,.tf"
                  @change="handleFileSelect"
                  class="hidden"
                />
                <svg class="mx-auto h-12 w-12 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M15 13l-3-3m0 0l-3 3m3-3v12" />
                </svg>
                <p class="mt-2 text-sm text-gray-600">
                  <button @click="($refs.fileInput as HTMLInputElement).click()" class="text-indigo-600 hover:text-indigo-500">
                    Click to upload
                  </button>
                  or drag and drop
                </p>
                <p class="mt-1 text-xs text-gray-500">.hcl or .tf files</p>
                <p v-if="selectedFile" class="mt-2 text-sm text-indigo-600">
                  Selected: {{ selectedFile.name }}
                </p>
              </div>
            </div>
            <div class="bg-gray-50 px-4 py-3 sm:px-6 sm:flex sm:flex-row-reverse">
              <button
                @click="handleUpload"
                :disabled="!selectedFile || uploading"
                class="w-full inline-flex justify-center rounded-md border border-transparent shadow-sm px-4 py-2 bg-indigo-600 text-base font-medium text-white hover:bg-indigo-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500 sm:ml-3 sm:w-auto sm:text-sm disabled:opacity-50 disabled:cursor-not-allowed"
              >
                {{ uploading ? 'Uploading...' : 'Upload' }}
              </button>
              <button
                @click="showUploadModal = false; selectedFile = null"
                class="mt-3 w-full inline-flex justify-center rounded-md border border-gray-300 shadow-sm px-4 py-2 bg-white text-base font-medium text-gray-700 hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500 sm:mt-0 sm:ml-3 sm:w-auto sm:text-sm"
              >
                Cancel
              </button>
            </div>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- Details Modal -->
    <Teleport to="body">
      <div v-if="showDetailsModal && selectedProvider" class="fixed inset-0 z-50 overflow-y-auto">
        <div class="flex items-center justify-center min-h-screen pt-4 px-4 pb-20 text-center sm:p-0">
          <div class="fixed inset-0 bg-gray-500 bg-opacity-75" @click="showDetailsModal = false" />
          
          <div class="relative bg-white rounded-lg text-left overflow-hidden shadow-xl transform sm:my-8 sm:max-w-2xl sm:w-full">
            <div class="bg-white px-4 pt-5 pb-4 sm:p-6">
              <div class="flex items-start justify-between mb-4">
                <div>
                  <h3 class="text-lg font-medium text-gray-900">
                    {{ selectedProvider.Namespace }}/{{ selectedProvider.Type }}
                  </h3>
                  <p class="text-sm text-gray-500">v{{ selectedProvider.Version }} - {{ selectedProvider.Platform }}</p>
                </div>
                <button @click="showDetailsModal = false" class="text-gray-400 hover:text-gray-600">
                  <svg class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                  </svg>
                </button>
              </div>

              <dl class="grid grid-cols-2 gap-4">
                <div>
                  <dt class="text-sm font-medium text-gray-500">Status</dt>
                  <dd class="mt-1">
                    <span :class="getStatusBadgeClass(selectedProvider)">
                      {{ getStatusLabel(selectedProvider) }}
                    </span>
                  </dd>
                </div>
                <div>
                  <dt class="text-sm font-medium text-gray-500">Size</dt>
                  <dd class="mt-1 text-sm text-gray-900">
                    {{ formatBytes(selectedProvider.SizeBytes) }}
                  </dd>
                </div>
                <div>
                  <dt class="text-sm font-medium text-gray-500">Created</dt>
                  <dd class="mt-1 text-sm text-gray-900">{{ formatDate(selectedProvider.CreatedAt) }}</dd>
                </div>
                <div>
                  <dt class="text-sm font-medium text-gray-500">Updated</dt>
                  <dd class="mt-1 text-sm text-gray-900">{{ formatDate(selectedProvider.UpdatedAt) }}</dd>
                </div>
              </dl>

              <div class="mt-6">
                <h4 class="text-sm font-medium text-gray-900 mb-2">Details</h4>
                <dl class="space-y-2 text-sm">
                  <div>
                    <dt class="text-gray-500">Filename</dt>
                    <dd class="text-gray-900">{{ selectedProvider.Filename }}</dd>
                  </div>
                  <div>
                    <dt class="text-gray-500">SHA256</dt>
                    <dd class="text-gray-900 font-mono text-xs break-all">{{ selectedProvider.Shasum }}</dd>
                  </div>
                </dl>
              </div>
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
import { useProvidersStore } from '@/stores'
import type { Provider } from '@/types'

const providersStore = useProvidersStore()

const searchQuery = ref('')
const statusFilter = ref('')
const namespaceFilter = ref('')
const currentPage = ref(1)
const pageSize = 20
const openDropdown = ref<string | null>(null)
const showUploadModal = ref(false)
const showDetailsModal = ref(false)
const selectedProvider = ref<Provider | null>(null)
const selectedFile = ref<File | null>(null)
const uploading = ref(false)
const fileInput = ref<HTMLInputElement | null>(null)

const namespaces = computed(() => {
  const ns = new Set(providersStore.providers.map(p => p.Namespace))
  return Array.from(ns).sort()
})

const filteredProviders = computed(() => {
  let result = providersStore.providers

  if (searchQuery.value) {
    const query = searchQuery.value.toLowerCase()
    result = result.filter(p =>
      p.Type.toLowerCase().includes(query) ||
      p.Namespace.toLowerCase().includes(query) ||
      p.Version.toLowerCase().includes(query)
    )
  }

  if (statusFilter.value) {
    result = result.filter(p => {
      if (statusFilter.value === 'blocked') return p.Blocked
      if (statusFilter.value === 'deprecated') return p.Deprecated && !p.Blocked
      if (statusFilter.value === 'active') return !p.Deprecated && !p.Blocked
      return true
    })
  }

  if (namespaceFilter.value) {
    result = result.filter(p => p.Namespace === namespaceFilter.value)
  }

  return result
})

const totalCount = computed(() => filteredProviders.value.length)
const totalPages = computed(() => Math.ceil(totalCount.value / pageSize))

// Click outside directive with WeakMap for type safety
const clickOutsideHandlers = new WeakMap<HTMLElement, (event: Event) => void>()

const vClickOutside = {
  mounted(el: HTMLElement, binding: { value: () => void }) {
    const handler = (event: Event) => {
      if (!(el === event.target || el.contains(event.target as Node))) {
        binding.value()
      }
    }
    clickOutsideHandlers.set(el, handler)
    document.addEventListener('click', handler)
  },
  unmounted(el: HTMLElement) {
    const handler = clickOutsideHandlers.get(el)
    if (handler) {
      document.removeEventListener('click', handler)
      clickOutsideHandlers.delete(el)
    }
  }
}

function toggleDropdown(id: string) {
  openDropdown.value = openDropdown.value === id ? null : id
}

function closeDropdown(id: string) {
  if (openDropdown.value === id) {
    openDropdown.value = null
  }
}

function getStatusLabel(provider: Provider): string {
  if (provider.Blocked) return 'Blocked'
  if (provider.Deprecated) return 'Deprecated'
  return 'Active'
}

function getStatusBadgeClass(provider: Provider): string {
  if (provider.Blocked) return 'inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-red-100 text-red-800'
  if (provider.Deprecated) return 'inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-yellow-100 text-yellow-800'
  return 'inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-green-100 text-green-800'
}

function formatDate(dateString: string): string {
  return new Date(dateString).toLocaleDateString('en-US', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit'
  })
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 Bytes'
  const k = 1024
  const sizes = ['Bytes', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

async function handleDeprecate(provider: Provider) {
  openDropdown.value = null
  await providersStore.updateProvider(provider.ID, { deprecated: true })
}

async function handleUndeprecate(provider: Provider) {
  openDropdown.value = null
  await providersStore.updateProvider(provider.ID, { deprecated: false })
}

async function handleBlock(provider: Provider) {
  openDropdown.value = null
  if (confirm(`Are you sure you want to block ${provider.Namespace}/${provider.Type}?`)) {
    await providersStore.updateProvider(provider.ID, { blocked: true })
  }
}

async function handleUnblock(provider: Provider) {
  openDropdown.value = null
  await providersStore.updateProvider(provider.ID, { blocked: false })
}

async function handleDelete(provider: Provider) {
  openDropdown.value = null
  if (confirm(`Are you sure you want to delete ${provider.Namespace}/${provider.Type}? This cannot be undone.`)) {
    await providersStore.deleteProvider(provider.ID)
  }
}

function handleFileSelect(event: Event) {
  const input = event.target as HTMLInputElement
  if (input.files?.length) {
    selectedFile.value = input.files[0]
  }
}

function handleDrop(event: DragEvent) {
  const files = event.dataTransfer?.files
  if (files?.length) {
    const file = files[0]
    if (file.name.endsWith('.hcl') || file.name.endsWith('.tf')) {
      selectedFile.value = file
    }
  }
}

async function handleUpload() {
  if (!selectedFile.value) return
  
  uploading.value = true
  try {
    await providersStore.loadProviders(selectedFile.value)
    showUploadModal.value = false
    selectedFile.value = null
    // Refresh providers list
    await providersStore.fetchProviders()
  } finally {
    uploading.value = false
  }
}

// Reset page when filters change
watch([searchQuery, statusFilter, namespaceFilter], () => {
  currentPage.value = 1
})

onMounted(() => {
  providersStore.fetchProviders()
})
</script>
