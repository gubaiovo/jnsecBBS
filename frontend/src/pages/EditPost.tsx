import { useEffect, useState } from "react"
import { useMutation, useQuery } from "@tanstack/react-query"
import { useNavigate, useParams } from "react-router"
import { toast } from "sonner"
import { api, discussionUrl } from "@/lib/api"
import { MarkdownEditor } from "@/components/MarkdownEditor"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"

export default function EditPostPage() {
  const { id = "" } = useParams()
  const navigate = useNavigate()
  const [content, setContent] = useState("")
  const [disc, setDisc] = useState<{ id: number; slug: string; title: string } | null>(null)

  const { data, isError, error } = useQuery({
    queryKey: ["postEdit", id],
    queryFn: () =>
      api<{ id: number; content: string; discussion: { id: number; slug: string; title: string } }>(
        `/api/posts/${id}/edit`,
      ),
    retry: false,
  })
  useEffect(() => {
    if (data) {
      setContent(data.content)
      setDisc(data.discussion)
    }
  }, [data])

  const save = useMutation({
    mutationFn: () => api(`/api/posts/${id}/edit`, { json: { content } }),
    onSuccess: () => {
      toast.success("已保存")
      if (disc) navigate(discussionUrl(disc.id, disc.slug))
      else navigate(-1)
    },
    onError: (e) => toast.error((e as Error).message),
  })

  if (isError) {
    return <div className="py-20 text-center text-destructive">{(error as Error).message}</div>
  }

  return (
    <div className="mx-auto max-w-3xl">
      <h1 className="mb-4 text-xl font-bold">编辑回复{disc ? ` · ${disc.title}` : ""}</h1>
      <Card>
        <CardHeader>
          <CardTitle>正文</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <MarkdownEditor value={content} onChange={setContent} rows={12} />
          <div className="flex justify-end gap-2">
            <Button variant="outline" onClick={() => navigate(-1)}>
              取消
            </Button>
            <Button onClick={() => save.mutate()} disabled={save.isPending || !content.trim()}>
              保存
            </Button>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
