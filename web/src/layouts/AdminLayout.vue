<template>
  <div class="min-h-screen bg-gray-100">
    <AppHeader />
    
    <div class="flex">
      <!-- Sidebar for desktop -->
      <AppSidebar class="hidden lg:block" />
      
      <!-- Mobile sidebar overlay -->
      <Transition name="fade">
        <div
          v-if="sidebarOpen"
          class="fixed inset-0 bg-gray-600 bg-opacity-75 z-40 lg:hidden"
          @click="sidebarOpen = false"
        />
      </Transition>
      
      <!-- Mobile sidebar -->
      <Transition name="slide">
        <div
          v-if="sidebarOpen"
          class="fixed inset-y-0 left-0 z-50 lg:hidden"
        >
          <AppSidebar />
        </div>
      </Transition>
      
      <!-- Main content -->
      <main class="flex-1 p-6">
        <div class="max-w-7xl mx-auto">
          <!-- Breadcrumb -->
          <nav v-if="breadcrumbs.length > 0" class="mb-4">
            <ol class="flex items-center space-x-2 text-sm text-gray-500">
              <li v-for="(crumb, index) in breadcrumbs" :key="crumb.path">
                <div class="flex items-center">
                  <span v-if="index > 0" class="mx-2">/</span>
                  <router-link
                    v-if="index < breadcrumbs.length - 1"
                    :to="crumb.path"
                    class="hover:text-gray-700"
                  >
                    {{ crumb.name }}
                  </router-link>
                  <span v-else class="text-gray-900 font-medium">{{ crumb.name }}</span>
                </div>
              </li>
            </ol>
          </nav>
          
          <!-- Page content -->
          <slot />
        </div>
      </main>
    </div>
    
    <!-- Mobile menu button -->
    <button
      type="button"
      class="fixed bottom-4 left-4 lg:hidden z-50 bg-indigo-600 text-white p-3 rounded-full shadow-lg hover:bg-indigo-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500"
      @click="sidebarOpen = !sidebarOpen"
    >
      <svg v-if="!sidebarOpen" class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 12h16M4 18h16" />
      </svg>
      <svg v-else class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
      </svg>
    </button>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useRoute } from 'vue-router'
import AppHeader from '@/components/AppHeader.vue'
import AppSidebar from '@/components/AppSidebar.vue'

const route = useRoute()
const sidebarOpen = ref(false)

// Close sidebar on route change (mobile)
watch(() => route.path, () => {
  sidebarOpen.value = false
})

interface Breadcrumb {
  name: string
  path: string
}

const breadcrumbs = computed((): Breadcrumb[] => {
  const crumbs: Breadcrumb[] = []
  const path = route.path
  
  if (path === '/admin') {
    return []
  }
  
  crumbs.push({ name: 'Dashboard', path: '/admin' })
  
  if (path.startsWith('/admin/providers')) {
    crumbs.push({ name: 'Providers', path: '/admin/providers' })
    if (route.params.id) {
      crumbs.push({ name: route.params.id as string, path: path })
    }
  } else if (path.startsWith('/admin/jobs')) {
    crumbs.push({ name: 'Jobs', path: '/admin/jobs' })
    if (route.params.id) {
      crumbs.push({ name: `Job #${route.params.id}`, path: path })
    }
  } else if (path.startsWith('/admin/audit')) {
    crumbs.push({ name: 'Audit Logs', path: '/admin/audit' })
  } else if (path.startsWith('/admin/settings')) {
    crumbs.push({ name: 'Settings', path: '/admin/settings' })
  }
  
  return crumbs
})
</script>

<style scoped>
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.3s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}

.slide-enter-active,
.slide-leave-active {
  transition: transform 0.3s ease;
}

.slide-enter-from,
.slide-leave-to {
  transform: translateX(-100%);
}
</style>
