<!-- 404 页：提示路径不存在并提供返回入口。 -->
<script setup lang="ts">
  import Error from '@/components/Error/index.vue'
  import { useRouter } from 'vue-router'

  const { push } = useRouter()

  const errorClick = () => {
    void push('/')
  }
</script>

<template>
  <Error @error-click="errorClick" />
</template>
