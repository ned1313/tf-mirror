<template>
  <AdminLayout>
    <!-- Page header -->
    <div class="mb-6">
      <h1 class="text-2xl font-bold text-gray-900">Settings</h1>
      <p class="mt-1 text-sm text-gray-600">View configuration and system information</p>
    </div>

    <div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
      <!-- Configuration -->
      <div class="bg-white rounded-lg shadow">
        <div class="px-6 py-4 border-b border-gray-200">
          <h2 class="text-lg font-medium text-gray-900">Configuration</h2>
          <p class="text-sm text-gray-500">Current server configuration (read-only)</p>
        </div>
        <div class="p-6">
          <div v-if="statsStore.configLoading" class="text-center text-gray-500">Loading...</div>
          <div v-else-if="!statsStore.config" class="text-center text-gray-500">
            Unable to load configuration
          </div>
          <dl v-else class="space-y-4">
            <div>
              <dt class="text-sm font-medium text-gray-500">Server Port</dt>
              <dd class="mt-1 text-sm text-gray-900 font-mono">{{ statsStore.config.server.port }}</dd>
            </div>
            <div>
              <dt class="text-sm font-medium text-gray-500">TLS Enabled</dt>
              <dd class="mt-1">
                <span :class="statsStore.config.server.tls_enabled ? 'bg-green-100 text-green-800' : 'bg-gray-100 text-gray-800'" class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium">
                  {{ statsStore.config.server.tls_enabled ? 'Yes' : 'No' }}
                </span>
              </dd>
            </div>
            <div>
              <dt class="text-sm font-medium text-gray-500">Database Path</dt>
              <dd class="mt-1 text-sm text-gray-900 font-mono">{{ statsStore.config.database.path }}</dd>
            </div>
            <div>
              <dt class="text-sm font-medium text-gray-500">Backup Enabled</dt>
              <dd class="mt-1">
                <span :class="statsStore.config.database.backup_enabled ? 'bg-green-100 text-green-800' : 'bg-gray-100 text-gray-800'" class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium">
                  {{ statsStore.config.database.backup_enabled ? 'Yes' : 'No' }}
                </span>
              </dd>
            </div>
            <div>
              <dt class="text-sm font-medium text-gray-500">Storage Type</dt>
              <dd class="mt-1 text-sm text-gray-900 font-mono">{{ statsStore.config.storage.type }}</dd>
            </div>
            <div v-if="statsStore.config.storage.bucket">
              <dt class="text-sm font-medium text-gray-500">S3 Bucket</dt>
              <dd class="mt-1 text-sm text-gray-900 font-mono">{{ statsStore.config.storage.bucket }}</dd>
            </div>
            <div>
              <dt class="text-sm font-medium text-gray-500">Log Level</dt>
              <dd class="mt-1">
                <span :class="getLogLevelClass(statsStore.config.logging.level)">
                  {{ statsStore.config.logging.level }}
                </span>
              </dd>
            </div>
            <div>
              <dt class="text-sm font-medium text-gray-500">Max Concurrent Jobs</dt>
              <dd class="mt-1 text-sm text-gray-900">{{ statsStore.config.processor.max_concurrent_jobs }}</dd>
            </div>
            <div>
              <dt class="text-sm font-medium text-gray-500">Cache Memory</dt>
              <dd class="mt-1 text-sm text-gray-900">{{ statsStore.config.cache.memory_size_mb }} MB</dd>
            </div>
          </dl>
        </div>
      </div>

      <!-- System Information -->
      <div class="space-y-6">
        <!-- Storage Stats -->
        <div class="bg-white rounded-lg shadow">
          <div class="px-6 py-4 border-b border-gray-200">
            <h2 class="text-lg font-medium text-gray-900">Storage</h2>
          </div>
          <div class="p-6">
            <div v-if="statsStore.loading" class="text-center text-gray-500">Loading...</div>
            <dl v-else class="grid grid-cols-2 gap-4">
              <div>
                <dt class="text-sm font-medium text-gray-500">Total Providers</dt>
                <dd class="mt-1 text-2xl font-semibold text-gray-900">
                  {{ statsStore.storageStats?.total_providers ?? 0 }}
                </dd>
              </div>
              <div>
                <dt class="text-sm font-medium text-gray-500">Unique Versions</dt>
                <dd class="mt-1 text-2xl font-semibold text-gray-900">
                  {{ statsStore.storageStats?.unique_versions ?? 0 }}
                </dd>
              </div>
              <div>
                <dt class="text-sm font-medium text-gray-500">Total Size</dt>
                <dd class="mt-1 text-2xl font-semibold text-gray-900">
                  {{ statsStore.storageStats?.total_size_human ?? '0 B' }}
                </dd>
              </div>
              <div>
                <dt class="text-sm font-medium text-gray-500">Bytes Used</dt>
                <dd class="mt-1 text-sm text-gray-900 font-mono">
                  {{ (statsStore.storageStats?.total_size_bytes ?? 0).toLocaleString() }}
                </dd>
              </div>
            </dl>
          </div>
        </div>

        <!-- Backup -->
        <div class="bg-white rounded-lg shadow">
          <div class="px-6 py-4 border-b border-gray-200">
            <h2 class="text-lg font-medium text-gray-900">Database Backup</h2>
          </div>
          <div class="p-6">
            <p class="text-sm text-gray-600 mb-4">
              Create a backup of the database. Backups are stored in the configured backup directory.
            </p>
            <div class="flex items-center space-x-4">
              <button
                @click="handleBackup"
                :disabled="backingUp"
                class="inline-flex items-center px-4 py-2 border border-transparent rounded-md shadow-sm text-sm font-medium text-white bg-indigo-600 hover:bg-indigo-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500 disabled:opacity-50 disabled:cursor-not-allowed"
              >
                <svg
                  v-if="backingUp"
                  class="animate-spin -ml-1 mr-2 h-4 w-4 text-white"
                  fill="none"
                  viewBox="0 0 24 24"
                >
                  <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
                  <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
                </svg>
                <svg v-else class="-ml-1 mr-2 h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 7H5a2 2 0 00-2 2v9a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-3m-1 4l-3 3m0 0l-3-3m3 3V4" />
                </svg>
                {{ backingUp ? 'Creating Backup...' : 'Create Backup' }}
              </button>
              <span v-if="backupResult" :class="backupResult.success ? 'text-green-600' : 'text-red-600'" class="text-sm">
                {{ backupResult.message }}
              </span>
            </div>
          </div>
        </div>

        <!-- API Information -->
        <div class="bg-white rounded-lg shadow">
          <div class="px-6 py-4 border-b border-gray-200">
            <h2 class="text-lg font-medium text-gray-900">API Information</h2>
          </div>
          <div class="p-6">
            <dl class="space-y-4">
              <div>
                <dt class="text-sm font-medium text-gray-500">API Base URL</dt>
                <dd class="mt-1 text-sm text-gray-900 font-mono">{{ apiBaseUrl }}</dd>
              </div>
              <div>
                <dt class="text-sm font-medium text-gray-500">Terraform Provider Protocol</dt>
                <dd class="mt-1 text-sm text-gray-900">
                  <span class="font-mono">{{ apiBaseUrl }}/v1/providers/{namespace}/{type}</span>
                </dd>
              </div>
              <div>
                <dt class="text-sm font-medium text-gray-500">Terraform Configuration</dt>
                <dd class="mt-1">
                  <pre class="bg-gray-100 rounded-md p-3 text-xs text-gray-800 overflow-x-auto">provider_installation {
  network_mirror {
    url = "{{ apiBaseUrl }}/v1/providers/"
  }
}</pre>
                </dd>
              </div>
            </dl>
          </div>
        </div>
      </div>
    </div>
  </AdminLayout>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import AdminLayout from '@/layouts/AdminLayout.vue'
import { useStatsStore } from '@/stores'

const statsStore = useStatsStore()

const backingUp = ref(false)
const backupResult = ref<{ success: boolean; message: string } | null>(null)

const apiBaseUrl = import.meta.env.VITE_API_URL || window.location.origin

function getLogLevelClass(level: string): string {
  const classes: Record<string, string> = {
    debug: 'inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-gray-100 text-gray-800',
    info: 'inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-blue-100 text-blue-800',
    warn: 'inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-yellow-100 text-yellow-800',
    error: 'inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-red-100 text-red-800',
  }
  return classes[level] || classes.info
}

async function handleBackup() {
  backingUp.value = true
  backupResult.value = null
  try {
    const result = await statsStore.triggerBackup()
    backupResult.value = {
      success: true,
      message: `Backup created: ${result.backup_path}`
    }
  } catch (error) {
    backupResult.value = {
      success: false,
      message: error instanceof Error ? error.message : 'Backup failed'
    }
  } finally {
    backingUp.value = false
  }
}

onMounted(() => {
  statsStore.fetchConfig()
  statsStore.fetchStorageStats()
})
</script>
