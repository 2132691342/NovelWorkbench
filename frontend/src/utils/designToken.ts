// 设计令牌读取：图表与 X6 等 Canvas/SVG 渲染器拿不到 CSS 变量，需要在运行时取令牌值。
// 令牌只在这里读取一次（模块级缓存），主题切换时调用 resetDesignTokenCache 重新读取。

/** 令牌缓存（首次读取后复用，避免每次重绘都触发样式计算） */
const tokenCache = new Map<string, string>()

/** 读取 CSS 变量值；未定义时返回兜底色，保证图表不出现透明或黑色块 */
export function readDesignToken(tokenName: string, fallbackColor: string): string {
  const cached = tokenCache.get(tokenName)
  if (cached !== undefined) {
    return cached
  }
  const valueText = getComputedStyle(document.documentElement).getPropertyValue(tokenName).trim()
  const resolved = valueText || fallbackColor
  tokenCache.set(tokenName, resolved)
  return resolved
}

/** 清空令牌缓存（主题或语言切换导致变量变化时调用） */
export function resetDesignTokenCache(): void {
  tokenCache.clear()
}

/** 图表系列色（对齐 --chart-series-N，深色给主序列） */
export function chartSeriesColor(seriesIndex: number): string {
  return readDesignToken(`--chart-series-${seriesIndex}`, '#6366f1')
}

/** 把 #rrggbb 令牌转成带透明度的 rgba（Canvas 不支持 color-mix，面积渐变必须给实值） */
export function tokenWithAlpha(tokenName: string, alphaValue: number, fallbackColor = '#6366f1'): string {
  const hexText = readDesignToken(tokenName, fallbackColor).replace('#', '')
  const normalized = hexText.length === 3
    ? hexText.split('').map((char) => char + char).join('')
    : hexText
  if (normalized.length !== 6 || !/^[0-9a-fA-F]{6}$/.test(normalized)) {
    return `rgba(99, 102, 241, ${alphaValue})`
  }
  const red = parseInt(normalized.slice(0, 2), 16)
  const green = parseInt(normalized.slice(2, 4), 16)
  const blue = parseInt(normalized.slice(4, 6), 16)
  return `rgba(${red}, ${green}, ${blue}, ${alphaValue})`
}

/** 图表通用中性色（网格、轴线、次要文字） */
export const chartToken = {
  /** 图表背景 */
  background(): string {
    return readDesignToken('--bg-card', '#ffffff')
  },
  /** 提示框描边 */
  border(): string {
    return readDesignToken('--border-light', '#eef2f7')
  },
  /** 主文字 */
  textMain(): string {
    return readDesignToken('--text-main', '#1e293b')
  },
  /** 次要文字 */
  textSecondary(): string {
    return readDesignToken('--text-secondary', '#64748b')
  },
  /** 坐标轴标签 */
  textPlaceholder(): string {
    return readDesignToken('--text-placeholder', '#94a3b8')
  },
  /** 轴线 */
  axisLine(): string {
    return readDesignToken('--border', '#e2e8f0')
  },
  /** 分割线 */
  splitLine(): string {
    return readDesignToken('--chart-grid', '#eef2f7')
  },
} as const
