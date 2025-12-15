<template>
  <div class="min-h-screen bg-gray-50">
    <!-- Header -->
    <header class="bg-white shadow">
      <div class="max-w-7xl mx-auto py-6 px-4 sm:px-6 lg:px-8 flex items-center justify-between">
        <div>
          <router-link to="/" class="text-3xl font-bold text-gray-900 hover:text-indigo-600">
            Terraform Mirror
          </router-link>
          <p class="mt-1 text-sm text-gray-600">Browse Cached Providers</p>
        </div>
        <div class="flex space-x-4">
          <router-link
            to="/modules"
            class="text-gray-600 hover:text-indigo-600"
          >
            Modules
          </router-link>
          <router-link
            to="/login"
            class="text-indigo-600 hover:text-indigo-800 font-medium"
          >
            Admin Login
          </router-link>
        </div>
      </div>
    </header>

    <main class="max-w-7xl mx-auto py-6 sm:px-6 lg:px-8">
      <!-- Search and filters -->
      <div class="px-4 sm:px-0 mb-6">
        <div class="bg-white rounded-lg shadow p-4">
          <div class="flex flex-wrap items-center gap-4">
            <div class="flex-1 min-w-64">
              <input
                v-model="searchQuery"
                type="text"
                placeholder="Search providers (e.g., hashicorp/aws)..."
                class="w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 sm:text-sm"
              />
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

      <!-- Providers list -->
      <div class="px-4 sm:px-0">
        <div v-if="loading" class="bg-white rounded-lg shadow p-8 text-center text-gray-500">
          <svg class="animate-spin h-8 w-8 mx-auto text-indigo-600" fill="none" viewBox="0 0 24 24">
            <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
            <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
          </svg>
          <p class="mt-2">Loading providers...</p>
        </div>

        <div v-else-if="filteredProviders.length === 0" class="bg-white rounded-lg shadow p-8 text-center text-gray-500">
          <svg class="mx-auto h-12 w-12 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M20 13V6a2 2 0 00-2-2H6a2 2 0 00-2 2v7m16 0v5a2 2 0 01-2 2H6a2 2 0 01-2-2v-5m16 0h-2.586a1 1 0 00-.707.293l-2.414 2.414a1 1 0 01-.707.293h-3.172a1 1 0 01-.707-.293l-2.414-2.414A1 1 0 006.586 13H4" />
          </svg>
          <h3 class="mt-2 text-sm font-medium text-gray-900">No providers found</h3>
          <p class="mt-1 text-sm text-gray-500">
            {{ searchQuery || namespaceFilter ? 'Try adjusting your search filters' : 'No providers have been cached yet' }}
          </p>
        </div>

        <div v-else class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          <div
            v-for="provider in paginatedProviders"
            :key="`${provider.namespace}-${provider.type}`"
            class="bg-white rounded-lg shadow p-4 hover:shadow-md transition-shadow cursor-pointer"
            @click="selectedProvider = provider; showDetailsModal = true"
          >
            <div class="flex items-start justify-between">
              <div>
                <h3 class="text-lg font-medium text-gray-900">
                  {{ provider.namespace }}/{{ provider.type }}
                </h3>
                <p class="text-sm text-gray-500 mt-1">
                  {{ provider.versions.length }} version(s) available
                </p>
              </div>
              <span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-green-100 text-green-800">
                Cached
              </span>
            </div>
            <div class="mt-3 text-xs text-gray-500">
              Latest: v{{ provider.versions[0] }}
            </div>
          </div>
        </div>

        <!-- Pagination -->
        <div v-if="totalPages > 1" class="mt-6 flex items-center justify-between">
          <div class="text-sm text-gray-700">
            Showing {{ (currentPage - 1) * pageSize + 1 }} to {{ Math.min(currentPage * pageSize, filteredProviders.length) }} of {{ filteredProviders.length }}
          </div>
          <div class="flex space-x-2">
            <button
              @click="currentPage--"
              :disabled="currentPage === 1"
              class="px-3 py-1 border rounded-md text-sm disabled:opacity-50 disabled:cursor-not-allowed hover:bg-gray-50"
            >
              Previous
            </button>
            <button
              @click="currentPage++"
              :disabled="currentPage === totalPages"
              class="px-3 py-1 border rounded-md text-sm disabled:opacity-50 disabled:cursor-not-allowed hover:bg-gray-50"
            >
              Next
            </button>
          </div>
        </div>
      </div>
    </main>

    <!-- Provider Details Modal -->
    <Teleport to="body">
      <div v-if="showDetailsModal && selectedProvider" class="fixed inset-0 z-50 overflow-y-auto">
        <div class="flex items-center justify-center min-h-screen pt-4 px-4 pb-20 text-center sm:p-0">
          <div class="fixed inset-0 bg-gray-500 bg-opacity-75" @click="showDetailsModal = false" />
          
          <div class="relative bg-white rounded-lg text-left overflow-hidden shadow-xl transform sm:my-8 sm:max-w-2xl sm:w-full">
            <div class="bg-white px-4 pt-5 pb-4 sm:p-6">
              <div class="flex items-start justify-between mb-4">
                <div>
                  <h3 class="text-lg font-medium text-gray-900">
                    {{ selectedProvider.namespace }}/{{ selectedProvider.type }}
                  </h3>
                  <p class="text-sm text-gray-500">Terraform Provider</p>
                </div>
                <button @click="showDetailsModal = false" class="text-gray-400 hover:text-gray-600">
                  <svg class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                  </svg>
                </button>
              </div>

              <div class="space-y-4">
                <div>
                  <h4 class="text-sm font-medium text-gray-500">Available Versions</h4>
                  <div class="mt-2 flex flex-wrap gap-2">
                    <span
                      v-for="version in selectedProvider.versions"
                      :key="version"
                      class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-blue-100 text-blue-800"
                    >
                      v{{ version }}
                    </span>
                  </div>
                </div>

                <div>
                  <h4 class="text-sm font-medium text-gray-500">Terraform Configuration</h4>
                  <pre class="mt-2 p-3 bg-gray-100 rounded-md text-xs overflow-x-auto">terraform {
  required_providers {
    {{ selectedProvider.type }} = {
      source  = "{{ mirrorHost }}/{{ selectedProvider.namespace }}/{{ selectedProvider.type }}"
      version = "{{ selectedProvider.versions[0] }}"
    }
  }
}</pre>
                </div>

                <div>
                  <h4 class="text-sm font-medium text-gray-500">Mirror URL</h4>
                  <code class="mt-1 block p-2 bg-gray-100 rounded text-xs break-all">
                    {{ mirrorHost }}/v1/providers/{{ selectedProvider.namespace }}/{{ selectedProvider.type }}/
                  </code>
                </div>
              </div>
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
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'

interface Provider {
  namespace: string
  type: string
  versions: string[]
  platforms: string[]
}

const loading = ref(true)
const providers = ref<Provider[]>([])
const searchQuery = ref('')
const namespaceFilter = ref('')
const currentPage = ref(1)
const pageSize = 12
const showDetailsModal = ref(false)
const selectedProvider = ref<Provider | null>(null)

const mirrorHost = computed(() => window.location.host)

const namespaces = computed(() => {
  const ns = new Set(providers.value.map(p => p.namespace))
  return Array.from(ns).sort()
})

const filteredProviders = computed(() => {
  let result = providers.value
  
  if (searchQuery.value) {
    const query = searchQuery.value.toLowerCase()
    result = result.filter(p =>
      p.namespace.toLowerCase().includes(query) ||
      p.type.toLowerCase().includes(query)
    )
  }
  
  if (namespaceFilter.value) {
    result = result.filter(p => p.namespace === namespaceFilter.value)
  }
  
  return result
})

const totalPages = computed(() => Math.ceil(filteredProviders.value.length / pageSize))

const paginatedProviders = computed(() => {
  const start = (currentPage.value - 1) * pageSize
  return filteredProviders.value.slice(start, start + pageSize)
})

// Reset page when filters change
watch([searchQuery, namespaceFilter], () => {
  currentPage.value = 1
})

async function fetchProviders() {
  loading.value = true
  try {
    const response = await fetch('/api/public/providers?page_size=1000')
    if (response.ok) {
      const data = await response.json()
      providers.value = data.providers || []
    }
  } catch (error) {
    console.error('Failed to fetch providers:', error)
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  fetchProviders()
})
</script>
