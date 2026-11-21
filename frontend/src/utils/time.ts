// 时间展示工具：后端统一以 RFC3339 字符串返回时间，这里集中处理解析、格式化与相对时间。

/** 解析后端 RFC3339 时间为 Date（空值/非法值返回 null） */
export function parseTimeText(timeText: string | null | undefined): Date | null {
  if (!timeText) {
    return null
  }
  const parsedDate = new Date(timeText)
  return Number.isNaN(parsedDate.getTime()) ? null : parsedDate
}

/** 格式化为本地日期时间（列表、详情通用） */
export function formatTime(timeText: string | null | undefined): string {
  const parsedDate = parseTimeText(timeText)
  return parsedDate ? parsedDate.toLocaleString() : '—'
}

/** 格式化为本地时分（对话消息用） */
export function formatClockTime(timeText: string | null | undefined): string {
  const parsedDate = parseTimeText(timeText)
  return parsedDate ? parsedDate.toLocaleTimeString() : ''
}

/** 格式化为「刚刚 / N 分钟前 / N 小时前 / N 天前」，超过 30 天回落到日期 */
export function formatRelativeTime(timeText: string | null | undefined): string {
  const parsedDate = parseTimeText(timeText)
  if (!parsedDate) {
    return '—'
  }
  const elapsedMs = Date.now() - parsedDate.getTime()
  if (elapsedMs < 0) {
    return formatTime(timeText)
  }
  const elapsedMinutes = Math.floor(elapsedMs / 60000)
  if (elapsedMinutes < 1) {
    return '刚刚'
  }
  if (elapsedMinutes < 60) {
    return `${elapsedMinutes} 分钟前`
  }
  const elapsedHours = Math.floor(elapsedMinutes / 60)
  if (elapsedHours < 24) {
    return `${elapsedHours} 小时前`
  }
  const elapsedDays = Math.floor(elapsedHours / 24)
  if (elapsedDays <= 30) {
    return `${elapsedDays} 天前`
  }
  return formatTime(timeText)
}
