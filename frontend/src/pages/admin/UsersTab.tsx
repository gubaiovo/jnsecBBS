import { useState } from "react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { Ban, Search } from "lucide-react"
import { toast } from "sonner"
import { api } from "@/lib/api"
import type { AdminUserRow } from "@/lib/types"
import type { AdminMeta } from "../Admin"
import { Button } from "@/components/ui/button"
import { Card } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Switch } from "@/components/ui/switch"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"

export function UsersTab({ meta }: { meta?: AdminMeta }) {
  const qc = useQueryClient()
  const [q, setQ] = useState("")
  const [query, setQuery] = useState("")
  const { data } = useQuery({
    queryKey: ["adminUsers", query],
    queryFn: () => api<{ users: AdminUserRow[] }>(`/api/admin/users?q=${encodeURIComponent(query)}`),
  })

  const save = useMutation({
    mutationFn: ({ id, body }: { id: number; body: Record<string, unknown> }) =>
      api(`/api/admin/users/${id}`, { json: body }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["adminUsers"] })
      toast.success("已保存")
    },
    onError: (e) => toast.error((e as Error).message),
  })

  return (
    <div className="space-y-4">
      <form
        className="flex max-w-md gap-2"
        onSubmit={(e) => {
          e.preventDefault()
          setQuery(q)
        }}
      >
        <Input value={q} onChange={(e) => setQ(e.target.value)} placeholder="搜索用户名 / 邮箱" />
        <Button type="submit" variant="outline" className="gap-1.5">
          <Search className="size-4" /> 搜索
        </Button>
      </form>
      <Card className="overflow-x-auto p-0">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>ID</TableHead>
              <TableHead>用户名</TableHead>
              <TableHead>邮箱</TableHead>
              <TableHead>用户组</TableHead>
              <TableHead>主题/回复</TableHead>
              <TableHead>状态</TableHead>
              <TableHead>操作</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {(data?.users ?? []).map((row) => (
              <TableRow key={row.user.id}>
                <TableCell>{row.user.id}</TableCell>
                <TableCell className="font-medium">{row.user.username}</TableCell>
                <TableCell className="text-muted-foreground">{row.email}</TableCell>
                <TableCell>
                  <div className="flex flex-wrap items-center gap-2">
                    {meta?.groups
                      .filter((g) => g.id !== 2)
                      .map((g) => (
                        <label key={g.id} className="flex items-center gap-1 text-xs">
                          <Switch
                            checked={row.groupIds.includes(g.id)}
                            onCheckedChange={() => {
                              const next = row.groupIds.includes(g.id)
                                ? row.groupIds.filter((x) => x !== g.id)
                                : [...row.groupIds, g.id]
                              save.mutate({ id: row.user.id, body: { action: "groups", groups: next } })
                            }}
                          />
                          {g.name}
                        </label>
                      ))}
                  </div>
                </TableCell>
                <TableCell>
                  {row.user.discussionsCount} / {row.user.postsCount}
                </TableCell>
                <TableCell>
                  {row.user.isBanned ? (
                    <span className="text-xs font-semibold text-red-600">已封禁</span>
                  ) : (
                    <span className="text-xs font-semibold text-green-600">正常</span>
                  )}
                </TableCell>
                <TableCell>
                  <Button
                    variant="outline"
                    size="sm"
                    className={row.user.isBanned ? "" : "text-red-600"}
                    onClick={() =>
                      save.mutate({
                        id: row.user.id,
                        body: { action: row.user.isBanned ? "unban" : "ban" },
                      })
                    }
                  >
                    <Ban className="size-3.5" /> {row.user.isBanned ? "解封" : "封禁"}
                  </Button>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </Card>
    </div>
  )
}
