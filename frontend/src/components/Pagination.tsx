import { useState } from "react"
import { ChevronFirst, ChevronLast, ChevronLeft, ChevronRight } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import type { Page } from "@/lib/types"

function pageWindow(page: number, pages: number): number[] {
  let start = Math.max(1, page - 3)
  const end = Math.min(pages, start + 6)
  start = Math.max(1, end - 6)
  const out: number[] = []
  for (let i = start; i <= end; i++) out.push(i)
  return out
}

export function Pagination({
  page,
  onPage,
}: {
  page: Page
  onPage: (p: number) => void
}) {
  if (page.pages <= 1) return null
  return (
    <nav aria-label="分页" className="flex items-center justify-center gap-1.5 flex-wrap py-4">
      <Button variant="outline" size="sm" disabled={page.page <= 1} onClick={() => onPage(1)}>
        <ChevronFirst /> 首页
      </Button>
      <Button
        variant="outline"
        size="sm"
        disabled={page.page <= 1}
        onClick={() => onPage(page.page - 1)}
      >
        <ChevronLeft /> 上一页
      </Button>
      {pageWindow(page.page, page.pages).map((p) => (
        <Button
          key={p}
          variant={p === page.page ? "default" : "outline"}
          size="sm"
          aria-current={p === page.page ? "page" : undefined}
          onClick={() => onPage(p)}
        >
          {p}
        </Button>
      ))}
      <Button
        variant="outline"
        size="sm"
        disabled={page.page >= page.pages}
        onClick={() => onPage(page.page + 1)}
      >
        下一页 <ChevronRight />
      </Button>
      <Button variant="outline" size="sm" disabled={page.page >= page.pages} onClick={() => onPage(page.pages)}>
        末页 <ChevronLast />
      </Button>
      <PageJump key={`${page.page}-${page.pages}`} page={page} onPage={onPage} />
    </nav>
  )
}

function PageJump({ page, onPage }: { page: Page; onPage: (p: number) => void }) {
  const [value, setValue] = useState(String(page.page))
  return (
    <form
      className="flex items-center gap-1.5 text-sm text-muted-foreground"
      onSubmit={(e) => {
        e.preventDefault()
        const target = Number(value)
        if (Number.isSafeInteger(target) && target >= 1 && target <= page.pages && target !== page.page) {
          onPage(target)
        }
      }}
    >
      <span>共 {page.pages} 页</span>
      <Input
        aria-label="跳转页码"
        type="number"
        min={1}
        max={page.pages}
        step={1}
        required
        value={value}
        onChange={(e) => setValue(e.target.value)}
        className="h-8 w-16"
      />
      <Button type="submit" variant="outline" size="sm">跳转</Button>
    </form>
  )
}
