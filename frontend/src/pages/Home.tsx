import { useQuery } from "@tanstack/react-query"
import { Link, useNavigate } from "react-router"
import { api } from "@/lib/api"
import { useSession } from "@/lib/session"
import { timeAgo } from "@/lib/format"
import type { HomeCard } from "@/lib/types"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"

export default function HomePage() {
  const { data: session, isLoading: sessionLoading } = useSession()
  const siteName = session?.siteName ?? "JNSEC BBS"
  const user = session?.user ?? null
  const { data, isLoading } = useQuery({
    queryKey: ["home"],
    queryFn: () => api<{ cards: HomeCard[] }>("/api/home"),
    enabled: !!user, // 内容接口需要登录，未登录不请求
  })

  return (
    <div>
      {/* 未登录：欢迎横幅（文案可在管理后台设置），版块内容不可见 */}
      {!user && !sessionLoading && (
        <div className="mb-6 rounded-xl bg-gradient-to-r from-[#005661] to-[#00838f] px-6 py-10 text-center text-white shadow-sm">
          <h1 className="text-2xl font-bold">欢迎来到 {siteName}</h1>
          <p className="mx-auto mt-2 max-w-xl text-sm leading-relaxed text-white/85">
            {session?.welcomeText || "内部论坛，所有内容请登录后查看，请勿在其他渠道发布本论坛的内容。"}
          </p>
          <div className="mt-5 flex justify-center gap-3">
            <Button asChild className="bg-white text-[#006064] hover:bg-white/90">
              <Link to="/login">登录</Link>
            </Button>
            <Button
              asChild
              variant="outline"
              className="border-white/70 bg-transparent text-white hover:bg-white/15 hover:text-white"
            >
              <Link to="/register">注册</Link>
            </Button>
          </div>
        </div>
      )}

      {/* 登录后：版块卡片（固定行高，整卡可点） */}
      {user && isLoading ? (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3 auto-rows-[220px]">
          {Array.from({ length: 6 }).map((_, i) => (
            <Skeleton key={i} className="h-full rounded-xl" />
          ))}
        </div>
      ) : (
        user && (
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3 auto-rows-[220px]">
            {(data?.cards ?? []).map((card) => (
              <CategoryCard key={card.tag.id} card={card} />
            ))}
          </div>
        )
      )}
    </div>
  )
}

function CategoryCard({ card }: { card: HomeCard }) {
  const navigate = useNavigate()
  const color = card.tag.color || "#1e88e5"
  return (
    <section
      onClick={() => navigate(`/t/${card.tag.slug}`)}
      className="group flex h-full cursor-pointer flex-col overflow-hidden rounded-xl text-white shadow-sm transition-all hover:-translate-y-0.5 hover:shadow-lg"
      style={{ background: color }}
    >
      <header className="border-b border-white/25 px-4 pb-2 pt-4">
        <h2 className="text-lg font-bold leading-tight">
          <span className="group-hover:underline">{card.tag.name}</span>
        </h2>
        {card.tag.description && (
          <p className="mt-1 line-clamp-1 text-xs text-white/85">{card.tag.description}</p>
        )}
      </header>
      {card.children.length > 0 && (
        <div className="flex flex-wrap gap-1.5 px-4 py-3">
          {card.children.map((t) => (
            <Link
              key={t.id}
              to={`/t/${t.slug}`}
              onClick={(e) => e.stopPropagation()}
              className="rounded-full bg-white/20 px-2.5 py-0.5 text-xs font-medium hover:bg-white/40"
            >
              {t.name}
            </Link>
          ))}
        </div>
      )}
      <footer className="mt-auto border-t border-dashed border-white/35 px-4 py-3 text-sm">
        {card.latest ? (
          <Link
            to={card.latest.url}
            onClick={(e) => e.stopPropagation()}
            className="block truncate hover:underline"
          >
            {card.latest.title}
            <time className="ml-2 text-xs text-white/75">{timeAgo(card.latest.at)}</time>
          </Link>
        ) : (
          <span className="text-white/85">暂无话题，来发第一帖吧</span>
        )}
      </footer>
    </section>
  )
}
