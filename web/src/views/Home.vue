<template>
  <div class="min-h-screen bg-gray-50">
    <header class="bg-white shadow">
      <div class="max-w-7xl mx-auto py-6 px-4 sm:px-6 lg:px-8">
        <h1 class="text-3xl font-bold text-gray-900">Terraform Mirror</h1>
      </div>
    </header>
    <main>
      <div class="max-w-7xl mx-auto py-6 sm:px-6 lg:px-8">
        <div class="px-4 py-6 sm:px-0">
          <div class="text-center">
            <h2 class="text-2xl font-semibold text-gray-700 mb-4">Welcome to Terraform Mirror</h2>
            <p class="text-gray-600 mb-8">A caching proxy for Terraform providers and modules</p>
            
            <div class="grid grid-cols-1 md:grid-cols-3 gap-6 max-w-5xl mx-auto">
              <router-link
                to="/providers"
                class="block p-6 bg-white border border-gray-200 rounded-lg shadow hover:bg-gray-50"
              >
                <h3 class="text-xl font-bold mb-2">Browse Providers</h3>
                <p class="text-gray-600">Search and view cached Terraform providers</p>
              </router-link>

              <router-link
                to="/modules"
                class="block p-6 bg-white border border-gray-200 rounded-lg shadow hover:bg-gray-50"
              >
                <h3 class="text-xl font-bold mb-2">Browse Modules</h3>
                <p class="text-gray-600">Search and view cached Terraform modules</p>
              </router-link>
              
              <router-link
                to="/login"
                class="block p-6 bg-white border border-gray-200 rounded-lg shadow hover:bg-gray-50"
              >
                <h3 class="text-xl font-bold mb-2">Admin Login</h3>
                <p class="text-gray-600">Manage providers, modules, and system settings</p>
              </router-link>
            </div>
            
            <div class="mt-12 text-left max-w-2xl mx-auto bg-white p-6 rounded-lg shadow">
              <h3 class="text-lg font-semibold mb-4">Quick Start</h3>
              <p class="text-gray-600 mb-4">Configure your Terraform client to use this mirror:</p>
              <pre class="bg-gray-800 text-gray-100 p-4 rounded overflow-x-auto"><code>provider_installation {
  network_mirror {
    url = "{{ mirrorUrl }}/v1/providers/"
  }
}</code></pre>
            </div>
          </div>
        </div>
      </div>
    </main>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'

const mirrorUrl = computed(() => {
  return window.location.origin
})
</script>
