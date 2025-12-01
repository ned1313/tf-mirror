<template>
  <AdminLayout>
    <!-- Page header -->
    <div class="mb-6">
      <h1 class="text-2xl font-bold text-gray-900">Dashboard</h1>
      <p class="mt-1 text-sm text-gray-600">Overview of your Terraform registry mirror</p>
    </div>

    <!-- Stats cards -->
    <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6 mb-8">
      <div class="bg-white rounded-lg shadow p-6">
        <div class="flex items-center">
          <div class="flex-shrink-0 bg-blue-100 rounded-lg p-3">
            <svg class="h-6 w-6 text-blue-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10M4 7v10l8 4" />
            </svg>
          </div>
          <div class="ml-4">
            <p class="text-sm font-medium text-gray-500">Total Providers</p>
            <p class="text-2xl font-semibold text-gray-900">
              {{ statsStore.loading ? '...' : statsStore.storageStats?.total_providers ?? 0 }}
            </p>
          </div>
        </div>
      </div>

      <div class="bg-white rounded-lg shadow p-6">
        <div class="flex items-center">
          <div class="flex-shrink-0 bg-green-100 rounded-lg p-3">
            <svg class="h-6 w-6 text-green-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 7h.01M7 3h5c.512 0 1.024.195 1.414.586l7 7a2 2 0 010 2.828l-7 7a2 2 0 01-2.828 0l-7-7A1.994 1.994 0 013 12V7a4 4 0 014-4z" />
            </svg>
          </div>
          <div class="ml-4">
            <p class="text-sm font-medium text-gray-500">Total Versions</p>
            <p class="text-2xl font-semibold text-gray-900">
              {{ statsStore.loading ? '...' : statsStore.storageStats?.unique_versions ?? 0 }}
            </p>
          </div>
        </div>
      </div>

      <div class="bg-white rounded-lg shadow p-6">
        <div class="flex items-center">
          <div class="flex-shrink-0 bg-purple-100 rounded-lg p-3">
            <svg class="h-6 w-6 text-purple-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 7v10c0 2.21 3.582 4 8 4s8-1.79 8-4V7M4 7c0 2.21 3.582 4 8 4s8-1.79 8-4M4 7c0-2.21 3.582-4 8-4s8 1.79 8 4m0 5c0 2.21-3.582 4-8 4s-8-1.79-8-4" />
            </svg>
          </div>
          <div class="ml-4">
            <p class="text-sm font-medium text-gray-500">Storage Used</p>
            <p class="text-2xl font-semibold text-gray-900">
              {{ statsStore.loading ? '...' : statsStore.storageStats?.total_size_human ?? '0 B' }}
            </p>
          </div>
        </div>
      </div>

      <div class="bg-white rounded-lg shadow p-6">
        <div class="flex items-center">
          <div class="flex-shrink-0 bg-yellow-100 rounded-lg p-3">
            <svg class="h-6 w-6 text-yellow-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2" />
            </svg>
          </div>
          <div class="ml-4">
            <p class="text-sm font-medium text-gray-500">Active Jobs</p>
            <p class="text-2xl font-semibold text-gray-900">
              {{ jobsStore.loading ? '...' : activeJobCount }}
            </p>
          </div>
        </div>
      </div>
    </div>

    <!-- Two column layout -->
    <div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
      <!-- Recent Activity -->
      <div class="bg-white rounded-lg shadow">
        <div class="px-6 py-4 border-b border-gray-200">
          <h2 class="text-lg font-medium text-gray-900">Recent Activity</h2>
        </div>
        <div class="p-6">
          <div v-if="statsStore.loading" class="text-center text-gray-500">Loading...</div>
          <div v-else-if="statsStore.auditLogs.length === 0" class="text-center text-gray-500">
            No recent activity
          </div>
          <ul v-else class="space-y-4">
            <li v-for="log in recentLogs" :key="log.id" class="flex items-start space-x-3">
              <div :class="[
                'flex-shrink-0 w-2 h-2 rounded-full mt-2',
                getActionColor(log.action)
              ]" />
              <div class="min-w-0 flex-1">
                <p class="text-sm text-gray-900">
                  <span class="font-medium">{{ log.action }}</span>
                  <span v-if="log.resource_id" class="text-gray-600"> - {{ log.resource_id }}</span>
                </p>
                <p class="text-xs text-gray-500">
                  {{ formatRelativeTime(log.created_at) }}
                </p>
              </div>
            </li>
          </ul>
          <router-link
            v-if="statsStore.auditLogs.length > 0"
            to="/admin/audit"
            class="mt-4 block text-center text-sm text-indigo-600 hover:text-indigo-800"
          >
            View all activity →
          </router-link>
        </div>
      </div>

      <!-- Active Jobs -->
      <div class="bg-white rounded-lg shadow">
        <div class="px-6 py-4 border-b border-gray-200">
          <h2 class="text-lg font-medium text-gray-900">Active Jobs</h2>
        </div>
        <div class="p-6">
          <div v-if="jobsStore.loading" class="text-center text-gray-500">Loading...</div>
          <div v-else-if="activeJobs.length === 0" class="text-center text-gray-500">
            No active jobs
          </div>
          <ul v-else class="space-y-4">
            <li v-for="job in activeJobs" :key="job.id" class="border border-gray-200 rounded-lg p-4">
              <div class="flex items-center justify-between">
                <div class="flex items-center space-x-2">
                  <span :class="[
                    'px-2 py-1 text-xs font-medium rounded-full',
                    getStatusColor(job.status)
                  ]">
                    {{ job.status }}
                  </span>
                  <span class="text-sm font-medium text-gray-900">Job #{{ job.id }}</span>
                </div>
                <span class="text-xs text-gray-500">{{ job.started_at ? formatRelativeTime(job.started_at) : 'Pending' }}</span>
              </div>
              <div v-if="job.total_items > 0" class="mt-2">
                <div class="flex items-center justify-between text-xs text-gray-500 mb-1">
                  <span>Progress</span>
                  <span>{{ job.completed_items }}/{{ job.total_items }}</span>
                </div>
                <div class="w-full bg-gray-200 rounded-full h-1.5">
                  <div
                    class="bg-indigo-600 h-1.5 rounded-full"
                    :style="{ width: `${(job.completed_items / job.total_items) * 100}%` }"
                  />
                </div>
              </div>
            </li>
          </ul>
          <router-link
            to="/admin/jobs"
            class="mt-4 block text-center text-sm text-indigo-600 hover:text-indigo-800"
          >
            View all jobs →
          </router-link>
        </div>
      </div>
    </div>

    <!-- Quick Actions -->
    <div class="mt-6 bg-white rounded-lg shadow">
      <div class="px-6 py-4 border-b border-gray-200">
        <h2 class="text-lg font-medium text-gray-900">Quick Actions</h2>
      </div>
      <div class="p-6">
        <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
          <router-link
            to="/admin/providers"
            class="flex items-center p-4 border border-gray-200 rounded-lg hover:border-indigo-500 hover:bg-indigo-50 transition-colors"
          >
            <svg class="h-8 w-8 text-indigo-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 6v6m0 0v6m0-6h6m-6 0H6" />
            </svg>
            <div class="ml-4">
              <p class="text-sm font-medium text-gray-900">Add Provider</p>
              <p class="text-xs text-gray-500">Upload HCL or add manually</p>
            </div>
          </router-link>

          <button
            @click="triggerBackup"
            :disabled="backingUp"
            class="flex items-center p-4 border border-gray-200 rounded-lg hover:border-indigo-500 hover:bg-indigo-50 transition-colors disabled:opacity-50 disabled:cursor-not-allowed text-left"
          >
            <svg class="h-8 w-8 text-indigo-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 7H5a2 2 0 00-2 2v9a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-3m-1 4l-3 3m0 0l-3-3m3 3V4" />
            </svg>
            <div class="ml-4">
              <p class="text-sm font-medium text-gray-900">
                {{ backingUp ? 'Backing up...' : 'Trigger Backup' }}
              </p>
              <p class="text-xs text-gray-500">Create database backup</p>
            </div>
          </button>

          <router-link
            to="/admin/settings"
            class="flex items-center p-4 border border-gray-200 rounded-lg hover:border-indigo-500 hover:bg-indigo-50 transition-colors"
          >
            <svg class="h-8 w-8 text-indigo-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
            </svg>
            <div class="ml-4">
              <p class="text-sm font-medium text-gray-900">Settings</p>
              <p class="text-xs text-gray-500">View configuration</p>
            </div>
          </router-link>
        </div>
      </div>
    </div>
  </AdminLayout>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import AdminLayout from '@/layouts/AdminLayout.vue'
import { useStatsStore, useJobsStore } from '@/stores'
import type { Job } from '@/types'

const statsStore = useStatsStore()
const jobsStore = useJobsStore()
const backingUp = ref(false)

const activeJobCount = computed(() => {
  return jobsStore.jobs.filter(j => j.status === 'running' || j.status === 'pending').length
})

const activeJobs = computed(() => {
  return jobsStore.jobs
    .filter(j => j.status === 'running' || j.status === 'pending')
    .slice(0, 5)
})

const recentLogs = computed(() => {
  return statsStore.auditLogs.slice(0, 5)
})

function getStatusColor(status: Job['status']): string {
  const colors: Record<string, string> = {
    pending: 'bg-yellow-100 text-yellow-800',
    running: 'bg-blue-100 text-blue-800',
    completed: 'bg-green-100 text-green-800',
    failed: 'bg-red-100 text-red-800',
  }
  return colors[status] || 'bg-gray-100 text-gray-800'
}

function getActionColor(action: string): string {
  if (action.includes('create') || action.includes('add')) return 'bg-green-400'
  if (action.includes('delete') || action.includes('remove')) return 'bg-red-400'
  if (action.includes('update') || action.includes('modify')) return 'bg-yellow-400'
  return 'bg-blue-400'
}

function formatRelativeTime(timestamp: string): string {
  const date = new Date(timestamp)
  const now = new Date()
  const diff = now.getTime() - date.getTime()
  
  const minutes = Math.floor(diff / 60000)
  const hours = Math.floor(diff / 3600000)
  const days = Math.floor(diff / 86400000)
  
  if (minutes < 1) return 'just now'
  if (minutes < 60) return `${minutes}m ago`
  if (hours < 24) return `${hours}h ago`
  return `${days}d ago`
}

async function triggerBackup() {
  backingUp.value = true
  try {
    await statsStore.triggerBackup()
  } finally {
    backingUp.value = false
  }
}

onMounted(async () => {
  await Promise.all([
    statsStore.fetchStorageStats(),
    statsStore.fetchAuditLogs({ limit: 5 }),
    jobsStore.fetchJobs()
  ])
})
</script>
