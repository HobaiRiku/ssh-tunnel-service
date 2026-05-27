<script setup lang="ts">
import { NConfigProvider, NLayout, NLayoutSider, NLayoutContent, NMenu, NMessageProvider, darkTheme, useOsTheme } from 'naive-ui'
import { RouterView, RouterLink, useRoute } from 'vue-router'
import { computed, h } from 'vue'
import type { MenuOption } from 'naive-ui'

const osTheme = useOsTheme()
const theme = computed(() => osTheme.value === 'dark' ? darkTheme : null)
const route = useRoute()

const menuOptions: MenuOption[] = [
  {
    label: () => h(RouterLink, { to: '/' }, { default: () => '🗺 Topology' }),
    key: '/'
  },
  {
    label: () => h(RouterLink, { to: '/tunnels' }, { default: () => '🔌 Tunnels' }),
    key: '/tunnels'
  },
  {
    label: () => h(RouterLink, { to: '/remotes' }, { default: () => '🖥 Remotes' }),
    key: '/remotes'
  }
]

const activeMenu = computed(() => route.path)
</script>

<template>
  <n-config-provider :theme="theme">
    <n-message-provider>
      <n-layout has-sider style="height: 100vh">
        <n-layout-sider
          bordered
          :width="200"
          :native-scrollbar="false"
          content-style="padding: 16px 0"
        >
          <div style="padding: 16px; font-weight: 700; font-size: 15px; letter-spacing: 0.02em; white-space: nowrap; overflow: hidden; text-overflow: ellipsis">
            SSH Tunnel Service
          </div>
          <n-menu
            :options="menuOptions"
            :value="activeMenu"
          />
        </n-layout-sider>
        <n-layout-content content-style="padding: 0">
          <RouterView />
        </n-layout-content>
      </n-layout>
    </n-message-provider>
  </n-config-provider>
</template>

<style>
* { box-sizing: border-box; margin: 0; padding: 0; }
body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif; }
</style>
