<script setup lang="ts">
import { NConfigProvider, NMessageProvider, NSelect, darkTheme, useOsTheme } from 'naive-ui'
import { RouterLink, useRoute } from 'vue-router'
import { computed } from 'vue'
import { useI18n, type Locale } from '@/i18n'

const osTheme = useOsTheme()
const theme = computed(() => osTheme.value === 'dark' ? darkTheme : null)
const route = useRoute()
const { locale, setLocale, t } = useI18n()

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
            <svg class="brand-icon" viewBox="0 0 32 32" fill="none" xmlns="http://www.w3.org/2000/svg">
              <rect x="1" y="5" width="13" height="9" rx="2" fill="#2563eb"/>
              <rect x="2" y="6" width="11" height="7" rx="1.5" fill="#3b82f6"/>
              <circle cx="11" cy="9.5" r="1" fill="#bfdbfe"/>
              <rect x="18" y="18" width="13" height="9" rx="2" fill="#1d4ed8"/>
              <rect x="19" y="19" width="11" height="7" rx="1.5" fill="#2563eb"/>
              <circle cx="28" cy="22.5" r="1" fill="#bfdbfe"/>
              <path d="M14 9.5 Q20 9.5 20 16 Q20 22.5 18 22.5" stroke="#60a5fa" stroke-width="1.5" fill="none" stroke-linecap="round"/>
              <circle cx="14" cy="9.5" r="1.5" fill="#60a5fa"/>
              <circle cx="18" cy="22.5" r="1.5" fill="#60a5fa"/>
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
.locale-switcher {
  margin-left: auto;
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
