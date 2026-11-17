<script setup lang="ts">
import DefaultTheme from 'vitepress/theme'
import VersionBadge from './components/VersionBadge.vue'

const { Layout: DefaultLayout } = DefaultTheme
</script>

<template>
  <DefaultLayout>
    <template #nav-bar-title-after>
      <VersionBadge />
    </template>
  </DefaultLayout>
</template>
