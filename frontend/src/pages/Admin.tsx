import { useEffect, useState } from "react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { useSearchParams } from "react-router"
import { Ban, Copy, Plus, RotateCcw, Search } from "lucide-react"
import { toast } from "sonner"
import { api } from "@/lib/api"
import type { AdminTagNode, AdminUserRow, Group, Invite, Tag } from "@/lib/types"
import { Button } from "@/components/ui/button"
import { Card } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Switch } from "@/components/ui/switch"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { Textarea } from "@/components/ui/textarea"
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog"
import { fmtDate } from "@/lib/format"

type Meta = { groups: Group[]; topTags: Tag[] }

export default function AdminPage() {
  const [params, setParams] = useSearchParams()
  const tab = params.get("tab") ?? "tags"
  const { data: meta } = useQuery({
    queryKey: ["adminMeta"],
    queryFn: () => api<Meta>("/api/admin/meta"),
  })

  return (
    <div>
      <h1 className="mb-4 text-xl font-bold">管理后台</h1>
      <Tabs value={tab} onValueChange={(v) => setParams({ tab: v })}>
        <TabsList className="mb-4 flex-wrap">
          <TabsTrigger value="tags">版块</TabsTrigger>
          <TabsTrigger value="users">用户</TabsTrigger>
          <TabsTrigger value="invites">邀请码</TabsTrigger>
          <TabsTrigger value="hidden">已删除</TabsTrigger>
          <TabsTrigger value="settings">设置</TabsTrigger>
        </TabsList>
        <TabsContent value="tags"><TagsTab meta={meta} /></TabsContent>
        <TabsContent value="users"><UsersTab meta={meta} /></TabsContent>
        <TabsContent value="invites"><InvitesTab /></TabsContent>
        <TabsContent value="hidden"><HiddenTab /></TabsContent>
        <TabsContent value="settings"><SettingsTab /></TabsContent>
      </Tabs>
    </div>
  )
}

// ---------- 版块 ----------

function TagsTab({ meta }: { meta?: Meta }) {
  const qc = useQueryClient()
  const { data } = useQuery({
    queryKey: ["adminTags"],
    queryFn: () => api<{ nodes: AdminTagNode[] }>("/api/admin/tags"),
  })

  const save = useMutation({
    mutationFn: (body: Record<string, unknown>) => api("/api/admin/tags", { json: body }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["adminTags"] })
      void qc.invalidateQueries({ queryKey: ["adminMeta"] })
      void qc.invalidateQueries({ queryKey: ["home"] })
      toast.success("已保存")
    },
    onError: (e) => toast.error((e as Error).message),
  })
  const del = useMutation({
    mutationFn: (id: number) => api(`/api/admin/tags/${id}/delete`, { method: "POST" }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["adminTags"] })
      toast.success("已删除")
    },
    onError: (e) => toast.error((e as Error).message),
  })

  return (
    <div className="space-y-4">
      <TagEditForm meta={meta} onSubmit={(body) => save.mutate(body)} />
      {(data?.nodes ?? []).map((node) => (
        <div key={node.tag.id}>
          <TagNodeRow node={node} meta={meta} onSave={(b) => save.mutate({ ...b, id: node.tag.id })} onDelete={() => del.mutate(node.tag.id)} />
          {node.children.map((ch) => (
            <div key={ch.tag.id} className="ml-6 border-l pl-4">
              <TagNodeRow node={ch} meta={meta} onSave={(b) => save.mutate({ ...b, id: ch.tag.id })} onDelete={() => del.mutate(ch.tag.id)} />
            </div>
          ))}
        </div>
      ))}
    </div>
  )
}

function TagNodeRow({
  node, meta, onSave, onDelete,
}: {
  node: AdminTagNode
  meta?: Meta
  onSave: (body: Record<string, unknown>) => void
  onDelete: () => void
}) {
  const [open, setOpen] = useState(false)
  const t = node.tag
  return (
    <Card className="mb-2 p-3">
      <div className="flex flex-wrap items-center gap-2">
        <span className="size-3 rounded" style={{ background: t.color || "#888" }} />
        <b>{t.name}</b>
        <code className="text-xs text-muted-foreground">/t/{t.slug}</code>
        <span className="text-xs text-muted-foreground">{t.discussionsCount} 主题</span>
        <Dialog open={open} onOpenChange={setOpen}>
          <DialogTrigger asChild>
            <Button variant="outline" size="sm" className="ml-auto">
              编辑
            </Button>
          </DialogTrigger>
          <DialogContent className="max-w-xl">
            <DialogHeader>
              <DialogTitle>编辑版块「{t.name}」</DialogTitle>
            </DialogHeader>
            <TagEditForm
              meta={meta}
              initial={{ ...node, id: t.id }}
              onSubmit={(b) => {
                onSave(b)
                setOpen(false)
              }}
            />
          </DialogContent>
        </Dialog>
        <Button
          variant="outline"
          size="sm"
          className="text-red-600"
          onClick={() => confirm(`确定删除版块「${t.name}」？`) && onDelete()}
        >
          删除
        </Button>
      </div>
    </Card>
  )
}

function TagEditForm({
  meta, initial, onSubmit,
}: {
  meta?: Meta
  initial?: { id?: number; tag: Tag; viewIds: number[]; postIds: number[] }
  onSubmit: (body: Record<string, unknown>) => void
}) {
  const editing = !!initial?.id
  const t = initial?.tag
  const [form, setForm] = useState({
    name: t?.name ?? "",
    slug: t?.slug ?? "",
    color: t?.color ?? "",
    position: String(t?.position ?? 10),
    parentId: "0",
    description: t?.description ?? "",
  })
  const [viewIds, setViewIds] = useState<number[]>(initial?.viewIds ?? [])
  const [postIds, setPostIds] = useState<number[]>(initial?.postIds ?? [])

  function toggle(list: number[], setList: (v: number[]) => void, id: number) {
    setList(list.includes(id) ? list.filter((x) => x !== id) : [...list, id])
  }

  return (
    <form
      className="space-y-4"
      onSubmit={(e) => {
        e.preventDefault()
        onSubmit({
          id: initial?.id ?? 0,
          name: form.name,
          slug: form.slug,
          color: form.color,
          position: Number(form.position) || 0,
          parentId: Number(form.parentId) || 0,
          description: form.description,
          viewGroups: viewIds.join(","),
          postGroups: postIds.join(","),
        })
      }}
    >
      <div className="grid gap-3 sm:grid-cols-2">
        <div className="space-y-1.5">
          <Label>名称</Label>
          <Input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} required />
        </div>
        <div className="space-y-1.5">
          <Label>URL 标识（留空自动生成）</Label>
          <Input value={form.slug} onChange={(e) => setForm({ ...form, slug: e.target.value })} />
        </div>
        <div className="space-y-1.5">
          <Label>颜色（如 #1e88e5）</Label>
          <Input value={form.color} onChange={(e) => setForm({ ...form, color: e.target.value })} placeholder="#1e88e5" />
        </div>
        <div className="space-y-1.5">
          <Label>排序</Label>
          <Input type="number" value={form.position} onChange={(e) => setForm({ ...form, position: e.target.value })} />
        </div>
        {!editing && (
          <div className="space-y-1.5">
            <Label>父版块</Label>
            <Select value={form.parentId} onValueChange={(v) => setForm({ ...form, parentId: v })}>
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="0">无（作为大版块）</SelectItem>
                {meta?.topTags.map((t) => (
                  <SelectItem key={t.id} value={String(t.id)}>
                    {t.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        )}
        <div className="space-y-1.5">
          <Label>描述</Label>
          <Input value={form.description} onChange={(e) => setForm({ ...form, description: e.target.value })} />
        </div>
      </div>
      {(["viewGroups", "postGroups"] as const).map((field) => {
        const list = field === "viewGroups" ? viewIds : postIds
        const setList = field === "viewGroups" ? setViewIds : setPostIds
        return (
          <div key={field}>
            <Label>{field === "viewGroups" ? "谁可以看" : "谁可以发帖"}</Label>
            <div className="mt-1.5 flex flex-wrap gap-3">
              {meta?.groups
                .filter((g) => g.id !== 2)
                .map((g) => (
                  <label key={g.id} className="flex items-center gap-1.5 text-sm">
                    <Switch
                      checked={list.includes(g.id)}
                      onCheckedChange={() => toggle(list, setList, g.id)}
                    />
                    {g.name}
                  </label>
                ))}
            </div>
            <p className="mt-1 text-xs text-muted-foreground">
              {field === "viewGroups" ? "不勾选 = 任何人（含游客）可见" : "不勾选 = 任何登录用户"}
            </p>
          </div>
        )
      })}
      <div className="flex justify-end">
        <Button type="submit">
          <Plus className="size-4" /> {editing ? "保存" : "创建版块"}
        </Button>
      </div>
    </form>
  )
}

// ---------- 用户 ----------

function UsersTab({ meta }: { meta?: Meta }) {
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

// ---------- 邀请码 ----------

function InvitesTab() {
  const qc = useQueryClient()
  const [count, setCount] = useState("1")
  const [days, setDays] = useState("")
  const { data } = useQuery({
    queryKey: ["adminInvites"],
    queryFn: () => api<{ invites: Invite[] }>("/api/admin/invites"),
  })

  const create = useMutation({
    mutationFn: () =>
      api<{ codes: string[] }>("/api/admin/invites", {
        json: { count: Number(count) || 1, expiresDays: Number(days) || 0 },
      }),
    onSuccess: (r) => {
      void qc.invalidateQueries({ queryKey: ["adminInvites"] })
      void navigator.clipboard?.writeText(r.codes.join("\n")).catch(() => {})
      toast.success(`已生成 ${r.codes.length} 个邀请码（已复制）`)
    },
    onError: (e) => toast.error((e as Error).message),
  })

  return (
    <div className="space-y-4">
      <Card className="flex flex-wrap items-end gap-3 p-4">
        <div className="space-y-1.5">
          <Label>数量</Label>
          <Input type="number" min={1} max={50} value={count} onChange={(e) => setCount(e.target.value)} className="w-24" />
        </div>
        <div className="space-y-1.5">
          <Label>有效天数（留空永久）</Label>
          <Input type="number" min={1} value={days} onChange={(e) => setDays(e.target.value)} className="w-36" />
        </div>
        <Button onClick={() => create.mutate()} disabled={create.isPending}>
          生成邀请码
        </Button>
      </Card>
      <Card className="p-0">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>邀请码</TableHead>
              <TableHead>创建时间</TableHead>
              <TableHead>有效期</TableHead>
              <TableHead>状态</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {(data?.invites ?? []).map((inv) => (
              <TableRow key={inv.code}>
                <TableCell>
                  <button
                    className="inline-flex items-center gap-1 font-mono hover:text-primary"
                    title="点击复制"
                    onClick={() => {
                      void navigator.clipboard?.writeText(inv.code)
                      toast.success("已复制")
                    }}
                  >
                    {inv.code} <Copy className="size-3" />
                  </button>
                </TableCell>
                <TableCell>{fmtDate(inv.createdAt)}</TableCell>
                <TableCell>{inv.expiresAt ? fmtDate(inv.expiresAt) : "永久"}</TableCell>
                <TableCell>
                  {inv.usedAt ? (
                    <span className="text-xs text-muted-foreground">已使用</span>
                  ) : inv.expired ? (
                    <span className="text-xs font-semibold text-red-600">已过期</span>
                  ) : (
                    <span className="text-xs font-semibold text-green-600">可用</span>
                  )}
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </Card>
    </div>
  )
}

// ---------- 已删除 ----------

function HiddenTab() {
  const qc = useQueryClient()
  const { data } = useQuery({
    queryKey: ["adminHidden"],
    queryFn: () => api<{ items: { id: number; title: string; slug: string; author: { username: string } | null; isHidden: boolean }[] }>("/api/admin/hidden"),
  })
  const restore = useMutation({
    mutationFn: (id: number) => api(`/api/admin/discussions/${id}/restore`, { method: "POST" }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["adminHidden"] })
      toast.success("已恢复")
    },
    onError: (e) => toast.error((e as Error).message),
  })
  return (
    <Card className="p-0">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>标题</TableHead>
            <TableHead>作者</TableHead>
            <TableHead>操作</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {(data?.items ?? []).map((d) => (
            <TableRow key={d.id}>
              <TableCell className="font-medium">{d.title}</TableCell>
              <TableCell>{d.author?.username}</TableCell>
              <TableCell>
                <Button variant="outline" size="sm" className="gap-1.5" onClick={() => restore.mutate(d.id)}>
                  <RotateCcw className="size-3.5" /> 恢复
                </Button>
              </TableCell>
            </TableRow>
          ))}
          {(data?.items.length ?? 0) === 0 && (
            <TableRow>
              <TableCell colSpan={3} className="py-8 text-center text-muted-foreground">
                没有已删除的帖子。
              </TableCell>
            </TableRow>
          )}
        </TableBody>
      </Table>
    </Card>
  )
}

// ---------- 设置 ----------

function SettingsTab() {
  const qc = useQueryClient()
  const { data } = useQuery({
    queryKey: ["adminSettings"],
    queryFn: () =>
      api<{ siteName: string; registration: "open" | "invite"; welcomeText: string }>("/api/admin/settings"),
  })
  const [siteName, setSiteName] = useState("")
  const [registration, setRegistration] = useState<"open" | "invite">("invite")
  const [welcomeText, setWelcomeText] = useState("")
  const [initialized, setInitialized] = useState(false)
  useEffect(() => {
    if (data && !initialized) {
      setSiteName(data.siteName)
      setRegistration(data.registration)
      setWelcomeText(data.welcomeText)
      setInitialized(true)
    }
  }, [data, initialized])

  const save = useMutation({
    mutationFn: () => api("/api/admin/settings", { json: { siteName, registration, welcomeText } }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["adminSettings"] })
      void qc.invalidateQueries({ queryKey: ["me"] })
      toast.success("已保存")
    },
    onError: (e) => toast.error((e as Error).message),
  })

  return (
    <Card className="max-w-xl space-y-4 p-5">
      <div className="space-y-1.5">
        <Label>站点名称</Label>
        <Input
          value={siteName}
          onChange={(e) => setSiteName(e.target.value)}
          placeholder="留空则使用 config.yaml 中的默认值"
        />
      </div>
      <div className="space-y-1.5">
        <Label>注册方式</Label>
        <Select value={registration} onValueChange={(v) => setRegistration(v as "open" | "invite")}>
          <SelectTrigger>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="invite">仅邀请码</SelectItem>
            <SelectItem value="open">开放注册（带验证码）</SelectItem>
          </SelectContent>
        </Select>
      </div>
      <div className="space-y-1.5">
        <Label>未登录欢迎语</Label>
        <Textarea
          value={welcomeText}
          onChange={(e) => setWelcomeText(e.target.value)}
          rows={3}
          placeholder="显示在首页欢迎横幅上，留空则使用默认文案"
        />
      </div>
      <div className="flex justify-end">
        <Button onClick={() => save.mutate()} disabled={save.isPending}>
          保存设置
        </Button>
      </div>
    </Card>
  )
}
