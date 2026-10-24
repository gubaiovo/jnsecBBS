import { useState } from "react"
import { useMutation } from "@tanstack/react-query"
import { Link, useLocation, useNavigate } from "react-router"
import { toast } from "sonner"
import { api } from "@/lib/api"
import type { User } from "@/lib/types"
import { useInvalidateSession, useSession } from "@/lib/session"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"

export default function LoginPage() {
  const navigate = useNavigate()
  const location = useLocation()
  const invalidate = useInvalidateSession()
  const { data: session } = useSession()
  const [username, setUsername] = useState("")
  const [password, setPassword] = useState("")

  const login = useMutation({
    mutationFn: () => api<{ user: User }>("/api/auth/login", { json: { username, password } }),
    onSuccess: () => {
      invalidate()
      toast.success("登录成功")
      const from = (location.state as { from?: string } | null)?.from ?? "/"
      navigate(from)
    },
    onError: (e) => toast.error((e as Error).message),
  })

  return (
    <div className="mx-auto mt-10 max-w-sm">
      <Card>
        <CardHeader>
          <CardTitle>登录 {session?.siteName ?? ""}</CardTitle>
        </CardHeader>
        <CardContent>
          <form
            className="space-y-4"
            onSubmit={(e) => {
              e.preventDefault()
              if (username && password) login.mutate()
            }}
          >
            <div className="space-y-2">
              <Label htmlFor="username">用户名</Label>
              <Input id="username" value={username} onChange={(e) => setUsername(e.target.value)} autoComplete="username" required />
            </div>
            <div className="space-y-2">
              <Label htmlFor="password">密码</Label>
              <Input
                id="password"
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                autoComplete="current-password"
                required
              />
            </div>
            <Button type="submit" className="w-full" disabled={login.isPending}>
              登录
            </Button>
          </form>
          {session?.registration === "open" && (
            <p className="mt-4 text-center text-sm text-muted-foreground">
              还没有账号？<Link to="/register" className="text-primary hover:underline">立即注册</Link>
            </p>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
