import { useState } from "react"
import { useMutation, useQuery } from "@tanstack/react-query"
import { useNavigate } from "react-router"
import { toast } from "sonner"
import { api } from "@/lib/api"
import type { PostForm } from "@/lib/types"
import { MarkdownEditor } from "@/components/MarkdownEditor"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Alert, AlertDescription } from "@/components/ui/alert"

export default function NewPostPage() {
  const navigate = useNavigate()
  const [title, setTitle] = useState("")
  const [content, setContent] = useState("")
  const [tagIds, setTagIds] = useState<number[]>([])

  const { data } = useQuery({
    queryKey: ["postForm"],
    queryFn: () => api<PostForm>("/api/post-form"),
  })

  const create = useMutation({
    mutationFn: () => api<{ id: number; url: string }>("/api/discussions", { json: { title, content, tagIds } }),
    onSuccess: (r) => {
      toast.success("发布成功")
      void api; // noop 避免移除导入
      navigate(r.url)
    },
    onError: (e) => toast.error((e as Error).message),
  })

  function toggleTag(id: number) {
    setTagIds((cur) => (cur.includes(id) ? cur.filter((x) => x !== id) : [...cur, id]))
  }

  return (
    <div className="mx-auto max-w-3xl">
      <h1 className="mb-4 text-xl font-bold">发新帖</h1>
      <Card>
        <CardHeader>
          <CardTitle>主题信息</CardTitle>
        </CardHeader>
        <CardContent className="space-y-5">
          <div className="space-y-2">
            <Label htmlFor="title">标题</Label>
            <Input
              id="title"
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              minLength={3}
              maxLength={200}
              placeholder="请输入标题（3-200 字）"
            />
          </div>

          <div className="space-y-2">
            <Label>版块（可多选）</Label>
            {(data?.groups.length ?? 0) === 0 && (
              <Alert>
                <AlertDescription>没有可选的版块，请联系管理员。</AlertDescription>
              </Alert>
            )}
            <div className="space-y-3">
              {data?.groups.map((g) => (
                <div key={g.parent.id}>
                  <div className="mb-1.5 text-xs font-semibold text-muted-foreground">{g.parent.name}</div>
                  <div className="flex flex-wrap gap-2">
                    {g.children.map((t) => (
                      <label
                        key={t.id}
                        className={`cursor-pointer rounded-full border px-3 py-1 text-xs font-medium transition-colors ${
                          tagIds.includes(t.id)
                            ? "border-primary bg-primary text-primary-foreground"
                            : "hover:bg-secondary"
                        }`}
                      >
                        <input
                          type="checkbox"
                          className="hidden"
                          checked={tagIds.includes(t.id)}
                          onChange={() => toggleTag(t.id)}
                        />
                        {t.name}
                      </label>
                    ))}
                  </div>
                </div>
              ))}
            </div>
          </div>

          <div className="space-y-2">
            <Label>正文</Label>
            <MarkdownEditor value={content} onChange={setContent} rows={12} />
          </div>

          <div className="flex justify-end">
            <Button
              onClick={() => create.mutate()}
              disabled={create.isPending || !title.trim() || !content.trim() || tagIds.length === 0}
            >
              发布
            </Button>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
