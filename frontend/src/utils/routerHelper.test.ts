// 路由装配的回归：菜单 index 与已注册路由必须逐字相等，末尾斜杠会让点击落到 404；
// 一级菜单高亮按路由匹配链判定，项目作用域占位符不能被当作字面前缀比较。
import { describe, expect, it } from 'vitest'
import { pathResolve, routeNameOf, substituteProject } from '@/utils/routerHelper'

describe('pathResolve', () => {
  it('父路径 + 空子路径不产生末尾斜杠', () => {
    expect(pathResolve('/group_project', '')).toBe('/group_project')
    expect(pathResolve('/workspace/PRJ_1', '')).toBe('/workspace/PRJ_1')
  })

  it('绝对路径与外部链接原样返回', () => {
    expect(pathResolve('/group_project', '/dashboard')).toBe('/dashboard')
    expect(pathResolve('/', 'https://example.com/a/')).toBe('https://example.com/a/')
  })

  it('普通父子路径拼接且根路径保持单斜杠', () => {
    expect(pathResolve('/', 'dashboard')).toBe('/dashboard')
    expect(pathResolve('/admin', 'users')).toBe('/admin/users')
    expect(pathResolve('/', '')).toBe('/')
  })
})

describe('routeNameOf', () => {
  it('菜单编码转 PascalCase 作为唯一路由名', () => {
    expect(routeNameOf('admin_login_log')).toBe('AdminLoginLog')
    expect(routeNameOf('group-project')).toBe('GroupProject')
  })
})

describe('substituteProject', () => {
  it('把 :project_ulid 占位替换为当前项目，树形结构保持', () => {
    const [record] = substituteProject(
      [
        {
          path: '/group_project',
          name: 'GroupProject',
          meta: {},
          children: [{ path: '/workspace/:project_ulid', name: 'Workspace', meta: {} }]
        }
      ],
      'PRJ_01M825916VFP0AJQ683T9QPTVK'
    )
    expect(record.children?.[0].path).toBe('/workspace/PRJ_01M825916VFP0AJQ683T9QPTVK')
  })

  it('无当前项目时保留占位符', () => {
    const [record] = substituteProject(
      [{ path: '/outline/:project_ulid', name: 'Outline', meta: {} }],
      ''
    )
    expect(record.path).toBe('/outline/:project_ulid')
  })
})
