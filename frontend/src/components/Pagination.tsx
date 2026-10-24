import { ChevronLeft, ChevronRight } from "lucide-react"
import { Button } from "@/components/ui/button"
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
    <nav className="flex items-center justify-center gap-1.5 flex-wrap py-4">
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
    </nav>
  )
}
