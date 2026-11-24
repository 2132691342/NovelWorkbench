// 前端装配链路回归：路由路径拼接 → 路由名生成 → 项目占位替换。
// 这三者是同一次「后端导航树 → vue-router 路由表」装配的上下游：
// 拼接多一个结尾斜杠会让 strict 路由整片落到 404，路由名重复会让后者覆盖前者，
// 占位替换失败会让项目作用域页面拿不到当前项目。

import { describe, expect, it } from 'vitest'
import { pathResolve, routeNameOf, substituteProject } from '@/utils/routerHelper'

describe('路由装配链路', () => {
  it('路径拼接不产生结尾斜杠，路由名唯一，项目占位可替换', () => {
    // 环节1：路径拼接。结尾斜杠会让 strict 路由匹配失败（点击落到 404）。
    expect(pathResolve('/group_project', '')).toBe('/group_project')
    expect(pathResolve('/workspace/PRJ_1', '')).toBe('/workspace/PRJ_1')
    expect(pathResolve('/', 'dashboard')).toBe('/dashboard')
    expect(pathResolve('/admin', 'users')).toBe('/admin/users')
    // 绝对路径与外部链接原样返回，不参与拼接。
    expect(pathResolve('/group_project', '/dashboard')).toBe('/dashboard')
    expect(pathResolve('/', 'https://example.com/a/')).toBe('https://example.com/a/')

    // 环节2：路由名。菜单编码转 PascalCase，编码全局唯一保证路由名不重复。
    expect(routeNameOf('admin_login_log')).toBe('AdminLoginLog')
    expect(routeNameOf('group-project')).toBe('GroupProject')

    // 环节3：项目占位替换。菜单索引与当前 URL 靠同一个 ULID 对上。
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
    // 无当前项目时保留占位符，避免把非法路径注册进路由表。
    const [noProject] = substituteProject(
      [{ path: '/outline/:project_ulid', name: 'Outline', meta: {} }],
      ''
    )
    expect(noProject.path).toBe('/outline/:project_ulid')
  })
})
