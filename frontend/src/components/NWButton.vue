<script setup lang="ts">

// 通用按钮组件：支持 primary/default/text/danger 四种形态与三档尺寸，状态样式挂接全局五态体系。
import { computed } from 'vue'

type NWButtonType = 'primary' | 'default' | 'text' | 'danger' | 'danger-text'
type NWButtonSize = 'large' | 'default' | 'small'

const props = withDefaults(
  defineProps<{
    type?: NWButtonType
    size?: NWButtonSize
    loading?: boolean
    disabled?: boolean
  }>(),
  {
    type: 'default',
    size: 'default',
    loading: false,
    disabled: false,
  },
)

const emit = defineEmits<{ click: [payload: MouseEvent] }>()

const buttonClassList = computed(() => [
  `nw-button--${props.type}`,
  `nw-button--${props.size}`,
  {
    'nw-button--loading': props.loading,
    'nw-button--disabled': props.disabled,
  },
])

function handleClick(payload: MouseEvent): void {
  if (props.loading || props.disabled) {
    return
  }
  emit('click', payload)
}
</script>

<template>
  <button
    type="button"
    class="nw-button"
    :class="buttonClassList"
    :disabled="disabled || loading"
    @click="handleClick"
  >
    <span v-if="loading" class="nw-button__spinner" aria-hidden="true" />
    <span class="nw-button__content"><slot /></span>
  </button>
</template>
