import { useQuery } from "@tanstack/react-query"
import { Link, useParams } from "react-router"
import { api, discussionUrl } from "@/lib/api"
import type { ProfileData } from "@/lib/types"
import { DiscussionRow, GroupBadges, UserAvatar } from "@/components/DiscussionParts"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { useSession } from "@/lib/session"
import { fmtDate, timeAgo } from "@/lib/format"

export default function ProfilePage() {
  const { username = "" } = useParams()
  const { data: session } = useSession()
  const { data, isError } = useQuery({
    queryKey: ["profile", username],
    queryFn: () => api<ProfileData>(`/api/users/${encodeURIComponent(username)}`),
    retry: false,
  })
  if (isError) {
    return <p className="py-20 text-center text-muted-foreground">该用户不存在。</p>
  }
  if (!data) return null
  const u = data.user

  return (
    <div>
      <div className="mb-5 flex flex-wrap items-start gap-5">
        <UserAvatar user={u} className="size-20" />
        <div className="min-w-0 flex-1">
          <h1 className="flex flex-wrap items-center gap-2 text-xl font-bold">
            {u.username}
            <GroupBadges user={u} />
            {u.isBanned && <Badge variant="destructive">已封禁</Badge>}
          </h1>
          <p className="mt-1 text-sm text-muted-foreground">
            加入于 {fmtDate(u.createdAt)} · {u.discussionsCount} 主题 · {u.postsCount} 回复
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
                to={discussion ? `${discussionUrl(discussion.id, discussion.slug)}#post-${post.id}` : "/"}
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
