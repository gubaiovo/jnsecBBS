import { useState } from "react"
import { Eye, PenLine } from "lucide-react"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { Textarea } from "@/components/ui/textarea"
import { api } from "@/lib/api"
import { MarkdownBody } from "@/components/MarkdownBody"

// Markdown 编辑器：编写 + 服务端渲染预览（同一条消毒管线）
export function MarkdownEditor({
  value,
  onChange,
  placeholder,
  rows = 10,
}: {
  value: string
  onChange: (v: string) => void
  placeholder?: string
  rows?: number
}) {
  const [tab, setTab] = useState("write")
  const [preview, setPreview] = useState("")

  async function switchPreview(next: string) {
    setTab(next)
    if (next === "preview" && value.trim()) {
      try {
        const r = await api<{ html: string }>("/api/preview", { json: { content: value } })
        setPreview(r.html)
      } catch {
        setPreview("<p>预览失败</p>")
      }
    }
  }

  return (
    <Tabs value={tab} onValueChange={switchPreview}>
      <TabsList className="mb-2">
        <TabsTrigger value="write">
          <PenLine data-slot="icon" /> 编写
        </TabsTrigger>
        <TabsTrigger value="preview">
          <Eye data-slot="icon" /> 预览
        </TabsTrigger>
      </TabsList>
      <TabsContent value="write" className="mt-0">
        <Textarea
          value={value}
          onChange={(e) => onChange(e.target.value)}
          placeholder={placeholder ?? "支持 Markdown，@用户名 可提醒对方"}
          rows={rows}
          className="font-mono text-sm"
          required
        />
      </TabsContent>
      <TabsContent value="preview" className="mt-0">
        <div className="min-h-24 rounded-lg border bg-muted/30 p-4">
          {value.trim() ? <MarkdownBody html={preview} /> : <p className="text-sm text-muted-foreground">暂无内容</p>}
        </div>
      </TabsContent>
    </Tabs>
  )
}
