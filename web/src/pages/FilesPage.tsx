import { Card, Input, List, Button, message } from 'antd'
import { useEffect, useState } from 'react'
import { PageHeader } from '../components/PageHeader'
import { api } from '../api'

export default function FilesPage() {
  const [path, setPath] = useState('')
  const [items, setItems] = useState<any[]>([])
  const [content, setContent] = useState('')
  const [file, setFile] = useState('')
  async function list(p = path) { const d = await api<{ path: string; items: any[] }>('/api/files/list' + (p ? `?path=${encodeURIComponent(p)}` : '')); setPath(d.path); setItems(d.items || []) }
  async function read(p: string) { const d = await api<{ content: string; path: string }>('/api/files/read?path=' + encodeURIComponent(p)); setFile(d.path); setContent(d.content) }
  async function save() { await api('/api/files/write', { method: 'POST', body: { path: file, content } }); message.success('已保存') }
  useEffect(() => { list('') }, [])
  return <>
    <PageHeader title="文件管理" desc="默认限制在游戏目录、备份目录和配置目录内" actions={<Button onClick={() => list()}>刷新</Button>} />
    <Card className="section-card"><Input.Search value={path} onChange={e => setPath(e.target.value)} onSearch={list} enterButton="打开路径" /></Card>
    <div className="split">
      <Card title="目录"><List dataSource={items} renderItem={(it) => <List.Item actions={[it.isDir ? <Button onClick={() => list(it.path)}>打开</Button> : <Button onClick={() => read(it.path)}>编辑</Button>]}>{it.isDir ? '📁' : '📄'} {it.name}</List.Item>} /></Card>
      <Card title={file || '文本编辑'}><Input.TextArea value={content} onChange={e => setContent(e.target.value)} rows={18} /><Button type="primary" onClick={save} disabled={!file}>保存</Button></Card>
    </div>
  </>
}
