import { createRouter, createWebHistory } from 'vue-router'
import Tunnels from '@/views/Tunnels.vue'
import Remotes from '@/views/Remotes.vue'
import Keys from '@/views/Keys.vue'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    { path: '/', redirect: '/tunnels' },
    { path: '/tunnels', component: Tunnels },
    { path: '/remotes', component: Remotes },
    { path: '/keys', component: Keys },
  ],
})

export default router
