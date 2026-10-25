import { BrowserRouter, Link, Navigate, Route, Routes, useLocation, useParams } from "react-router"
import { ThemeProvider } from "next-themes"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { Toaster } from "@/components/ui/sonner"
import { TooltipProvider } from "@/components/ui/tooltip"
import { Navbar } from "@/components/Navbar"
import { useSession } from "@/lib/session"

import HomePage from "@/pages/Home"
import DiscussionPage from "@/pages/Discussion"
import NewPostPage from "@/pages/NewPost"
import EditPostPage from "@/pages/EditPost"
import LoginPage from "@/pages/Login"
import RegisterPage from "@/pages/Register"
import ProfilePage from "@/pages/Profile"
import SettingsPage from "@/pages/Settings"
import NotificationsPage from "@/pages/Notifications"
import SearchPage from "@/pages/Search"
import AdminPage from "@/pages/Admin"
import { Button } from "@/components/ui/button"

const queryClient = new QueryClient({
  defaultOptions: {
    queries: { retry: 1, refetchOnWindowFocus: false },
  },
})

function NotFound() {
  return (
    <div className="py-24 text-center">
      <h1 className="text-2xl font-bold">页面不存在</h1>
      <p className="mt-2 text-muted-foreground">你访问的页面不存在或已被删除。</p>
      <Button asChild className="mt-4">
        <Link to="/">返回首页</Link>
      </Button>
    </div>
  )
}

// 内容类页面守卫：未登录一律跳登录页（携带来源，登录后回跳）
// 板块主页已并入首页三栏：旧链接 /t/{slug} 重定向到首页并选中对应板块
function TagRedirect() {
  const { slug = "" } = useParams()
  return <Navigate to={`/?board=${encodeURIComponent(slug)}`} replace />
}

function RequireAuth({ children }: { children: React.ReactNode }) {
  const { data, isLoading } = useSession()
  const location = useLocation()
  if (isLoading) return <div className="py-20 text-center text-muted-foreground">加载中…</div>
  if (!data?.user) {
    return <Navigate to="/login" state={{ from: location.pathname + location.search }} replace />
  }
  return <>{children}</>
}

function RequireAdmin({ children }: { children: React.ReactNode }) {
  const { data, isLoading } = useSession()
  if (isLoading) return <div className="py-20 text-center text-muted-foreground">加载中…</div>
  if (!data?.user) return <LoginPage />
  if (!data.user.isAdmin) return <NotFound />
  return <>{children}</>
}

function RequireWritable({ children }: { children: React.ReactNode }) {
  const { data, isLoading } = useSession()
  if (isLoading) return <div className="py-20 text-center text-muted-foreground">加载中…</div>
  if (data?.readOnly) return <p className="py-12 text-center text-muted-foreground">迁移期间新站只读，请前往旧站进行修改。</p>
  return <>{children}</>
}

function Shell() {
  const { data } = useSession()
  const siteName = data?.siteName ?? "JNSEC BBS"
  return (
    <div className="min-h-screen bg-background text-foreground">
      <Navbar siteName={siteName} />
      <main className="mx-auto min-h-[calc(100vh-3.5rem)] max-w-6xl px-4 py-6">
        {data?.readOnly && (
          <div role="status" className="mb-5 rounded-lg border border-primary/20 bg-primary/5 px-4 py-3 text-sm">
            迁移期间新站只读，每 5 分钟同步旧站数据。
            <a href="/switch-to-old" className="ml-2 font-medium text-primary underline underline-offset-2">前往旧站发布或修改</a>
          </div>
        )}
        <Routes>
          <Route path="/" element={<HomePage />} />
          <Route path="/t/:slug" element={<RequireAuth><TagRedirect /></RequireAuth>} />
          <Route path="/d/:idSlug" element={<RequireAuth><DiscussionPage /></RequireAuth>} />
          <Route path="/post" element={<RequireAuth><RequireWritable><NewPostPage /></RequireWritable></RequireAuth>} />
          <Route path="/posts/:id/edit" element={<RequireAuth><RequireWritable><EditPostPage /></RequireWritable></RequireAuth>} />
          <Route path="/login" element={<LoginPage />} />
          <Route path="/register" element={<RequireWritable><RegisterPage /></RequireWritable>} />
          <Route path="/u/:username" element={<RequireAuth><ProfilePage /></RequireAuth>} />
          <Route path="/settings" element={<RequireAuth><RequireWritable><SettingsPage /></RequireWritable></RequireAuth>} />
          <Route path="/notifications" element={<RequireAuth><NotificationsPage /></RequireAuth>} />
          <Route path="/search" element={<RequireAuth><SearchPage /></RequireAuth>} />
          <Route path="/admin" element={<RequireAdmin><RequireWritable><AdminPage /></RequireWritable></RequireAdmin>} />
          <Route path="*" element={<NotFound />} />
        </Routes>
      </main>
    </div>
  )
}

export default function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <ThemeProvider attribute="class" defaultTheme="system" enableSystem>
        <TooltipProvider>
          <BrowserRouter>
            <Shell />
            <Toaster position="top-center" richColors />
          </BrowserRouter>
        </TooltipProvider>
      </ThemeProvider>
    </QueryClientProvider>
  )
}
