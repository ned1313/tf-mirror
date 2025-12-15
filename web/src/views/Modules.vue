<template>
  <AdminLayout>
    <!-- Page header -->
    <div class="mb-6 flex items-center justify-between">
      <div>
        <h1 class="text-2xl font-bold text-gray-900">Modules</h1>
        <p class="mt-1 text-sm text-gray-600">Manage cached Terraform modules</p>
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
              placeholder="Search modules..."
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
          <div>
            <select
              v-model="systemFilter"
              class="rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 sm:text-sm"
            >
              <option value="">All systems</option>
              <option v-for="sys in systems" :key="sys" :value="sys">{{ sys }}</option>
            </select>
          </div>
        </div>
      </div>
    </div>

    <!-- Modules table -->
    <div class="bg-white rounded-lg shadow">
      <div v-if="modulesStore.loading" class="p-8 text-center text-gray-500">
        <svg class="animate-spin h-8 w-8 mx-auto text-indigo-600" fill="none" viewBox="0 0 24 24">
          <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
          <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
        </svg>
        <p class="mt-2">Loading modules...</p>
      </div>

      <div v-else-if="filteredModules.length === 0" class="p-8 text-center text-gray-500">
        <svg class="mx-auto h-12 w-12 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" />
        </svg>
        <h3 class="mt-2 text-sm font-medium text-gray-900">No modules found</h3>
        <p class="mt-1 text-sm text-gray-500">
          {{ searchQuery || statusFilter || namespaceFilter || systemFilter ? 'Try adjusting your filters' : 'Upload an HCL file to get started' }}
        </p>
      </div>

      <table v-else class="min-w-full divide-y divide-gray-200">
        <thead class="bg-gray-50">
          <tr>
            <th scope="col" class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
              Module
            </th>
            <th scope="col" class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
              Versions
            </th>
            <th scope="col" class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
              Status
            </th>
            <th scope="col" class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
              Last Updated
            </th>
            <th scope="col" class="relative px-6 py-3">
              <span class="sr-only">Actions</span>
            </th>
          </tr>
        </thead>
        <tbody class="bg-white divide-y divide-gray-200">
          <tr v-for="(module, index) in paginatedModules" :key="module.id" class="hover:bg-gray-50">
            <td class="px-6 py-4 whitespace-nowrap">
              <div class="flex items-center">
                <div>
                  <div class="text-sm font-medium text-gray-900">
                    {{ module.namespace }}/{{ module.name }}
                  </div>
                  <div class="text-sm text-gray-500">{{ module.system }}</div>
                </div>
              </div>
            </td>
            <td class="px-6 py-4 whitespace-nowrap">
              <div class="text-sm text-gray-900">{{ module.versions.length }} version(s)</div>
              <div class="text-xs text-gray-500">
                Latest: v{{ module.versions[0] }}
              </div>
            </td>
            <td class="px-6 py-4 whitespace-nowrap">
              <span :class="getStatusBadgeClass(module)">
                {{ getStatusLabel(module) }}
              </span>
            </td>
            <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
              {{ formatDate(module.updated_at) }}
            </td>
            <td class="px-6 py-4 whitespace-nowrap text-right text-sm font-medium">
              <div class="flex items-center justify-end space-x-2">
                <button
                  @click="selectedModule = module; showDetailsModal = true"
                  class="text-indigo-600 hover:text-indigo-900"
                >
                  View
                </button>
                <div class="relative" v-click-outside="() => closeDropdown(module.id)">
                  <button
                    @click="toggleDropdown(module.id)"
                    class="text-gray-400 hover:text-gray-600"
                  >
                    <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 5v.01M12 12v.01M12 19v.01M12 6a1 1 0 110-2 1 1 0 010 2zm0 7a1 1 0 110-2 1 1 0 010 2zm0 7a1 1 0 110-2 1 1 0 010 2z" />
                    </svg>
                  </button>
                  <div
                    v-if="openDropdown === module.id"
                    :class="[
                      'absolute right-0 w-48 rounded-md shadow-lg bg-white ring-1 ring-black ring-opacity-5 z-50',
                      index >= paginatedModules.length - 3 ? 'bottom-full mb-2' : 'mt-2'
                    ]"
                  >
                    <div class="py-1">
                      <button
                        @click="handleDeleteModule(module)"
                        class="block w-full text-left px-4 py-2 text-sm text-red-700 hover:bg-gray-100"
                      >
                        Delete Module
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
            Showing {{ (currentPage - 1) * pageSize + 1 }} to {{ Math.min(currentPage * pageSize, totalCount) }} of {{ totalCount }} modules
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
              <h3 class="text-lg font-medium text-gray-900 mb-4">Upload Module HCL File</h3>
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
              <div class="mt-4 p-3 bg-gray-50 rounded-md">
                <p class="text-xs text-gray-600">
                  <strong>Example HCL format:</strong>
                </p>
                <pre class="mt-1 text-xs text-gray-500 overflow-x-auto">module {
  namespace = "hashicorp"
  name      = "consul"
  system    = "aws"
  versions  = ["0.11.0", "0.10.0"]
}</pre>
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
      <div v-if="showDetailsModal && selectedModule" class="fixed inset-0 z-50 overflow-y-auto">
        <div class="flex items-center justify-center min-h-screen pt-4 px-4 pb-20 text-center sm:p-0">
          <div class="fixed inset-0 bg-gray-500 bg-opacity-75" @click="showDetailsModal = false" />
          
          <div class="relative bg-white rounded-lg text-left overflow-hidden shadow-xl transform sm:my-8 sm:max-w-2xl sm:w-full">
            <div class="bg-white px-4 pt-5 pb-4 sm:p-6">
              <div class="flex items-start justify-between mb-4">
                <div>
                  <h3 class="text-lg font-medium text-gray-900">
                    {{ selectedModule.namespace }}/{{ selectedModule.name }}
                  </h3>
                  <p class="text-sm text-gray-500">System: {{ selectedModule.system }}</p>
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
                    <span :class="getStatusBadgeClass(selectedModule)">
                      {{ getStatusLabel(selectedModule) }}
                    </span>
                  </dd>
                </div>
                <div>
                  <dt class="text-sm font-medium text-gray-500">Versions</dt>
                  <dd class="mt-1 text-sm text-gray-900">{{ selectedModule.versions.length }} version(s)</dd>
                </div>
                <div class="col-span-2">
                  <dt class="text-sm font-medium text-gray-500">Available Versions</dt>
                  <dd class="mt-1">
                    <div class="flex flex-wrap gap-2">
                      <span
                        v-for="version in selectedModule.versions"
                        :key="version"
                        class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-blue-100 text-blue-800"
                      >
                        v{{ version }}
                      </span>
                    </div>
                  </dd>
                </div>
                <div>
                  <dt class="text-sm font-medium text-gray-500">Created</dt>
                  <dd class="mt-1 text-sm text-gray-900">{{ formatDate(selectedModule.created_at) }}</dd>
                </div>
                <div>
                  <dt class="text-sm font-medium text-gray-500">Last Updated</dt>
                  <dd class="mt-1 text-sm text-gray-900">{{ formatDate(selectedModule.updated_at) }}</dd>
                </div>
                <div class="col-span-2">
                  <dt class="text-sm font-medium text-gray-500">Terraform Usage</dt>
                  <dd class="mt-1">
                    <pre class="p-3 bg-gray-100 rounded-md text-xs overflow-x-auto">module "{{ selectedModule.name }}" {
  source  = "{{ getMirrorHost() }}/{{ selectedModule.namespace }}/{{ selectedModule.name }}/{{ selectedModule.system }}"
  version = "{{ selectedModule.versions[0] }}"
}</pre>
                  </dd>
                </div>
              </dl>
            </div>
            <div class="bg-gray-50 px-4 py-3 sm:px-6 flex justify-end">
              <button
                @click="showDetailsModal = false"
                class="inline-flex justify-center rounded-md border border-gray-300 shadow-sm px-4 py-2 bg-white text-base font-medium text-gray-700 hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500 sm:text-sm"
              >
                Close
              </button>
            </div>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- Delete Confirmation Modal -->
    <Teleport to="body">
      <div v-if="showDeleteModal && moduleToDelete" class="fixed inset-0 z-50 overflow-y-auto">
        <div class="flex items-center justify-center min-h-screen pt-4 px-4 pb-20 text-center sm:p-0">
          <div class="fixed inset-0 bg-gray-500 bg-opacity-75" @click="showDeleteModal = false" />
          
          <div class="relative bg-white rounded-lg text-left overflow-hidden shadow-xl transform sm:my-8 sm:max-w-lg sm:w-full">
            <div class="bg-white px-4 pt-5 pb-4 sm:p-6 sm:pb-4">
              <div class="sm:flex sm:items-start">
                <div class="mx-auto flex-shrink-0 flex items-center justify-center h-12 w-12 rounded-full bg-red-100 sm:mx-0 sm:h-10 sm:w-10">
                  <svg class="h-6 w-6 text-red-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
                  </svg>
                </div>
                <div class="mt-3 text-center sm:mt-0 sm:ml-4 sm:text-left">
                  <h3 class="text-lg leading-6 font-medium text-gray-900">
                    Delete Module
                  </h3>
                  <div class="mt-2">
                    <p class="text-sm text-gray-500">
                      Are you sure you want to delete <strong>{{ moduleToDelete.namespace }}/{{ moduleToDelete.name }}/{{ moduleToDelete.system }}</strong>?
                      This will delete all {{ moduleToDelete.versions.length }} version(s) and cannot be undone.
                    </p>
                  </div>
                </div>
              </div>
            </div>
            <div class="bg-gray-50 px-4 py-3 sm:px-6 sm:flex sm:flex-row-reverse">
              <button
                @click="confirmDelete"
                :disabled="deleting"
                class="w-full inline-flex justify-center rounded-md border border-transparent shadow-sm px-4 py-2 bg-red-600 text-base font-medium text-white hover:bg-red-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-red-500 sm:ml-3 sm:w-auto sm:text-sm disabled:opacity-50"
              >
                {{ deleting ? 'Deleting...' : 'Delete' }}
              </button>
              <button
                @click="showDeleteModal = false"
                class="mt-3 w-full inline-flex justify-center rounded-md border border-gray-300 shadow-sm px-4 py-2 bg-white text-base font-medium text-gray-700 hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500 sm:mt-0 sm:ml-3 sm:w-auto sm:text-sm"
              >
                Cancel
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
import { useRouter } from 'vue-router'
import AdminLayout from '@/layouts/AdminLayout.vue'
import { useModulesStore } from '@/stores'
import type { AggregatedModule } from '@/types'

const router = useRouter()

const modulesStore = useModulesStore()

// State
const showUploadModal = ref(false)
const showDetailsModal = ref(false)
const showDeleteModal = ref(false)
const selectedModule = ref<AggregatedModule | null>(null)
const moduleToDelete = ref<AggregatedModule | null>(null)
const selectedFile = ref<File | null>(null)
const uploading = ref(false)
const deleting = ref(false)
const searchQuery = ref('')
const statusFilter = ref('')
const namespaceFilter = ref('')
const systemFilter = ref('')
const currentPage = ref(1)
const pageSize = 20
const openDropdown = ref<string | null>(null)

// Computed
const namespaces = computed(() => modulesStore.uniqueNamespaces)
const systems = computed(() => modulesStore.uniqueSystems)

const filteredModules = computed(() => {
  let result = modulesStore.aggregatedModules

  if (searchQuery.value) {
    const search = searchQuery.value.toLowerCase()
    result = result.filter(m =>
      m.namespace.toLowerCase().includes(search) ||
      m.name.toLowerCase().includes(search) ||
      m.system.toLowerCase().includes(search)
    )
  }

  if (statusFilter.value === 'active') {
    result = result.filter(m => !m.deprecated && !m.blocked)
  } else if (statusFilter.value === 'deprecated') {
    result = result.filter(m => m.deprecated)
  } else if (statusFilter.value === 'blocked') {
    result = result.filter(m => m.blocked)
  }

  if (namespaceFilter.value) {
    result = result.filter(m => m.namespace === namespaceFilter.value)
  }

  if (systemFilter.value) {
    result = result.filter(m => m.system === systemFilter.value)
  }

  return result
})

const totalCount = computed(() => filteredModules.value.length)
const totalPages = computed(() => Math.ceil(totalCount.value / pageSize))

const paginatedModules = computed(() => {
  const start = (currentPage.value - 1) * pageSize
  return filteredModules.value.slice(start, start + pageSize)
})

// Watch for filter changes to reset page
watch([searchQuery, statusFilter, namespaceFilter, systemFilter], () => {
  currentPage.value = 1
})

// Methods
function formatDate(dateString: string): string {
  return new Date(dateString).toLocaleDateString('en-US', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit'
  })
}

function getStatusLabel(module: AggregatedModule): string {
  if (module.blocked) return 'Blocked'
  if (module.deprecated) return 'Deprecated'
  return 'Active'
}

function getStatusBadgeClass(module: AggregatedModule): string {
  const base = 'inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium'
  if (module.blocked) return `${base} bg-red-100 text-red-800`
  if (module.deprecated) return `${base} bg-yellow-100 text-yellow-800`
  return `${base} bg-green-100 text-green-800`
}

function getMirrorHost(): string {
  // Get the current host for module source
  return window.location.host
}

function toggleDropdown(id: string) {
  openDropdown.value = openDropdown.value === id ? null : id
}

function closeDropdown(id: string) {
  if (openDropdown.value === id) {
    openDropdown.value = null
  }
}

function handleFileSelect(event: Event) {
  const target = event.target as HTMLInputElement
  const file = target.files?.[0]
  if (file) {
    selectedFile.value = file
  }
}

function handleDrop(event: DragEvent) {
  const files = event.dataTransfer?.files
  const file = files?.[0]
  if (file && (file.name.endsWith('.hcl') || file.name.endsWith('.tf'))) {
    selectedFile.value = file
  }
}

async function handleUpload() {
  if (!selectedFile.value) return

  uploading.value = true
  try {
    const response = await modulesStore.loadModules(selectedFile.value)
    if (response) {
      showUploadModal.value = false
      selectedFile.value = null
      // Navigate to Jobs page to track the upload progress
      router.push('/admin/jobs')
    }
  } finally {
    uploading.value = false
  }
}

function handleDeleteModule(module: AggregatedModule) {
  openDropdown.value = null
  moduleToDelete.value = module
  showDeleteModal.value = true
}

async function confirmDelete() {
  if (!moduleToDelete.value) return

  deleting.value = true
  try {
    // Find all module versions for this aggregated module and delete them
    const modulesToDelete = modulesStore.modules.filter(m =>
      m.namespace === moduleToDelete.value!.namespace &&
      m.name === moduleToDelete.value!.name &&
      m.system === moduleToDelete.value!.system
    )

    for (const m of modulesToDelete) {
      await modulesStore.deleteModule(m.id)
    }

    showDeleteModal.value = false
    moduleToDelete.value = null
    await modulesStore.fetchModules()
  } finally {
    deleting.value = false
  }
}

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

onMounted(() => {
  modulesStore.fetchModules()
})
</script>

<script lang="ts">
// Module declaration for directive
export default {
  name: 'ModulesView'
}
</script>
