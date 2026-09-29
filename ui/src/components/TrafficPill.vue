<script setup lang="ts">
import { computed } from 'vue'
import Sparkline from '@/components/Sparkline.vue'
import { formatBytes, formatRate, isIdleRate } from '@/format'
import { useI18n } from '@/i18n'
import { useTrafficStore } from '@/stores/traffic'

// Header readout of the service's total throughput across every tunnel.
const traffic = useTrafficStore()
const { t } = useI18n()

const title = computed(() => [
  t('traffic.totalTitle'),
  `${t('traffic.up')}: ${formatBytes(traffic.total.up_bytes)} · ${t('traffic.down')}: ${formatBytes(traffic.total.down_bytes)}`,
  t('traffic.conns', { n: traffic.total.active_conns }),
  traffic.usage ? t('traffic.todayLine', {
    down: formatBytes(traffic.usage.total.today.down_bytes),
    up: formatBytes(traffic.usage.total.today.up_bytes),
  }) : '',
].filter(Boolean).join('\n'))
</script>

<template>
  <div class="traffic-pill" :class="{ offline: !traffic.connected }" :title="traffic.connected ? title : t('traffic.offline')">
    <div class="rates">
      <span class="rate" :class="{ idle: isIdleRate(traffic.total.down_rate) }">
        <span class="arrow down" aria-hidden="true">↓</span>{{ formatRate(traffic.total.down_rate) }}
      </span>
      <span class="rate" :class="{ idle: isIdleRate(traffic.total.up_rate) }">
        <span class="arrow up" aria-hidden="true">↑</span>{{ formatRate(traffic.total.up_rate) }}
      </span>
    </div>
    <Sparkline class="spark" :points="traffic.totalHistory" :span="90" :height="22" color="var(--color-accent)" />
  </div>
</template>

<style scoped>
.traffic-pill {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 4px 10px 4px 12px;
  border-radius: 999px;
  background: var(--color-surface-alt);
  border: 1px solid var(--color-border);
  font-variant-numeric: tabular-nums;
  flex-shrink: 0;
}
.traffic-pill.offline { opacity: 0.55; }
.rates { display: flex; flex-direction: column; line-height: 1.15; min-width: 84px; }
.rate { font-size: 11.5px; font-weight: 600; color: var(--color-text); white-space: nowrap; }
.rate.idle { color: var(--color-text-muted); font-weight: 500; }
.arrow { display: inline-block; width: 12px; font-weight: 700; }
.arrow.down { color: var(--color-accent); }
.arrow.up { color: var(--color-text-tertiary); }
.spark { width: 72px; }
@media (max-width: 640px) {
  .spark { display: none; }
  .traffic-pill { padding: 3px 10px; }
}
</style>
