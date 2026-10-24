import { useRef, useState } from "react"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { Camera, Trash2 } from "lucide-react"
import { toast } from "sonner"
import { api } from "@/lib/api"
import { useSession } from "@/lib/session"
import { UserAvatar } from "@/components/DiscussionParts"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Textarea } from "@/components/ui/textarea"

export default function SettingsPage() {
  const { data: session } = useSession()
  const qc = useQueryClient()
  const me = session?.user
  const fileRef = useRef<HTMLInputElement>(null)
  const [bio, setBio] = useState(me?.bio ?? "")
  const [pwd, setPwd] = useState({ current: "", next: "", confirm: "" })

  const saved = useMutation({
    mutationFn: (body: Record<string, unknown>) => api("/api/me/profile", { json: body }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["me"] })
      toast.success("已保存")
    },
    onError: (e) => toast.error((e as Error).message),
  })

  const changePwd = useMutation({
    mutationFn: (body: Record<string, unknown>) => api("/api/me/password", { json: body }),
    onSuccess: () => {
      setPwd({ current: "", next: "", confirm: "" })
      toast.success("密码已修改，其他设备已退出登录")
    },
    onError: (e) => toast.error((e as Error).message),
  })

  const avatar = useMutation({
    mutationFn: (form: FormData) => api<{ avatar: string }>("/api/me/avatar", { form }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["me"] })
      toast.success("头像已更新")
    },
    onError: (e) => toast.error((e as Error).message),
  })

  if (!me) return null

  return (
    <div className="mx-auto max-w-2xl space-y-5">
      <h1 className="text-xl font-bold">账号设置</h1>

      <Card>
        <CardHeader>
          <CardTitle>个人资料</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="bio">签名档</Label>
            <Textarea
              id="bio"
              value={bio}
              onChange={(e) => setBio(e.target.value)}
              rows={3}
              maxLength={500}
              placeholder="一句话介绍自己（500 字以内）"
            />
          </div>
          <div className="flex justify-end">
            <Button onClick={() => saved.mutate({ bio })} disabled={saved.isPending}>
              保存资料
            </Button>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>头像</CardTitle>
        </CardHeader>
        <CardContent className="flex items-center gap-5">
          <UserAvatar user={me} className="size-20" />
          <div className="space-y-2">
            <input
              ref={fileRef}
              type="file"
              accept="image/png,image/jpeg"
              className="hidden"
              onChange={() => {
                const f = fileRef.current?.files?.[0]
                if (f) {
                  const fd = new FormData()
                  fd.append("file", f)
                  avatar.mutate(fd)
                }
              }}
            />
            <div className="flex gap-2">
              <Button variant="outline" size="sm" className="gap-1.5" onClick={() => fileRef.current?.click()}>
                <Camera className="size-3.5" /> 上传头像
              </Button>
              {me.avatar.startsWith("/avatars/") && (
                <Button
                  variant="outline"
                  size="sm"
                  className="gap-1.5 text-red-600"
                  onClick={() => {
                    const fd = new FormData()
                    fd.append("action", "remove")
                    avatar.mutate(fd)
                  }}
                >
                  <Trash2 className="size-3.5" /> 恢复默认
                </Button>
              )}
            </div>
            <p className="text-xs text-muted-foreground">PNG / JPG，不超过 2MB，自动裁剪为 128×128</p>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>修改密码</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="current">当前密码</Label>
            <Input
              id="current"
              type="password"
              value={pwd.current}
              onChange={(e) => setPwd({ ...pwd, current: e.target.value })}
              autoComplete="current-password"
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="next">新密码</Label>
            <Input
              id="next"
              type="password"
              value={pwd.next}
              onChange={(e) => setPwd({ ...pwd, next: e.target.value })}
              minLength={8}
              autoComplete="new-password"
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="confirm">确认新密码</Label>
            <Input
              id="confirm"
              type="password"
              value={pwd.confirm}
              onChange={(e) => setPwd({ ...pwd, confirm: e.target.value })}
              minLength={8}
              autoComplete="new-password"
            />
          </div>
          <div className="flex justify-end">
            <Button
              onClick={() => changePwd.mutate(pwd)}
              disabled={changePwd.isPending || !pwd.current || !pwd.next}
            >
              修改密码
            </Button>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
