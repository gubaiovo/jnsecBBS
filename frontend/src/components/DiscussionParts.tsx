import { Link } from "react-router"
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar"
import { Badge } from "@/components/ui/badge"
import { cn } from "cn"
import type { DiscussionSummary, Tag, User } from "@/lib/types"
import { timeAgo } from "@/lib/format"

export function UserAvatar({ user, className }: { user: User | null; className?: string }) {
  return (
    <Avatar className={cn("size-9", className)}>
      <AvatarImage src={user?.avatar ?? "/avatar/_unknown.svg"} alt={user?.username ?? "?"} />
      <AvatarFallback>{user?.username?.[0] ?? "?"}</AvatarFallback>
    </Avatar>
  )
}

export function TagChip({ tag, onColor = false }: { tag: Tag; onColor?: boolean }) {
  return (
    <Link
      to={`/t/${tag.slug}`}
      className={cn(
        "inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium transition-colors",
        onColor
          ? "bg-white/20 text-white hover:bg-white/35"
          : "border bg-secondary/60 text-secondary-foreground hover:bg-secondary",
      )}
    >
      {tag.name}
    </Link>
  )
}

export function GroupBadges({ user }: { user: User }) {
  return (
    <>
      {user.groups
        .filter((g) => g.id !== 2)
        .map((g) => (
          <Badge
            key={g.id}
            variant="outline"
            className="text-[10px] px-1.5 py-0"
            style={g.color ? { color: g.color, borderColor: `${g.color}66` } : undefined}
          >
            {g.name}
          </Badge>
        ))}
    </>
  )
}

export function DiscussionRow({ d }: { d: DiscussionSummary }) {
  return (
    <div className="flex gap-3 py-3.5 border-b border-border/50 last:border-b-0">
      <Link to={`/u/${d.author?.username ?? ""}`} className="shrink-0">
        <UserAvatar user={d.author} />
      </Link>
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-1.5 flex-wrap">
          {d.isSticky && (
            <span className="inline-flex items-center gap-0.5 rounded bg-orange-500/10 px-1.5 py-0.5 text-[10px] font-semibold text-orange-600 dark:text-orange-400">
              置顶
            </span>
          )}
          {d.isLocked && (
            <span className="inline-flex items-center gap-0.5 rounded bg-blue-500/10 px-1.5 py-0.5 text-[10px] font-semibold text-blue-600 dark:text-blue-400">
              锁定
            </span>
          )}
          {d.isHidden && (
            <span className="inline-flex items-center rounded bg-red-500/10 px-1.5 py-0.5 text-[10px] font-semibold text-red-600 dark:text-red-400">
              已删除
            </span>
          )}
          <Link
            to={`/d/${d.id}-${d.slug}`}
            className="font-semibold text-foreground hover:text-primary truncate"
          >
            {d.title}
          </Link>
        </div>
        <div className="mt-1 flex items-center gap-2 text-xs text-muted-foreground flex-wrap">
          <Link to={`/u/${d.author?.username ?? ""}`} className="hover:text-primary">
            {d.author?.username}
          </Link>
          {d.tags.map((t) => (
            <TagChip key={t.id} tag={t} />
          ))}
        </div>
      </div>
      <div className="flex flex-col items-end justify-center text-xs text-muted-foreground whitespace-nowrap">
        <span>
          <b className="text-primary text-sm">{d.commentCount}</b> 回复
        </span>
        <span>{timeAgo(d.lastPostedAt)}</span>
        {d.lastPoster && <span>最后 {d.lastPoster}</span>}
      </div>
    </div>
  )
}
