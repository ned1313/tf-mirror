import { createRouter, createWebHistory, type RouteLocationNormalized, type NavigationGuardNext } from 'vue-router'
import Home from '../views/Home.vue'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: '/',
      name: 'home',
      component: Home
    },
    {
      path: '/login',
      name: 'login',
      component: () => import('../views/Login.vue')
    },
    {
      path: '/admin',
      name: 'admin',
      component: () => import('../views/Admin.vue'),
      meta: { requiresAuth: true }
    },
    {
      path: '/admin/providers',
      name: 'admin-providers',
      component: () => import('../views/Providers.vue'),
      meta: { requiresAuth: true }
    },
    {
      path: '/admin/jobs',
      name: 'admin-jobs',
      component: () => import('../views/Jobs.vue'),
      meta: { requiresAuth: true }
    },
    {
      path: '/admin/audit',
      name: 'admin-audit',
      component: () => import('../views/AuditLogs.vue'),
      meta: { requiresAuth: true }
    },
    {
      path: '/admin/settings',
      name: 'admin-settings',
      component: () => import('../views/Settings.vue'),
      meta: { requiresAuth: true }
    },
    // Redirect old providers route to admin providers
    {
      path: '/providers',
      redirect: '/admin/providers'
    }
  ]
})

router.beforeEach((to: RouteLocationNormalized, _from: RouteLocationNormalized, next: NavigationGuardNext) => {
  const token = localStorage.getItem('auth_token')
  
  if (to.meta.requiresAuth && !token) {
    next({ name: 'login', query: { redirect: to.fullPath } })
  } else {
    next()
  }
})

export default router
