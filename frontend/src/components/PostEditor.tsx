import { useState } from "react"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"
import { api } from "@/lib/api"
import type { Post } from "@/lib/types"
import { MarkdownEditor } from "@/components/MarkdownEditor"
import { Button } from "@/components/ui/button"

export function PostEditor({ post, onClose }: { post: Post; onClose: () => void }) {
  const [content, setContent] = useState(post.content)
  const qc = useQueryClient()
  const save = useMutation({
    mutationFn: () => api(`/api/posts/${post.id}/edit`, { json: { content } }),
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: ["discussion", post.discussion.id] })
      toast.success("已保存")
      onClose()
    },
    onError: (e) => toast.error((e as Error).message),
  })

  return (
    <form
      aria-label={`编辑第 ${post.number} 楼回复`}
      onSubmit={(e) => {
        e.preventDefault()
        if (content.trim() && !save.isPending) save.mutate()
      }}
    >
      <fieldset disabled={save.isPending} className="space-y-3">
        <MarkdownEditor value={content} onChange={setContent} rows={8} />
        {save.isError && <p role="alert" className="text-sm text-destructive">{save.error.message}</p>}
        <div className="flex justify-end gap-2">
          <Button type="button" variant="outline" onClick={onClose}>取消</Button>
          <Button type="submit" disabled={!content.trim()}>{save.isPending ? "保存中…" : "保存"}</Button>
        </div>
      </fieldset>
    </form>
  )
}
