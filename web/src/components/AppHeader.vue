<template>
  <header class="bg-white shadow-sm border-b border-gray-200">
    <div class="flex items-center justify-between h-16 px-4 sm:px-6 lg:px-8">
      <!-- Logo and title -->
      <div class="flex items-center">
        <router-link to="/" class="flex items-center">
          <svg class="h-8 w-8 text-indigo-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" 
              d="M5 12h14M12 5l7 7-7 7" />
          </svg>
          <span class="ml-2 text-xl font-bold text-gray-900">Terraform Mirror</span>
        </router-link>
      </div>

      <!-- Right side -->
      <div class="flex items-center space-x-4">
        <!-- Processor status indicator -->
        <div v-if="processorStatus" class="hidden sm:flex items-center">
          <span 
            :class="[
              'h-2 w-2 rounded-full mr-2',
              processorStatus.running ? 'bg-green-400' : 'bg-red-400'
            ]"
          ></span>
          <span class="text-sm text-gray-500">
            {{ processorStatus.running ? 'Processor Active' : 'Processor Stopped' }}
          </span>
        </div>

        <!-- User menu -->
        <div v-if="isAuthenticated" class="relative">
          <div class="flex items-center space-x-3">
            <span class="text-sm text-gray-700">{{ username }}</span>
            <button
              @click="handleLogout"
              class="text-sm text-red-600 hover:text-red-800 font-medium"
            >
              Logout
            </button>
          </div>
        </div>

        <!-- Login button -->
        <router-link
          v-else
          to="/login"
          class="text-sm font-medium text-indigo-600 hover:text-indigo-500"
        >
          Login
        </router-link>
      </div>
    </div>
  </header>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore, useStatsStore } from '@/stores'

const router = useRouter()
const authStore = useAuthStore()
const statsStore = useStatsStore()

const isAuthenticated = computed(() => authStore.isAuthenticated)
const username = computed(() => authStore.username)
const processorStatus = computed(() => statsStore.processorStatus)

onMounted(() => {
  if (isAuthenticated.value) {
    statsStore.fetchProcessorStatus()
  }
})

async function handleLogout() {
  await authStore.logout()
  router.push('/login')
}
</script>
