import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { Link } from "react-router"
import { AtSign, Heart, Reply } from "lucide-react"
import { toast } from "sonner"
import { useSession } from "@/lib/session"
import { api, discussionUrl } from "@/lib/api"
import type { NotificationItem } from "@/lib/types"
import { UserAvatar } from "@/components/DiscussionParts"
import { Button } from "@/components/ui/button"
import { timeAgo } from "@/lib/format"

const icons = { reply: Reply, mention: AtSign, like: Heart }

export default function NotificationsPage() {
  const { data: session } = useSession()
  const qc = useQueryClient()
  const { data } = useQuery({
    queryKey: ["notifications"],
    queryFn: () => api<{ items: NotificationItem[]; unread: number }>("/api/me/notifications"),
  })

  const readAll = useMutation({
    mutationFn: () => api("/api/me/notifications/read", { method: "POST" }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["notifications"] })
      void qc.invalidateQueries({ queryKey: ["me"] })
      void qc.invalidateQueries({ queryKey: ["unread"] })
      toast.success("已全部标记为已读")
    },
  })

  return (
    <div>
      <div className="mb-4 flex items-center justify-between">
        <h1 className="text-xl font-bold">通知</h1>
        <Button variant="outline" size="sm" onClick={() => readAll.mutate()} disabled={readAll.isPending || session?.readOnly}>
          全部标记已读
        </Button>
      </div>
      <div>
        {(data?.items.length ?? 0) === 0 ? (
          <p className="p-10 text-center text-sm text-muted-foreground">暂无通知。</p>
        ) : (
          data!.items.map((n) => {
            const Icon = icons[n.type] ?? Reply
            const url = n.discussion
              ? `${discussionUrl(n.discussion.id, n.discussion.slug)}#post-${n.postId}`
              : "#"
            return (
              <Link
                key={n.id}
                to={url}
                className={`flex items-center gap-3 border-b px-4 py-3 transition-colors last:border-b-0 hover:bg-muted/50 ${
                  n.read ? "" : "bg-primary/5"
                }`}
              >
                <span className="flex size-7 shrink-0 items-center justify-center rounded-full bg-primary/10 text-primary">
                  <Icon className="size-3.5" />
                </span>
                <UserAvatar user={n.sender} className="size-7" />
                <span className="min-w-0 flex-1 text-sm">
                  <b>{n.sender?.username}</b>{" "}
                  {n.type === "reply" && "回复了你的帖子"}
                  {n.type === "mention" && "在帖子中提到了你"}
                  {n.type === "like" && "赞了你的回复"}
                  {n.discussion && (
                    <span className="block truncate text-primary">{n.discussion.title}</span>
                  )}
                </span>
                <time className="shrink-0 text-xs text-muted-foreground">{timeAgo(n.createdAt)}</time>
              </Link>
            )
          })
        )}
      </div>
    </div>
  )
}
