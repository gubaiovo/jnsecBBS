// 相对时间与日期格式化（Intl，无第三方依赖）
export function timeAgo(iso: string): string {
  const t = new Date(iso).getTime()
  if (Number.isNaN(t)) return ""
  const d = Date.now() - t
  const min = 60_000, hour = 3_600_000, day = 86_400_000
  if (d < min) return "刚刚"
  if (d < hour) return `${Math.floor(d / min)} 分钟前`
  if (d < day) return `${Math.floor(d / hour)} 小时前`
  if (d < 30 * day) return `${Math.floor(d / day)} 天前`
  if (d < 365 * day) return `${Math.floor(d / (30 * day))} 个月前`
  return fmtDate(iso)
}

export function fmtDate(iso: string): string {
  if (!iso) return ""
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ""
  const p = (n: number) => String(n).padStart(2, "0")
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`
}
