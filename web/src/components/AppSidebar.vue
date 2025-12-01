<template>
  <aside class="w-64 bg-gray-800 min-h-screen">
    <nav class="mt-5 px-2">
      <router-link
        v-for="item in navigation"
        :key="item.name"
        :to="item.to"
        :class="[
          isActive(item.to)
            ? 'bg-gray-900 text-white'
            : 'text-gray-300 hover:bg-gray-700 hover:text-white',
          'group flex items-center px-3 py-2 text-sm font-medium rounded-md mb-1'
        ]"
      >
        <component
          :is="item.icon"
          :class="[
            isActive(item.to) ? 'text-gray-300' : 'text-gray-400 group-hover:text-gray-300',
            'mr-3 flex-shrink-0 h-5 w-5'
          ]"
        />
        {{ item.name }}
      </router-link>
    </nav>

    <!-- Quick stats at bottom -->
    <div class="absolute bottom-0 w-64 p-4 border-t border-gray-700">
      <div class="text-gray-400 text-xs mb-2">Quick Stats</div>
      <div class="grid grid-cols-2 gap-2 text-xs">
        <div class="text-gray-300">
          <span class="text-gray-500">Providers:</span> {{ stats.providers }}
        </div>
        <div class="text-gray-300">
          <span class="text-gray-500">Storage:</span> {{ stats.storage }}
        </div>
      </div>
    </div>
  </aside>
</template>

<script setup lang="ts">
import { computed, onMounted, h } from 'vue'
import { useRoute } from 'vue-router'
import { useStatsStore } from '@/stores'

const route = useRoute()
const statsStore = useStatsStore()

// Icon components (using simple SVG inline)
const DashboardIcon = {
  render() {
    return h('svg', { fill: 'none', viewBox: '0 0 24 24', stroke: 'currentColor' }, [
      h('path', { 
        'stroke-linecap': 'round', 
        'stroke-linejoin': 'round', 
        'stroke-width': '2',
        d: 'M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-6 0a1 1 0 001-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 001 1m-6 0h6'
      })
    ])
  }
}

const ProvidersIcon = {
  render() {
    return h('svg', { fill: 'none', viewBox: '0 0 24 24', stroke: 'currentColor' }, [
      h('path', { 
        'stroke-linecap': 'round', 
        'stroke-linejoin': 'round', 
        'stroke-width': '2',
        d: 'M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10M4 7v10l8 4'
      })
    ])
  }
}

const JobsIcon = {
  render() {
    return h('svg', { fill: 'none', viewBox: '0 0 24 24', stroke: 'currentColor' }, [
      h('path', { 
        'stroke-linecap': 'round', 
        'stroke-linejoin': 'round', 
        'stroke-width': '2',
        d: 'M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-6 9l2 2 4-4'
      })
    ])
  }
}

const AuditIcon = {
  render() {
    return h('svg', { fill: 'none', viewBox: '0 0 24 24', stroke: 'currentColor' }, [
      h('path', { 
        'stroke-linecap': 'round', 
        'stroke-linejoin': 'round', 
        'stroke-width': '2',
        d: 'M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z'
      })
    ])
  }
}

const SettingsIcon = {
  render() {
    return h('svg', { fill: 'none', viewBox: '0 0 24 24', stroke: 'currentColor' }, [
      h('path', { 
        'stroke-linecap': 'round', 
        'stroke-linejoin': 'round', 
        'stroke-width': '2',
        d: 'M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z'
      }),
      h('path', { 
        'stroke-linecap': 'round', 
        'stroke-linejoin': 'round', 
        'stroke-width': '2',
        d: 'M15 12a3 3 0 11-6 0 3 3 0 016 0z'
      })
    ])
  }
}

const navigation = [
  { name: 'Dashboard', to: '/admin', icon: DashboardIcon },
  { name: 'Providers', to: '/admin/providers', icon: ProvidersIcon },
  { name: 'Jobs', to: '/admin/jobs', icon: JobsIcon },
  { name: 'Audit Logs', to: '/admin/audit', icon: AuditIcon },
  { name: 'Settings', to: '/admin/settings', icon: SettingsIcon },
]

const stats = computed(() => ({
  providers: statsStore.storageStats?.total_providers ?? '-',
  storage: statsStore.storageStats?.total_size_human ?? '-'
}))

function isActive(path: string): boolean {
  if (path === '/admin') {
    return route.path === '/admin'
  }
  return route.path.startsWith(path)
}

onMounted(() => {
  statsStore.fetchStorageStats()
})
</script>
