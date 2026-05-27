<script setup lang="ts">
import { Handle, Position } from '@vue-flow/core'

const props = defineProps<{
  data: {
    label: string
  }
}>()
</script>

<template>
  <div class="target-node">
    <div class="node-header">
      <span class="icon">📍</span>
      <span class="label">{{ data.label }}</span>
    </div>
    <Handle type="target" :position="Position.Left" />
  </div>
</template>

<style scoped>
.target-node {
  border: 2px solid #7c3aed;
  border-radius: 8px;
  min-width: 140px;
  font-size: 12px;
  overflow: hidden;
  background: #f5f3ff20;
}
.node-header {
  background: #7c3aed;
  color: white;
  padding: 6px 10px;
  display: flex;
  align-items: center;
  gap: 6px;
  font-weight: 600;
  font-size: 13px;
}
</style>
