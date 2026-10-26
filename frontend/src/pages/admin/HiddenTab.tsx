import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { RotateCcw } from "lucide-react"
import { toast } from "sonner"
import { api } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { Card } from "@/components/ui/card"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"

export function HiddenTab() {
  const qc = useQueryClient()
  const { data } = useQuery({
    queryKey: ["adminHidden"],
    queryFn: () => api<{ items: { id: number; title: string; slug: string; author: { username: string } | null; isHidden: boolean }[] }>("/api/admin/hidden"),
  })
  const restore = useMutation({
    mutationFn: (id: number) => api(`/api/admin/discussions/${id}/restore`, { method: "POST" }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["adminHidden"] })
      toast.success("已恢复")
    },
    onError: (e) => toast.error((e as Error).message),
  })
  return (
    <Card className="p-0">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>标题</TableHead>
            <TableHead>作者</TableHead>
            <TableHead>操作</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {(data?.items ?? []).map((d) => (
            <TableRow key={d.id}>
              <TableCell className="font-medium">{d.title}</TableCell>
              <TableCell>{d.author?.username}</TableCell>
              <TableCell>
                <Button variant="outline" size="sm" className="gap-1.5" onClick={() => restore.mutate(d.id)}>
                  <RotateCcw className="size-3.5" /> 恢复
                </Button>
              </TableCell>
            </TableRow>
          ))}
          {(data?.items.length ?? 0) === 0 && (
            <TableRow>
              <TableCell colSpan={3} className="py-8 text-center text-muted-foreground">
                没有已删除的帖子。
              </TableCell>
            </TableRow>
          )}
        </TableBody>
      </Table>
    </Card>
  )
}
