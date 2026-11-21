/** 业务成功码（后端包络 resp_code） */
export const RESP_CODE_SUCCESS = 'SUCCESS'

/** 未登录 / 令牌失效业务码 */
export const RESP_CODE_UNAUTHORIZED = 'ERR_UNAUTHORIZED'

/** 无权限业务码 */
export const RESP_CODE_FORBIDDEN = 'ERR_FORBIDDEN'

/** 资源不存在业务码 */
export const RESP_CODE_NOT_FOUND = 'ERR_NOT_FOUND'

/** 接口前缀（nginx 与 vite 均按 /api 反代到后端） */
export const API_BASE_PATH = '/api/v1'

/** 请求超时时间（毫秒） */
export const REQUEST_TIMEOUT = 120000

/** 免登录白名单 */
export const NO_REDIRECT_WHITE_LIST = ['/login']

/** 登出时保留的路由名（登录、重定向、404 与业务壳） */
export const NO_RESET_WHITE_LIST = ['Login', 'Redirect', 'Fallback', 'Root']
