// 图标表：登记实际使用的 mdi 图标，供全局 Icon 组件按名渲染。
import type { IconRegistry } from '@novelworkbench/components'

import accountCircleOutline from '@iconify-icons/mdi/account-circle-outline'
import accountGroup from '@iconify-icons/mdi/account-group'
import accountMultipleOutline from '@iconify-icons/mdi/account-multiple-outline'
import alertCircleOutline from '@iconify-icons/mdi/alert-circle-outline'
import arrowLeft from '@iconify-icons/mdi/arrow-left'
import bellOutline from '@iconify-icons/mdi/bell-outline'
import book from '@iconify-icons/mdi/book'
import bookOpenPageVariantOutline from '@iconify-icons/mdi/book-open-page-variant-outline'
import bookOpenVariant from '@iconify-icons/mdi/book-open-variant'
import chartBoxOutline from '@iconify-icons/mdi/chart-box-outline'
import check from '@iconify-icons/mdi/check'
import chevronDoubleLeft from '@iconify-icons/mdi/chevron-double-left'
import chevronDoubleRight from '@iconify-icons/mdi/chevron-double-right'
import close from '@iconify-icons/mdi/close'
import cogOutline from '@iconify-icons/mdi/cog-outline'
import contentCopy from '@iconify-icons/mdi/content-copy'
import database from '@iconify-icons/mdi/database'
import databaseOutline from '@iconify-icons/mdi/database-outline'
import deleteOutline from '@iconify-icons/mdi/delete-outline'
import dotsHorizontal from '@iconify-icons/mdi/dots-horizontal'
import download from '@iconify-icons/mdi/download'
import earth from '@iconify-icons/mdi/earth'
import fileDocumentOutline from '@iconify-icons/mdi/file-document-outline'
import fileTree from '@iconify-icons/mdi/file-tree'
import folderOutline from '@iconify-icons/mdi/folder-outline'
import history from '@iconify-icons/mdi/history'
import keyOutline from '@iconify-icons/mdi/key-outline'
import layersOutline from '@iconify-icons/mdi/layers-outline'
import lockOutline from '@iconify-icons/mdi/lock-outline'
import loginVariant from '@iconify-icons/mdi/login-variant'
import logout from '@iconify-icons/mdi/logout'
import magicStaff from '@iconify-icons/mdi/magic-staff'
import menu from '@iconify-icons/mdi/menu'
import menuOpen from '@iconify-icons/mdi/menu-open'
import messageTextOutline from '@iconify-icons/mdi/message-text-outline'
import minus from '@iconify-icons/mdi/minus'
import notebookOutline from '@iconify-icons/mdi/notebook-outline'
import pageFirst from '@iconify-icons/mdi/page-first'
import pageLast from '@iconify-icons/mdi/page-last'
import pencilOutline from '@iconify-icons/mdi/pencil-outline'
import plus from '@iconify-icons/mdi/plus'
import progressClock from '@iconify-icons/mdi/progress-clock'
import pulse from '@iconify-icons/mdi/pulse'
import refresh from '@iconify-icons/mdi/refresh'
import reload from '@iconify-icons/mdi/reload'
import robotOutline from '@iconify-icons/mdi/robot-outline'
import scriptTextOutline from '@iconify-icons/mdi/script-text-outline'
import send from '@iconify-icons/mdi/send'
import shieldCheckOutline from '@iconify-icons/mdi/shield-check-outline'
import sparkles from '@iconify-icons/mdi/sparkles'
import stop from '@iconify-icons/mdi/stop'
import sync from '@iconify-icons/mdi/sync'
import tagOutline from '@iconify-icons/mdi/tag-outline'
import viewDashboard from '@iconify-icons/mdi/view-dashboard'
import wand from '@iconify-icons/mdi/wand'

/** 离线图标表：只登记界面实际用到的图标，避免整包引入 iconify 数据集 */
export const icons = {
  'mdi:account-circle-outline': accountCircleOutline,
  'mdi:account-group': accountGroup,
  'mdi:account-multiple-outline': accountMultipleOutline,
  'mdi:alert-circle-outline': alertCircleOutline,
  'mdi:arrow-left': arrowLeft,
  'mdi:bell-outline': bellOutline,
  'mdi:book': book,
  'mdi:book-open-page-variant-outline': bookOpenPageVariantOutline,
  'mdi:book-open-variant': bookOpenVariant,
  'mdi:chart-box-outline': chartBoxOutline,
  'mdi:check': check,
  'mdi:chevron-double-left': chevronDoubleLeft,
  'mdi:chevron-double-right': chevronDoubleRight,
  'mdi:close': close,
  'mdi:cog-outline': cogOutline,
  'mdi:content-copy': contentCopy,
  'mdi:database': database,
  'mdi:database-outline': databaseOutline,
  'mdi:delete-outline': deleteOutline,
  'mdi:dots-horizontal': dotsHorizontal,
  'mdi:download': download,
  'mdi:earth': earth,
  'mdi:file-document-outline': fileDocumentOutline,
  'mdi:file-tree': fileTree,
  'mdi:folder-outline': folderOutline,
  'mdi:history': history,
  'mdi:key-outline': keyOutline,
  'mdi:layers-outline': layersOutline,
  'mdi:lock-outline': lockOutline,
  'mdi:login-variant': loginVariant,
  'mdi:logout': logout,
  'mdi:magic-staff': magicStaff,
  'mdi:menu': menu,
  'mdi:menu-open': menuOpen,
  'mdi:message-text-outline': messageTextOutline,
  'mdi:minus': minus,
  'mdi:notebook-outline': notebookOutline,
  'mdi:page-first': pageFirst,
  'mdi:page-last': pageLast,
  'mdi:pencil-outline': pencilOutline,
  'mdi:plus': plus,
  'mdi:progress-clock': progressClock,
  'mdi:pulse': pulse,
  'mdi:refresh': refresh,
  'mdi:reload': reload,
  'mdi:robot-outline': robotOutline,
  'mdi:script-text-outline': scriptTextOutline,
  'mdi:send': send,
  'mdi:shield-check-outline': shieldCheckOutline,
  'mdi:sparkles': sparkles,
  'mdi:stop': stop,
  'mdi:sync': sync,
  'mdi:tag-outline': tagOutline,
  'mdi:view-dashboard': viewDashboard,
  'mdi:wand': wand
} satisfies IconRegistry
