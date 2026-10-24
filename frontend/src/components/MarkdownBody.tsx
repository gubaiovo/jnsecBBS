// 服务端已消毒的 HTML 渲染容器
export function MarkdownBody({ html }: { html: string }) {
  return (
    <div
      className="md-body text-[15px] leading-relaxed break-words"
      dangerouslySetInnerHTML={{ __html: html }}
    />
  )
}
