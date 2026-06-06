<script setup lang="ts">
import { NConfigProvider, NMessageProvider, NSelect, darkTheme, useOsTheme } from 'naive-ui'
import { RouterLink, useRoute } from 'vue-router'
import { computed, onMounted, ref } from 'vue'
import { useI18n, type Locale } from '@/i18n'
import { api, type InstanceInfo } from '@/api/client'

const osTheme = useOsTheme()
const theme = computed(() => osTheme.value === 'dark' ? darkTheme : null)
const route = useRoute()
const { locale, setLocale, t } = useI18n()

// Instance identity badge: which instance this UI is talking to, mirroring the
// CLI banner so users always know what they are operating on.
const instance = ref<InstanceInfo | null>(null)
const instanceLabel = computed(() => {
  const info = instance.value
  if (!info) return ''
  const tail = info.home ? info.home.split(/[\\/]/).filter(Boolean).pop() : ''
  const scope = info.scope || 'instance'
  return info.scope === 'custom' && tail ? `${scope} · ${tail}` : scope
})

onMounted(async () => {
  try {
    instance.value = await api.instance()
  } catch {
    instance.value = null
  }
})

const localeOptions = computed(() => [
  { label: t('app.english'), value: 'en' satisfies Locale },
  { label: t('app.chinese'), value: 'zh-CN' satisfies Locale },
])

const tabs = computed(() => [
  { label: t('app.tunnels'), to: '/tunnels' },
  { label: t('app.remotes'), to: '/remotes' },
  { label: t('app.keys'), to: '/keys' },
])

function switchLocale(next: Locale) {
  setLocale(next)
}
</script>

<template>
  <n-config-provider :theme="theme">
    <n-message-provider>
      <div class="app-shell">
        <header class="app-header">
          <div class="header-brand">
            <svg class="brand-icon" viewBox="0 0 64 64" fill="none" xmlns="http://www.w3.org/2000/svg">
              <defs>
                <linearGradient id="brandBg" x1="0" y1="0" x2="64" y2="64" gradientUnits="userSpaceOnUse">
                  <stop offset="0" stop-color="#3b82f6"/>
                  <stop offset="1" stop-color="#1d4ed8"/>
                </linearGradient>
              </defs>
              <rect x="0" y="0" width="64" height="64" rx="14" fill="url(#brandBg)"/>
              <path d="M28 25 C 28 36 36 28 36 39" stroke="#ffffff" stroke-width="3.5" fill="none" stroke-linecap="round"/>
              <circle cx="28" cy="25" r="2.7" fill="#ffffff"/>
              <circle cx="36" cy="39" r="2.7" fill="#ffffff"/>
              <rect x="10" y="14" width="20" height="13" rx="3" fill="#ffffff"/>
              <circle cx="14.5" cy="20.5" r="1.6" fill="#2563eb"/>
              <rect x="34" y="37" width="20" height="13" rx="3" fill="#ffffff"/>
              <circle cx="49.5" cy="43.5" r="1.6" fill="#2563eb"/>
            </svg>
            <span class="brand-name">{{ t('app.brand') }}</span>
          </div>
          <nav class="header-nav">
            <RouterLink
              v-for="tab in tabs"
              :key="tab.to"
              :to="tab.to"
              class="nav-tab"
              :class="{ active: route.path === tab.to }"
            >{{ tab.label }}</RouterLink>
          </nav>
          <span
            v-if="instanceLabel"
            class="instance-badge"
            :title="instance ? `${instance.address} · ${instance.home}` : ''"
          >{{ instanceLabel }}</span>
          <n-select
            class="locale-switcher"
            size="small"
            :value="locale"
            :options="localeOptions"
            :consistent-menu-width="false"
            :clearable="false"
            @update:value="switchLocale"
          />
        </header>
        <main class="app-content">
          <RouterView />
        </main>
      </div>
    </n-message-provider>
  </n-config-provider>
</template>

<style>
*, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }
html, body { height: 100%; }
body {
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', 'Inter', sans-serif;
  background: #f1f5f9;
  color: #1e293b;
}

.app-shell {
  display: flex;
  flex-direction: column;
  height: 100vh;
}

.app-header {
  display: flex;
  align-items: center;
  padding: 0 24px;
  height: 52px;
  background: #fff;
  border-bottom: 1px solid #e2e8f0;
  box-shadow: 0 1px 3px rgba(0,0,0,0.06);
  flex-shrink: 0;
  gap: 24px;
}

.header-brand {
  display: flex;
  align-items: center;
  gap: 10px;
  text-decoration: none;
  user-select: none;
}

.brand-icon { width: 28px; height: 28px; flex-shrink: 0; }
.brand-name { font-weight: 700; font-size: 14px; color: #1e293b; letter-spacing: -0.01em; white-space: nowrap; }

.header-nav {
  display: flex;
  align-items: center;
  height: 100%;
  gap: 0;
  flex: 1;
}

.nav-tab {
  display: inline-flex;
  align-items: center;
  padding: 0 18px;
  height: 52px;
  font-size: 13.5px;
  font-weight: 500;
  color: #64748b;
  text-decoration: none;
  border-bottom: 2px solid transparent;
  transition: color 0.15s, border-color 0.15s;
  white-space: nowrap;
}

.nav-tab:hover { color: #334155; }
.nav-tab.active { color: #2563eb; border-bottom-color: #2563eb; }

.instance-badge {
  display: inline-flex;
  align-items: center;
  margin-left: auto;
  padding: 3px 10px;
  font-size: 12px;
  font-weight: 600;
  color: #2563eb;
  background: #eff6ff;
  border: 1px solid #bfdbfe;
  border-radius: 999px;
  white-space: nowrap;
  user-select: none;
}

.locale-switcher {
  width: 96px;
  flex-shrink: 0;
}

.app-content {
  flex: 1;
  min-height: 0;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}
</style>
