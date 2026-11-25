// 行级动作状态：表格行内的删除/启停等操作共用，保证「正在处理的那一行」有加载态、其余行不可并发重复触发。
import { computed, ref, type Ref } from 'vue'

export interface RowActionState {
  busyFlag: Readonly<Ref<boolean>>
  isPending: (rowKey: string) => boolean
  run: (rowKey: string, action: () => Promise<void>) => Promise<void>
}

/**
 * 行级动作锁：同一时刻只允许一个行内动作在飞。
 * rowKey 取行主键（如 role_ulid），用于把加载态只落在那一行上。
 */
export const useRowAction = (): RowActionState => {
  const pendingKey = ref('')
  const busyFlag = computed(() => pendingKey.value !== '')

  const isPending = (rowKey: string) => pendingKey.value !== '' && pendingKey.value === rowKey

  const run = async (rowKey: string, action: () => Promise<void>) => {
    if (pendingKey.value !== '') return
    pendingKey.value = rowKey
    try {
      await action()
    } finally {
      pendingKey.value = ''
    }
  }

  return { busyFlag, isPending, run }
}
