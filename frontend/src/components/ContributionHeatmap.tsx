// GitHub 风格的贡献热力图：年历周列 × 7 行，五档颜色。
// 颜色分档 0 / 1-2 / 3-5 / 6-9 / 10+，含明暗两套与 GitHub 对齐的绿色。
import { useState } from "react"
import { useQuery } from "@tanstack/react-query"
import { api } from "@/lib/api"
import type { ActivityData } from "@/lib/types"
import { cn } from "cn"

const MONTHS = ["1月", "2月", "3月", "4月", "5月", "6月", "7月", "8月", "9月", "10月", "11月", "12月"]
const WEEKDAYS = ["一", "", "三", "", "五", "", ""] // 行序：周一到周日

const LEVEL_CLASS = [
  "bg-muted",
  "bg-[#9be9a8] dark:bg-[#0e4429]",
  "bg-[#40c463] dark:bg-[#006d32]",
  "bg-[#30a14e] dark:bg-[#26a641]",
  "bg-[#216e39] dark:bg-[#39d353]",
] as const

function levelOf(n: number): number {
  if (n <= 0) return 0
  if (n <= 2) return 1
  if (n <= 5) return 2
  if (n <= 9) return 3
  return 4
}

const CELL = 13 // 10px 方块 + 3px 间隙，月份标签定位用

export function ContributionHeatmap({ username }: { username: string }) {
  const [year, setYear] = useState<number | null>(null)
  const { data, isLoading } = useQuery({
    queryKey: ["activity", username, year ?? "current"],
    queryFn: () =>
      api<ActivityData>(`/api/users/${encodeURIComponent(username)}/activity${year ? `?year=${year}` : ""}`),
  })

  if (isLoading || !data) return null

  const year0 = data.year
  const jan1 = new Date(year0, 0, 1)
  const lead = (jan1.getDay() + 6) % 7 // 1 月 1 日前留白格数（周一为行首）
  const cols = Math.ceil((lead + data.days.length) / 7)
  const today = new Date()

  // 月份标签：每列首格所属月份相对前一列变化时标注（跳过上一年残留）
  const monthLabels: { col: number; text: string }[] = []
  let prevMonth = -1
  for (let col = 0; col < cols; col++) {
    const d = new Date(year0, 0, 1 + col * 7 - lead)
    if (d.getFullYear() < year0) continue
    if (d.getMonth() !== prevMonth) {
      monthLabels.push({ col, text: MONTHS[d.getMonth()] })
      prevMonth = d.getMonth()
    }
  }

  return (
    <section className="overflow-hidden rounded-xl border bg-card">
      <header className="flex flex-wrap items-center justify-between gap-2 border-b px-4 py-2.5">
        <h2 className="text-sm font-semibold">
          {data.total} 次回复 · {year0} 年
        </h2>
        <div className="flex items-center gap-1">
          {data.years
            .slice()
            .reverse()
            .map((y) => (
              <button
                key={y}
                onClick={() => setYear(y)}
                className={cn(
                  "rounded-md px-2 py-1 text-xs text-muted-foreground transition-colors hover:bg-muted hover:text-foreground",
                  y === data.year && "bg-secondary font-medium text-foreground",
                )}
              >
                {y}
              </button>
            ))}
        </div>
      </header>
      <div className="overflow-x-auto p-4">
        <div className="flex gap-1.5" style={{ width: cols * CELL + 24 }}>
          <div className="flex w-5 shrink-0 flex-col gap-[3px] pt-5 text-[9px] leading-[10px] text-muted-foreground">
            {WEEKDAYS.map((w, i) => (
              <span key={i} className="h-[10px]">
                {w}
              </span>
            ))}
          </div>
          <div>
            {/* 月份标签行 */}
            <div className="relative mb-1 h-4" style={{ width: cols * CELL }}>
              {monthLabels.map((m) => (
                <span
                  key={m.col}
                  className="absolute top-0 text-[10px] text-muted-foreground"
                  style={{ left: m.col * CELL }}
                >
                  {m.text}
                </span>
              ))}
            </div>
            {/* 方块网格：grid-rows-7 + grid-flow-col，先填 1 月 1 日前的空位 */}
            <div className="grid grid-flow-col grid-rows-7 gap-[3px]">
              {Array.from({ length: lead }).map((_, i) => (
                <span key={`lead-${i}`} className="size-[10px]" />
              ))}
              {data.days.map((d) => {
                const future = new Date(d.date + "T23:59:59") > today
                const level = levelOf(d.count)
                return (
                  <span
                    key={d.date}
                    title={`${d.date} · ${d.count} 次回复`}
                    className={cn(
                      "size-[10px] rounded-[2px]",
                      future ? "bg-transparent ring-1 ring-inset ring-border/50" : LEVEL_CLASS[level],
                    )}
                  />
                )
              })}
            </div>
          </div>
        </div>
      </div>
      <footer className="flex items-center justify-end gap-1 border-t px-4 py-2 text-xs text-muted-foreground">
        少
        {LEVEL_CLASS.map((cls, i) => (
          <span key={i} className={cn("size-[10px] rounded-[2px]", cls)} />
        ))}
        多
      </footer>
    </section>
  )
}
