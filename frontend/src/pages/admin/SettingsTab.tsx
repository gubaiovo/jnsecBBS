import { useEffect, useState } from "react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"
import { api } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { Card } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Textarea } from "@/components/ui/textarea"

export function SettingsTab() {
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
