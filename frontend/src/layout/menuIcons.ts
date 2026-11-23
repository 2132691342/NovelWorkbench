/**
 * 后端菜单 icon_code → 离线图标表键。
 * 后端只存语义编码（dashboard / book / tree …），前端决定具体图形；
 * 未登记的编码回落到通用图标，菜单不会因此丢图标。
 */
const ICON_CODE_MAP: Record<string, string> = {
  dashboard: 'mdi:view-dashboard',
  book: 'mdi:book-open-page-variant-outline',
  edit: 'mdi:pencil-outline',
  tree: 'mdi:file-tree',
  shield: 'mdi:shield-check-outline',
  user: 'mdi:account-group',
  globe: 'mdi:earth',
  database: 'mdi:database-outline',
  note: 'mdi:notebook-outline',
  layers: 'mdi:layers-outline',
  model: 'mdi:robot-outline',
  wand: 'mdi:magic-staff',
  sparkles: 'mdi:sparkles',
  bell: 'mdi:bell-outline',
  settings: 'mdi:cog-outline',
  sliders: 'mdi:cog-outline',
  users: 'mdi:account-multiple-outline',
  key: 'mdi:key-outline',
  menu: 'mdi:menu',
  lock: 'mdi:lock-outline',
  history: 'mdi:history',
  chart: 'mdi:chart-box-outline',
  pulse: 'mdi:pulse',
  tool: 'mdi:toolbox-outline',
  close: 'mdi:close',
  check: 'mdi:check',
  plus: 'mdi:plus'
}

/** 解析菜单图标：编码为空或未登记时给通用图标 */
export const resolveMenuIcon = (iconCode: string) =>
  ICON_CODE_MAP[iconCode] ?? 'mdi:file-document-outline'
