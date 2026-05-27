import { createRouter, createWebHistory } from 'vue-router'
import Dashboard from '@/views/Dashboard.vue'
import Tunnels from '@/views/Tunnels.vue'
import Remotes from '@/views/Remotes.vue'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    { path: '/', component: Dashboard },
    { path: '/tunnels', component: Tunnels },
    { path: '/remotes', component: Remotes }
  ]
})

export default router
