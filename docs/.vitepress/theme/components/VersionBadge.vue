<script setup lang="ts">
import { useRoute } from 'vitepress'
import { computed } from 'vue'

const route = useRoute()

// Derive current version from path: /v0.1.1/, /bleeding/, /edge/ (prod), etc.
const currentVersion = computed(() => {
  const path = route.path
  const verMatch = path.match(/\/(v\d+\.\d+\.\d+)(?:\/|$)/)
  if (verMatch) return verMatch[1]
  if (path.endsWith('/bleeding') || path.endsWith('/bleeding/')) return 'Bleeding'
  if (path.endsWith('/edge') || path.endsWith('/edge/')) return 'Bleeding'
  if (path.endsWith('/latest') || path.endsWith('/latest/')) return 'Redirecting…'
  return null
})
</script>

<template>
  <span v-if="currentVersion" class="version-badge">{{ currentVersion }}</span>
</template>

<style scoped>
.version-badge {
  margin-left: 0.75rem;
  padding: 0.15rem 0.5rem;
  font-size: 0.75rem;
  font-weight: 500;
  color: var(--vp-c-brand-1);
  background: var(--vp-c-brand-soft);
  border-radius: 4px;
}
</style>
