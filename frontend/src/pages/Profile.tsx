import { useQuery } from "@tanstack/react-query"
import { Link, useParams } from "react-router"
import { BadgeCheck, Link2 } from "lucide-react"
import { api, ApiError } from "@/lib/api"
import type { ProfileData } from "@/lib/types"
import { DiscussionRow, GroupBadges, UserAvatar } from "@/components/DiscussionParts"
import { ContributionHeatmap } from "@/components/ContributionHeatmap"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { useSession } from "@/lib/session"
import { fmtDate, timeAgo } from "@/lib/format"

export default function ProfilePage() {
  const { username = "" } = useParams()
  const { data: session } = useSession()
  const { data, error } = useQuery({
    queryKey: ["profile", username],
    queryFn: () => api<ProfileData>(`/api/users/${encodeURIComponent(username)}`),
    retry: false,
  })
  if (error) {
    const status = error instanceof ApiError ? error.status : 0
    return (
      <p className="py-20 text-center text-muted-foreground">
        {status === 401 ? "登录后才能查看用户主页。" : "该用户不存在。"}
      </p>
    )
  }
  if (!data) return null
  const u = data.user
  const blogHost = (() => {
    if (!data.blogUrl) return null
    try {
      return new URL(data.blogUrl).hostname
    } catch {
      return data.blogUrl
    }
  })()

  return (
    <div>
      <div className="mb-5 flex flex-wrap items-start gap-5">
        <UserAvatar user={u} className="size-20" />
        <div className="min-w-0 flex-1">
          <h1 className="flex flex-wrap items-center gap-2 text-xl font-bold">
            {u.username}
            <GroupBadges user={u} />
            {data.realName && (
              <Badge variant="secondary" className="gap-1 font-normal">
                <BadgeCheck className="size-3.5 text-sky-500" />
                实名：{data.realName}
              </Badge>
            )}
            {u.isBanned && <Badge variant="destructive">已封禁</Badge>}
          </h1>
          <p className="mt-1 flex flex-wrap items-center gap-x-2 gap-y-1 text-sm text-muted-foreground">
            <span>加入于 {fmtDate(u.createdAt)}</span>
            <span>· {u.discussionsCount} 主题</span>
            <span>· {u.postsCount} 回复</span>
            {blogHost && data.blogUrl && (
              <>
                <span>·</span>
                <a
                  href={data.blogUrl}
                  target="_blank"
                  rel="noopener noreferrer nofollow"
                  className="inline-flex items-center gap-1 font-medium text-primary hover:underline"
                >
                  <Link2 className="size-3.5" />
                  {blogHost}
                </a>
              </>
            )}
          </p>
          {u.bio && <p className="mt-2 text-sm">{u.bio}</p>}
        </div>
        {!session?.readOnly && session?.user && session.user.id === u.id && (
          <Button asChild variant="outline" size="sm">
            <Link to="/settings">编辑资料与设置</Link>
          </Button>
        )}
        {!session?.readOnly && session?.user?.isAdmin && (
          <Button asChild variant="outline" size="sm">
            <Link to="/admin">管理后台</Link>
          </Button>
        )}
      </div>

      <div className="mb-8">
        <ContributionHeatmap username={u.username} />
      </div>

      <h2 className="mb-2 mt-6 text-lg font-bold">最近主题（{data.discTotal}）</h2>
      <div>
        {data.discussions.length === 0 ? (
          <p className="py-8 text-center text-sm text-muted-foreground">暂无主题。</p>
        ) : (
          data.discussions.map((d) => <DiscussionRow key={d.id} d={d} />)
        )}
      </div>

      <h2 className="mb-2 mt-6 text-lg font-bold">最近回复</h2>
      <div>
        {data.posts.length === 0 ? (
          <p className="py-8 text-center text-sm text-muted-foreground">暂无回复。</p>
        ) : (
          data.posts.map(({ post, discussion }) => (
            <div key={post.id} className="border-b border-border/50 py-3 last:border-b-0">
              <Link
                to={discussion ? `/d/${discussion.id}-${discussion.slug}#post-${post.id}` : "/"}
                className="font-semibold hover:text-primary"
              >
                {discussion?.title ?? ""}
              </Link>
              <p className="mt-1 line-clamp-2 text-sm text-muted-foreground">
                {post.content.slice(0, 120)}
              </p>
              <time className="mt-1 block text-xs text-muted-foreground">{timeAgo(post.createdAt)}</time>
            </div>
          ))
        )}
      </div>
    </div>
  )
}
