<script setup lang="ts">
defineProps<{
  data: {
    label: string
    host: string
    port: number
    user?: string
  }
}>()
</script>

<template>
  <div class="remote-group">
    <div class="group-header">
      <svg class="icon" viewBox="0 0 20 20" fill="currentColor">
        <rect x="1" y="2" width="18" height="6" rx="1.5"/>
        <circle cx="16" cy="5" r="1" fill="rgba(255,255,255,0.7)"/>
        <rect x="1" y="11" width="18" height="6" rx="1.5"/>
        <circle cx="16" cy="14" r="1" fill="rgba(255,255,255,0.7)"/>
      </svg>
      <div class="header-text">
        <span class="name">{{ data.label }}</span>
        <span class="conn">{{ data.user ? data.user + '@' : '' }}{{ data.host }}:{{ data.port }}</span>
      </div>
    </div>
    <!-- tunnel child nodes rendered by Vue Flow below the header -->
  </div>
</template>

<style scoped>
.remote-group {
  width: 100%;
  height: 100%;
  background: var(--color-node-remote-bg);
  border: 2px solid var(--color-node-remote-border);
  border-radius: 10px;
  overflow: visible;
}

.group-header {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  background: linear-gradient(135deg, var(--color-accent-strong), var(--color-accent));
  border-radius: 8px 8px 0 0;
  color: white;
  height: 52px;
}

.icon { width: 18px; height: 18px; flex-shrink: 0; }

.header-text {
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.name {
  font-weight: 600;
  font-size: 13px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  line-height: 1.3;
}

.conn {
  font-size: 10px;
  opacity: 0.8;
  font-family: 'SF Mono', 'Fira Code', monospace;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  line-height: 1.3;
}
</style>
