import { useEffect, useState } from "react"
import { useMutation } from "@tanstack/react-query"
import { Link, useNavigate } from "react-router"
import { RefreshCw } from "lucide-react"
import { toast } from "sonner"
import { api } from "@/lib/api"
import type { User } from "@/lib/types"
import { useInvalidateSession, useSession } from "@/lib/session"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"

export default function RegisterPage() {
  const navigate = useNavigate()
  const invalidate = useInvalidateSession()
  const { data: session } = useSession()
  const inviteMode = session?.registration !== "open"
  const [form, setForm] = useState({
    invite: "", username: "", email: "", password: "", passwordConfirm: "",
    captchaId: "", captchaValue: "",
  })

  // 开放注册时预取一个验证码
  useEffect(() => {
    if (!inviteMode) newCaptcha()
  }, [inviteMode])

  async function newCaptcha() {
    try {
      const r = await api<{ id: string }>("/api/auth/captcha")
      setForm((f) => ({ ...f, captchaId: r.id, captchaValue: "" }))
    } catch {
      /* ignore */
    }
  }

  const register = useMutation({
    mutationFn: () =>
      api<{ user: User }>("/api/auth/register", {
        json: {
          username: form.username,
          email: form.email,
          password: form.password,
          passwordConfirm: form.passwordConfirm,
          invite: form.invite,
          captchaId: form.captchaId,
          captchaValue: form.captchaValue,
        },
      }),
    onSuccess: () => {
      invalidate()
      toast.success("注册成功，欢迎加入！")
      navigate("/")
    },
    onError: (e) => {
      toast.error((e as Error).message)
      if (!inviteMode) void newCaptcha()
    },
  })

  const set = (k: keyof typeof form) => (e: React.ChangeEvent<HTMLInputElement>) =>
    setForm((f) => ({ ...f, [k]: e.target.value }))

  return (
    <div className="mx-auto mt-10 max-w-sm">
      <Card>
        <CardHeader>
          <CardTitle>注册 {session?.siteName ?? ""}</CardTitle>
        </CardHeader>
        <CardContent>
          <form
            className="space-y-4"
            onSubmit={(e) => {
              e.preventDefault()
              register.mutate()
            }}
          >
            {inviteMode && (
              <div className="space-y-2">
                <Label htmlFor="invite">邀请码</Label>
                <Input id="invite" value={form.invite} onChange={set("invite")} placeholder="向管理员索取" required />
              </div>
            )}
            <div className="space-y-2">
              <Label htmlFor="username">用户名</Label>
              <Input id="username" value={form.username} onChange={set("username")} pattern="[A-Za-z0-9_\-]{2,20}" required autoComplete="username" />
              <p className="text-xs text-muted-foreground">2-20 位字母、数字、下划线或连字符</p>
            </div>
            <div className="space-y-2">
              <Label htmlFor="email">邮箱</Label>
              <Input id="email" type="email" value={form.email} onChange={set("email")} required autoComplete="email" />
            </div>
            <div className="space-y-2">
              <Label htmlFor="password">密码</Label>
              <Input id="password" type="password" value={form.password} onChange={set("password")} minLength={8} required autoComplete="new-password" />
            </div>
            <div className="space-y-2">
              <Label htmlFor="passwordConfirm">确认密码</Label>
              <Input id="passwordConfirm" type="password" value={form.passwordConfirm} onChange={set("passwordConfirm")} minLength={8} required autoComplete="new-password" />
            </div>
            {!inviteMode && (
              <div className="space-y-2">
                <Label htmlFor="captchaValue">验证码</Label>
                <div className="flex items-center gap-2">
                  {form.captchaId && (
                    <img
                      src={`/captcha/${form.captchaId}.png`}
                      alt="验证码"
                      title="点击刷新"
                      className="h-11 cursor-pointer rounded-md border"
                      onClick={newCaptcha}
                    />
                  )}
                  <Input id="captchaValue" value={form.captchaValue} onChange={set("captchaValue")} required autoComplete="off" />
                  <Button type="button" variant="ghost" size="icon" onClick={newCaptcha} aria-label="刷新验证码">
                    <RefreshCw />
                  </Button>
                </div>
              </div>
            )}
            <Button type="submit" className="w-full" disabled={register.isPending}>
              注册
            </Button>
          </form>
          <p className="mt-4 text-center text-sm text-muted-foreground">
            已有账号？<Link to="/login" className="text-primary hover:underline">直接登录</Link>
          </p>
        </CardContent>
      </Card>
    </div>
  )
}
