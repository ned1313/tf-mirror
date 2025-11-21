<template>
  <div class="min-h-screen bg-gray-50">
    <header class="bg-white shadow">
      <div class="max-w-7xl mx-auto py-6 px-4 sm:px-6 lg:px-8 flex justify-between items-center">
        <h1 class="text-3xl font-bold text-gray-900">Admin Dashboard</h1>
        <button
          @click="handleLogout"
          class="text-red-600 hover:text-red-800 font-medium"
        >
          Logout
        </button>
      </div>
    </header>
    <main>
      <div class="max-w-7xl mx-auto py-6 sm:px-6 lg:px-8">
        <div class="px-4 py-6 sm:px-0">
          <div class="bg-white p-6 rounded-lg shadow mb-6">
            <h2 class="text-xl font-semibold mb-4">Welcome, Admin</h2>
            <p class="text-gray-600">Admin dashboard will be implemented in Phase 1.</p>
          </div>
          
          <div class="grid grid-cols-1 md:grid-cols-3 gap-6">
            <div class="bg-white p-6 rounded-lg shadow">
              <h3 class="text-lg font-semibold mb-2">Providers</h3>
              <p class="text-3xl font-bold text-blue-600">0</p>
              <p class="text-sm text-gray-600">Cached providers</p>
            </div>
            
            <div class="bg-white p-6 rounded-lg shadow">
              <h3 class="text-lg font-semibold mb-2">Storage</h3>
              <p class="text-3xl font-bold text-green-600">0 GB</p>
              <p class="text-sm text-gray-600">Used storage</p>
            </div>
            
            <div class="bg-white p-6 rounded-lg shadow">
              <h3 class="text-lg font-semibold mb-2">Jobs</h3>
              <p class="text-3xl font-bold text-purple-600">0</p>
              <p class="text-sm text-gray-600">Active jobs</p>
            </div>
          </div>
        </div>
      </div>
    </main>
  </div>
</template>

<script setup lang="ts">
import { useRouter } from 'vue-router'

const router = useRouter()

const handleLogout = () => {
  localStorage.removeItem('auth_token')
  router.push('/login')
}
</script>
