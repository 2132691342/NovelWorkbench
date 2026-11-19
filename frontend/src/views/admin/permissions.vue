<!--
  权限目录页：只读展示后端已登记的全部权限码（来自菜单表，菜单管理里登记即生效）。
  分组左侧导航 + 右侧权限表，便于对照角色勾选。
-->
<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { fetchPermCatalog } from '@/api/menu'
import type { NWPermGroupVo } from '@/api/contracts'
import NWCard from '@/components/NWCard.vue'
import NWPageHeader from '@/components/NWPageHeader.vue'
import NWEmpty from '@/components/NWEmpty.vue'
import NWLoading from '@/components/NWLoading.vue'

const { t } = useI18n()

const groupList = ref<NWPermGroupVo[]>([])
const listLoading = ref(false)
const activeGroupCode = ref('')

/** 当前分组（未选中时取首个分组） */
const activeGroup = computed(
  () => groupList.value.find((groupItem) => groupItem.group_code === activeGroupCode.value) ?? groupList.value[0] ?? null,
)

/** 全部权限条数 */
const totalPermCount = computed(() =>
  groupList.value.reduce((sumCount, groupItem) => sumCount + groupItem.perm_list.length, 0),
)

async function loadPermCatalog(): Promise<void> {
  listLoading.value = true
  try {
    groupList.value = await fetchPermCatalog()
    if (groupList.value.length > 0 && !activeGroupCode.value) {
      activeGroupCode.value = groupList.value[0].group_code
    }
  } catch {
    // 失败提示由 http 层统一弹出
  } finally {
    listLoading.value = false
  }
}

onMounted(() => {
  void loadPermCatalog()
})
</script>

<template>
  <div class="nw-page admin-perms-page">
    <NWPageHeader
      :title_text="t('admin.perms.title')"
      :subtitle_text="t('admin.perms.pageSubtitle')"
      icon_code="key"
    >
      <template #badge>
        <span class="nw-badge nw-badge--primary">{{ t('admin.perms.totalCount', { count: totalPermCount }) }}</span>
      </template>
    </NWPageHeader>

    <NWCard :hoverable="false">
      <p class="admin-perms-page__hint">{{ t('admin.perms.hint') }}</p>

      <NWLoading v-if="listLoading" />
      <div v-else-if="groupList.length" class="admin-perms-page__body">
        <aside class="admin-perms-page__aside">
          <button
            v-for="groupItem in groupList"
            :key="groupItem.group_code"
            type="button"
            class="admin-perms-page__group"
            :class="{ 'is-active': groupItem.group_code === activeGroup?.group_code }"
            @click="activeGroupCode = groupItem.group_code"
          >
            <span>{{ groupItem.group_name }}</span>
            <span class="admin-perms-page__group-count">{{ groupItem.perm_list.length }}</span>
          </button>
        </aside>

        <div class="admin-perms-page__content">
          <el-table :data="activeGroup?.perm_list ?? []">
            <el-table-column prop="perm_code" :label="t('admin.perms.permCode')" width="230">
              <template #default="{ row }">
                <code class="admin-perms-page__code">{{ row.perm_code }}</code>
              </template>
            </el-table-column>
            <el-table-column prop="perm_name" :label="t('admin.perms.permName')" width="180" />
            <el-table-column prop="perm_desc" :label="t('admin.perms.permDesc')" min-width="240" />
            <el-table-column :label="t('admin.perms.builtinFlag')" width="100">
              <template #default="{ row }">
                <el-tag size="small" :type="row.builtin_flag ? 'warning' : 'info'">
                  {{ row.builtin_flag ? t('admin.perms.builtinTag') : t('admin.perms.customTag') }}
                </el-tag>
              </template>
            </el-table-column>
          </el-table>
        </div>
      </div>
      <NWEmpty v-else :description_text="t('admin.perms.emptyTip')" />
    </NWCard>
  </div>
</template>

<style scoped>
.admin-perms-page__count {
  color: var(--text-secondary);
  font-size: 12.5px;
}

.admin-perms-page__hint {
  margin: 0 0 16px;
  color: var(--text-secondary);
  font-size: 12.5px;
  line-height: 1.7;
}

.admin-perms-page__body {
  display: flex;
  gap: 16px;
  align-items: flex-start;
}

.admin-perms-page__aside {
  display: flex;
  flex-direction: column;
  gap: 4px;
  width: 180px;
  flex-shrink: 0;
}

.admin-perms-page__group {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 9px 12px;
  border: none;
  border-radius: var(--radius-base);
  background-color: transparent;
  color: var(--text-secondary);
  font-size: 13.5px;
  cursor: pointer;
  transition: background-color var(--transition-base), color var(--transition-base),
    transform var(--transition-base);
}

.admin-perms-page__group:hover {
  background-color: var(--primary-light);
  color: var(--primary);
  transform: translateX(2px);
}

.admin-perms-page__group.is-active {
  background-color: var(--primary);
  color: var(--text-inverse);
}

.admin-perms-page__group-count {
  font-size: 12px;
  opacity: 0.75;
}

.admin-perms-page__content {
  flex: 1;
  min-width: 0;
}

.admin-perms-page__code {
  padding: 2px 6px;
  border-radius: 4px;
  background-color: var(--bg-hover);
  font-size: 12.5px;
}
</style>
