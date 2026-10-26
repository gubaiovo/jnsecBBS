import { useQuery } from "@tanstack/react-query"
import { useSearchParams } from "react-router"
import { api } from "@/lib/api"
import type { Group, Tag } from "@/lib/types"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { TagsTab } from "./admin/TagsTab"
import { UsersTab } from "./admin/UsersTab"
import { InvitesTab } from "./admin/InvitesTab"
import { HiddenTab } from "./admin/HiddenTab"
import { SettingsTab } from "./admin/SettingsTab"

export type AdminMeta = { groups: Group[]; topTags: Tag[] }

export default function AdminPage() {
  const [params, setParams] = useSearchParams()
  const tab = params.get("tab") ?? "tags"
  const { data: meta } = useQuery({
    queryKey: ["adminMeta"],
    queryFn: () => api<AdminMeta>("/api/admin/meta"),
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
