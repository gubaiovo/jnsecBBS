import { useQuery } from "@tanstack/react-query"
import { Link, useParams, useSearchParams } from "react-router"
import { PenSquare } from "lucide-react"
import { api } from "@/lib/api"
import type { DiscussionSummary, Page, Tag as TagType } from "@/lib/types"
import { DiscussionRow } from "@/components/DiscussionParts"
import { Pagination } from "@/components/Pagination"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"

export default function TagPage() {
  const { slug = "" } = useParams()
  const [params, setParams] = useSearchParams()
  const page = Number(params.get("page") ?? 1) || 1

  const { data: detail } = useQuery({
    queryKey: ["tag", slug],
    queryFn: () => api<{ tag: TagType; canPost: boolean }>(`/api/tags/${slug}`),
  })
  const { data, isLoading } = useQuery({
    queryKey: ["tagDiscussions", slug, page],
    queryFn: () =>
      api<{ items: DiscussionSummary[]; page: Page }>(
        `/api/tags/${slug}/discussions?page=${page}`,
      ),
  })

  const tag = detail?.tag
  const color = tag?.color || "#1e88e5"

  return (
    <div>
      <header
        className="mb-5 flex flex-wrap items-center justify-between gap-4 rounded-xl px-5 py-5 text-white"
        style={{ background: color }}
      >
        <div>
          <h1 className="text-xl font-bold">{tag?.name ?? "…"}</h1>
          {tag?.description && <p className="mt-1 text-sm text-white/90">{tag.description}</p>}
          {tag?.children && tag.children.length > 0 && (
            <div className="mt-2 flex flex-wrap gap-1.5">
              {tag.children.map((t) => (
                <Link
                  key={t.id}
                  to={`/t/${t.slug}`}
                  className="rounded-full bg-white/20 px-2.5 py-0.5 text-xs font-medium hover:bg-white/35"
                >
                  {t.name}
                </Link>
              ))}
            </div>
          )}
        </div>
        {detail?.canPost && (
          <Button asChild variant="secondary" size="sm" className="gap-1.5">
            <Link to="/post">
              <PenSquare /> 发新帖
            </Link>
          </Button>
        )}
      </header>

      <div>
        {isLoading ? (
          <div className="space-y-3 py-4">
            {Array.from({ length: 5 }).map((_, i) => (
              <Skeleton key={i} className="h-12" />
            ))}
          </div>
        ) : (data?.items.length ?? 0) === 0 ? (
          <p className="py-10 text-center text-sm text-muted-foreground">该版块还没有话题。</p>
        ) : (
          data!.items.map((d) => <DiscussionRow key={d.id} d={d} />)
        )}
      </div>
      <Pagination page={data?.page ?? { page: 1, pages: 1, total: 0 }} onPage={(p) => setParams({ page: String(p) })} />
    </div>
  )
}
