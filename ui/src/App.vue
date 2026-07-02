<script setup lang="ts">
import { NConfigProvider, NMessageProvider, NSelect, darkTheme, useOsTheme } from 'naive-ui'
import { RouterLink, useRoute } from 'vue-router'
import { computed, watchEffect } from 'vue'
import { useI18n, type Locale } from '@/i18n'
import InstanceBadge from '@/components/InstanceBadge.vue'

const osTheme = useOsTheme()
const isDark = computed(() => osTheme.value === 'dark')
const theme = computed(() => isDark.value ? darkTheme : null)
const route = useRoute()
const { locale, setLocale, t } = useI18n()

// Custom (non-naive-ui) elements read theme colors from CSS variables toggled
// on <html>, so they stay in sync with naive-ui's own dark theme even for
// teleported content (modals, popovers) that lives outside .app-shell.
watchEffect(() => {
  document.documentElement.classList.toggle('dark', isDark.value)
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
          <InstanceBadge />
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
:root {
  --color-bg: #f1f5f9;
  --color-surface: #ffffff;
  --color-surface-alt: #f8fafc;
  --color-surface-hover: #f1f5f9;
  --color-border: #e2e8f0;
  --color-border-strong: #cbd5e1;
  --color-text: #1e293b;
  --color-text-secondary: #475569;
  --color-text-tertiary: #64748b;
  --color-text-muted: #94a3b8;
  --color-accent: #2563eb;
  --color-accent-strong: #1d4ed8;
  --color-accent-bg: #eff6ff;
  --color-accent-border: #bfdbfe;
  --color-danger: #dc2626;
  --color-shadow-soft: rgba(0, 0, 0, 0.06);
  --color-shadow-medium: rgba(15, 23, 42, 0.14);
  --color-shadow-strong: rgba(15, 23, 42, 0.18);
  --color-overlay: rgba(255, 255, 255, 0.85);
  --color-divider-soft: rgba(15, 23, 42, 0.06);

  --color-tag-blue-bg: #dbeafe;
  --color-tag-blue-text: #1d4ed8;
  --color-tag-pink-bg: #fce7f3;
  --color-tag-pink-text: #9d174d;
  --color-tag-purple-bg: #ede9fe;
  --color-tag-purple-text: #6d28d9;

  --color-state-running-bg: #f0fdf4;
  --color-state-running-border: #22c55e;
  --color-state-error-bg: #fef2f2;
  --color-state-error-border: #ef4444;
  --color-state-stopped-bg: #f8fafc;
  --color-state-stopped-border: #cbd5e1;
  --color-state-stopped-dot: #94a3b8;

  --color-node-remote-bg: #eff6ff;
  --color-node-remote-border: rgba(37, 99, 235, 0.9);
  --color-node-target-bg: #faf5ff;
  --color-node-target-border: #7c3aed;
  --color-node-target-text: #4c1d95;
  --color-node-target-text-strong: #5b21b6;

  --color-flow-from: #f8fafc;
  --color-flow-to: #f1f5f9;
  --color-flow-grid: #e2e8f0;
}

:root.dark {
  --color-bg: #0f172a;
  --color-surface: #1e293b;
  --color-surface-alt: #253449;
  --color-surface-hover: #2c3e5c;
  --color-border: #334155;
  --color-border-strong: #475569;
  --color-text: #e2e8f0;
  --color-text-secondary: #cbd5e1;
  --color-text-tertiary: #94a3b8;
  --color-text-muted: #64748b;
  --color-accent: #3b82f6;
  --color-accent-strong: #60a5fa;
  --color-accent-bg: #1e3a5f;
  --color-accent-border: #2563eb;
  --color-danger: #f87171;
  --color-shadow-soft: rgba(0, 0, 0, 0.35);
  --color-shadow-medium: rgba(0, 0, 0, 0.5);
  --color-shadow-strong: rgba(0, 0, 0, 0.6);
  --color-overlay: rgba(30, 41, 59, 0.85);
  --color-divider-soft: rgba(255, 255, 255, 0.08);

  --color-tag-blue-bg: #1e3a5f;
  --color-tag-blue-text: #93c5fd;
  --color-tag-pink-bg: #4a1942;
  --color-tag-pink-text: #f9a8d4;
  --color-tag-purple-bg: #3b2e63;
  --color-tag-purple-text: #c4b5fd;

  --color-state-running-bg: #12261a;
  --color-state-running-border: #22c55e;
  --color-state-error-bg: #391414;
  --color-state-error-border: #ef4444;
  --color-state-stopped-bg: #1e293b;
  --color-state-stopped-border: #475569;
  --color-state-stopped-dot: #94a3b8;

  --color-node-remote-bg: #1e3a5f;
  --color-node-remote-border: rgba(96, 165, 250, 0.9);
  --color-node-target-bg: #2e2350;
  --color-node-target-text: #ddd6fe;
  --color-node-target-text-strong: #c4b5fd;
  --color-node-target-border: #a78bfa;

  --color-flow-from: #1a2436;
  --color-flow-to: #111827;
  --color-flow-grid: #334155;
}

*, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }
html, body { height: 100%; }
body {
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', 'Inter', sans-serif;
  background: var(--color-bg);
  color: var(--color-text);
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
  background: var(--color-surface);
  border-bottom: 1px solid var(--color-border);
  box-shadow: 0 1px 3px var(--color-shadow-soft);
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
.brand-name { font-weight: 700; font-size: 14px; color: var(--color-text); letter-spacing: -0.01em; white-space: nowrap; }

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
  color: var(--color-text-tertiary);
  text-decoration: none;
  border-bottom: 2px solid transparent;
  transition: color 0.15s, border-color 0.15s;
  white-space: nowrap;
}

.nav-tab:hover { color: var(--color-text-secondary); }
.nav-tab.active { color: var(--color-accent); border-bottom-color: var(--color-accent); }

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
