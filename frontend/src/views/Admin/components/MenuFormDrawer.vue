<!-- 菜单编辑抽屉：上级树选择、类型联动禁用、权限码唯一性提示。 -->
<script setup lang="ts">
  import { computed, reactive, ref } from 'vue'
  import {
    ElButton,
    ElDrawer,
    ElForm,
    ElFormItem,
    ElInput,
    ElInputNumber,
    ElMessage,
    ElOption,
    ElSelect,
    ElSwitch,
    ElTreeSelect,
    type FormInstance,
    type FormRules
  } from 'element-plus'
  import { saveMenu, type NWMenuSaveReqVo } from '@/api/menu'
  import type { NWMenuVo } from '@/api/contracts'

  interface ParentNode {
    value: string
    label: string
    children?: ParentNode[]
  }

  const props = defineProps<{ menuList: NWMenuVo[] }>()
  const emit = defineEmits<{ saved: [] }>()

  const visible = ref(false)
  const saving = ref(false)
  const formRef = ref<FormInstance>()
  const editing = ref<NWMenuVo | null>(null)

  const form = reactive({
    parent_ulid: '',
    menu_code: '',
    menu_name: '',
    menu_type_code: 'page',
    scope_code: 'global',
    route_path: '',
    component_path: '',
    icon_code: '',
    perm_code: '',
    visible_flag: true,
    enabled_flag: true,
    sort_order: 0,
    remark_text: ''
  })

  const title = computed(() => (editing.value ? '编辑菜单' : '新建菜单'))
  const isGroup = computed(() => form.menu_type_code === 'group')
  const isAction = computed(() => form.menu_type_code === 'action')

  const typeOptions = [
    { value: 'group', label: '分组' },
    { value: 'page', label: '页面' },
    { value: 'action', label: '动作' }
  ]

  const scopeOptions = [
    { value: 'global', label: '全局' },
    { value: 'project', label: '项目内' }
  ]

  /** 可选上级树：排除自身与其后代，从结构上杜绝成环 */
  const parentTree = computed<ParentNode[]>(() => {
    const excluded = new Set<string>()
    const selfUlid = editing.value?.menu_ulid
    if (selfUlid) {
      excluded.add(selfUlid)
      const collect = (parentUlid: string) => {
        props.menuList.forEach((item) => {
          if (item.parent_ulid === parentUlid && !excluded.has(item.menu_ulid)) {
            excluded.add(item.menu_ulid)
            collect(item.menu_ulid)
          }
        })
      }
      collect(selfUlid)
    }
    const childrenOf = (parentUlid: string): ParentNode[] =>
      props.menuList
        .filter((item) => (item.parent_ulid || '') === parentUlid && !excluded.has(item.menu_ulid))
        .sort((a, b) => a.sort_order - b.sort_order)
        .map((item) => {
          const children = childrenOf(item.menu_ulid)
          return {
            value: item.menu_ulid,
            label: `${item.menu_name}（${item.menu_code}）`,
            ...(children.length > 0 ? { children } : {})
          }
        })
    return childrenOf('')
  })

  /** 权限码冲突提示：同库中已被其他菜单登记（后端也会拒绝，这里提前告知） */
  const permCodeConflict = computed(() => {
    const permCode = form.perm_code.trim()
    if (!permCode) return ''
    const hit = props.menuList.find(
      (item) => item.perm_code === permCode && item.menu_ulid !== editing.value?.menu_ulid
    )
    return hit ? `该权限码已被菜单「${hit.menu_name}」登记` : ''
  })

  const rules = computed<FormRules>(() => ({
    menu_code: [
      { required: true, message: '请输入菜单编码', trigger: 'blur' },
      { min: 2, max: 64, message: '长度 2-64 个字符', trigger: 'blur' }
    ],
    menu_name: [{ required: true, message: '请输入菜单名称', trigger: 'blur' }],
    menu_type_code: [{ required: true, message: '请选择菜单类型', trigger: 'change' }],
    perm_code: [
      {
        validator: (_rule, value: string, callback: (error?: Error) => void) => {
          if (value && permCodeConflict.value) {
            callback(new Error(permCodeConflict.value))
            return
          }
          callback()
        },
        trigger: 'blur'
      }
    ]
  }))

  /** 打开抽屉：传菜单为编辑，不传为新建 */
  const open = (menu?: NWMenuVo) => {
    editing.value = menu ?? null
    form.parent_ulid = menu?.parent_ulid ?? ''
    form.menu_code = menu?.menu_code ?? ''
    form.menu_name = menu?.menu_name ?? ''
    form.menu_type_code = menu?.menu_type_code ?? 'page'
    form.scope_code = menu?.scope_code || 'global'
    form.route_path = menu?.route_path ?? ''
    form.component_path = menu?.component_path ?? ''
    form.icon_code = menu?.icon_code ?? ''
    form.perm_code = menu?.perm_code ?? ''
    form.visible_flag = menu?.visible_flag ?? true
    form.enabled_flag = menu?.enabled_flag ?? true
    form.sort_order = menu?.sort_order ?? 0
    form.remark_text = menu?.remark_text ?? ''
    visible.value = true
    formRef.value?.clearValidate()
  }

  const submit = async () => {
    const valid = await formRef.value?.validate().catch(() => false)
    if (!valid) return

    saving.value = true
    try {
      const payload: NWMenuSaveReqVo = {
        menu_code: form.menu_code,
        menu_name: form.menu_name,
        menu_type_code: form.menu_type_code,
        scope_code: form.scope_code,
        // 分组不承载路由与组件：类型切换后残留的值必须显式清空，否则后端会拒绝
        route_path: isGroup.value ? '' : form.route_path,
        component_path: isGroup.value ? '' : form.component_path,
        icon_code: form.icon_code,
        perm_code: form.perm_code,
        visible_flag: isAction.value ? false : form.visible_flag,
        enabled_flag: form.enabled_flag,
        sort_order: form.sort_order,
        remark_text: form.remark_text
      }
      if (form.parent_ulid) payload.parent_ulid = form.parent_ulid
      if (editing.value) payload.menu_ulid = editing.value.menu_ulid
      await saveMenu(payload)
      ElMessage.success(editing.value ? '菜单已更新' : '菜单已创建')
      visible.value = false
      emit('saved')
    } finally {
      saving.value = false
    }
  }

  defineExpose({ open })
</script>

<template>
  <ElDrawer v-model="visible" :title="title" size="560px" append-to-body destroy-on-close>
    <ElForm ref="formRef" :model="form" :rules="rules" label-position="top">
      <div class="menu-form__row">
        <ElFormItem label="上级菜单">
          <ElTreeSelect
            v-model="form.parent_ulid"
            :data="parentTree"
            check-strictly
            clearable
            :render-after-expand="false"
            placeholder="不选为根级"
            class="menu-form__field"
          />
        </ElFormItem>
        <ElFormItem label="菜单类型" prop="menu_type_code">
          <ElSelect v-model="form.menu_type_code" class="menu-form__field">
            <ElOption
              v-for="item in typeOptions"
              :key="item.value"
              :label="item.label"
              :value="item.value"
            />
          </ElSelect>
        </ElFormItem>
      </div>

      <div class="menu-form__row">
        <ElFormItem label="菜单编码" prop="menu_code">
          <ElInput v-model="form.menu_code" maxlength="64" placeholder="例如 admin_users" />
        </ElFormItem>
        <ElFormItem label="菜单名称" prop="menu_name">
          <ElInput v-model="form.menu_name" maxlength="64" placeholder="例如 用户管理" />
        </ElFormItem>
      </div>

      <div class="menu-form__row">
        <ElFormItem label="作用域">
          <ElSelect v-model="form.scope_code" class="menu-form__field">
            <ElOption
              v-for="item in scopeOptions"
              :key="item.value"
              :label="item.label"
              :value="item.value"
            />
          </ElSelect>
        </ElFormItem>
        <ElFormItem label="排序号">
          <ElInputNumber
            v-model="form.sort_order"
            :min="0"
            :max="9999"
            controls-position="right"
            class="menu-form__field"
          />
        </ElFormItem>
      </div>

      <ElFormItem label="路由路径">
        <ElInput
          v-model="form.route_path"
          :disabled="isGroup"
          maxlength="128"
          placeholder="例如 /admin/users"
        />
        <p class="menu-form__tip">
          {{ isGroup ? '分组只用于组织下级，不承载路由。' : '项目内菜单的路由以 :project_ulid 结尾。' }}
        </p>
      </ElFormItem>

      <ElFormItem label="组件路径">
        <ElInput
          v-model="form.component_path"
          :disabled="isGroup"
          maxlength="128"
          placeholder="例如 views/Admin/Users"
        />
        <p class="menu-form__tip">留空表示该菜单只注册路由或纯分组，不加载页面组件。</p>
      </ElFormItem>

      <div class="menu-form__row">
        <ElFormItem label="图标编码">
          <ElInput v-model="form.icon_code" maxlength="32" placeholder="例如 mdi:account-group" />
        </ElFormItem>
        <ElFormItem label="权限码" prop="perm_code">
          <ElInput v-model="form.perm_code" maxlength="64" placeholder="留空表示登录即可见" />
        </ElFormItem>
      </div>
      <p class="menu-form__tip menu-form__tip--block">
        权限码全局唯一：同一权限码只能被一行菜单登记，角色授权按它判定访问。
        <span v-if="permCodeConflict" class="menu-form__conflict">{{ permCodeConflict }}</span>
      </p>

      <div class="menu-form__row">
        <ElFormItem label="导航展示">
          <ElSwitch v-model="form.visible_flag" :disabled="isAction" />
        </ElFormItem>
        <ElFormItem label="启用">
          <ElSwitch v-model="form.enabled_flag" />
        </ElFormItem>
      </div>
      <p class="menu-form__tip menu-form__tip--block">
        动作节点不进导航；停用菜单会同时把其权限码从所有角色中剔除。
      </p>

      <ElFormItem label="备注">
        <ElInput v-model="form.remark_text" type="textarea" :rows="2" maxlength="256" />
      </ElFormItem>
    </ElForm>

    <template #footer>
      <ElButton @click="visible = false">取消</ElButton>
      <ElButton type="primary" :loading="saving" @click="submit">保存</ElButton>
    </template>
  </ElDrawer>
</template>

<style lang="less" scoped>
  .menu-form {
    &__row {
      display: grid;
      gap: 12px;
      grid-template-columns: 1fr 1fr;
    }

    &__field {
      width: 100%;
    }

    &__tip {
      margin-top: 4px;
      font-size: 11.5px;
      line-height: 1.6;
      color: var(--el-text-color-secondary);

      &--block {
        margin: -6px 0 14px;
      }
    }

    &__conflict {
      margin-left: 4px;
      color: var(--nw-danger);
    }
  }
</style>
