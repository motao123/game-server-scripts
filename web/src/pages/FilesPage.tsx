import { Button, Card, Dropdown, Input, Modal, Space, Table, Tag, message } from 'antd'
import { DownloadOutlined, FileAddOutlined, FolderAddOutlined, ReloadOutlined, UploadOutlined } from '@ant-design/icons'
import { useEffect, useRef, useState } from 'react'
import MonacoEditor from '@monaco-editor/react'
import { PageHeader } from '../components/PageHeader'
import { api } from '../api'

type FileItem = { name: string; path: string; isDir: boolean; size: number; modTime: string }

export default function FilesPage() {
  const [path, setPath] = useState('')
  const [items, setItems] = useState<FileItem[]>([])
  const [loading, setLoading] = useState(false)
  const [selected, setSelected] = useState<string[]>([])
  const [search, setSearch] = useState('')
  const [clipboard, setClipboard] = useState<{ path: string; op: 'copy' | 'move' } | null>(null)
  const [editFile, setEditFile] = useState<{ path: string; content: string; encoding?: any } | null>(null)
  const [editContent, setEditContent] = useState('')
  const [permModal, setPermModal] = useState<{ open: boolean; path: string; mode: string }>({ open: false, path: '', mode: '' })
  const [favorites, setFavorites] = useState<string[]>([])
  const fileInputRef = useRef<HTMLInputElement>(null)

  async function load(p?: string) {
    setLoading(true)
    try {
      const d = await api<{ path: string; items: FileItem[] }>('/api/files/list' + (p ? `?path=${encodeURIComponent(p)}` : ''))
      setPath(d.path)
      setItems(d.items || [])
    } catch (e: any) { message.error(e.message) } finally { setLoading(false) }
  }
  useEffect(() => { load(); loadFavorites() }, [])
  async function loadFavorites() { try { const d = await api<{ favorites: string[] }>('/api/files/favorites'); setFavorites(d.favorites || []) } catch {} }
  async function toggleFav(p: string) {
    if (favorites.includes(p)) await api('/api/files/favorites?path=' + encodeURIComponent(p), { method: 'DELETE' })
    else await api('/api/files/favorites', { method: 'POST', body: { path: p } })
    loadFavorites()
  }
  async function paste() {
    if (!clipboard) return
    const name = clipboard.path.split('/').pop()
    const dst = path + '/' + name
    try {
      if (clipboard.op === 'copy') await api('/api/files/copy', { method: 'POST', body: { src: clipboard.path, dst } })
      else await api('/api/files/move', { method: 'POST', body: { src: clipboard.path, dst } })
      message.success(clipboard.op === 'copy' ? '已复制' : '已移动')
      setClipboard(null); load()
    } catch (e: any) { message.error(e.message) }
  }
  async function showPermissions(item: FileItem) {
    try { const d = await api<{ mode: string }>(`/api/files/permissions?path=${encodeURIComponent(item.path)}`); setPermModal({ open: true, path: item.path, mode: d.mode }) }
    catch (e: any) { message.error(e.message) }
  }
  async function savePermissions() {
    try { await api('/api/files/permissions', { method: 'POST', body: { path: permModal.path, mode: permModal.mode } }); message.success('已保存'); setPermModal({ open: false, path: '', mode: '' }) }
    catch (e: any) { message.error(e.message) }
  }

  const filtered = items.filter(i => !search || i.name.toLowerCase().includes(search.toLowerCase()))
  const dirs = filtered.filter(i => i.isDir).sort((a, b) => a.name.localeCompare(b.name))
  const files = filtered.filter(i => !i.isDir).sort((a, b) => a.name.localeCompare(b.name))
  const sorted = [...dirs, ...files]

  function breadcrumb() {
    const parts = path.split('/').filter(Boolean)
    const items = [{ label: '/', path: '/' }]
    let cur = ''
    for (const p of parts) { cur += '/' + p; items.push({ label: p, path: cur }) }
    return items
  }

  async function open(item: FileItem) {
    if (item.isDir) { load(item.path); setSelected([]) }
    else {
      try {
        const d = await api<{ path: string; content: string; encoding: any }>('/api/files/read?path=' + encodeURIComponent(item.path))
        setEditFile({ path: d.path, content: d.content, encoding: d.encoding })
        setEditContent(d.content)
      } catch (e: any) { message.error(e.message) }
    }
  }

  async function saveFile() {
    if (!editFile) return
    try {
      await api('/api/files/write', { method: 'POST', body: { path: editFile.path, content: editContent } })
      message.success('已保存')
      setEditFile(null)
    } catch (e: any) { message.error(e.message) }
  }

  async function del(path: string) {
    Modal.confirm({
      title: '确认删除', content: path, okText: '删除', okType: 'danger', cancelText: '取消',
      onOk: async () => {
        try { await api('/api/files/delete', { method: 'POST', body: { path } }); message.success('已删除'); load() }
        catch (e: any) { message.error(e.message) }
      },
    })
  }

  async function rename(item: FileItem) {
    let newName = item.name
    Modal.confirm({
      title: '重命名', content: <Input defaultValue={item.name} onChange={e => newName = e.target.value} />,
      onOk: async () => {
        const newPath = path + '/' + newName
        try { await api('/api/files/rename', { method: 'POST', body: { oldPath: item.path, newPath } }); message.success('已重命名'); load() }
        catch (e: any) { message.error(e.message) }
      },
    })
  }

  async function compress(item: FileItem) {
    try { await api('/api/files/compress', { method: 'POST', body: { path: item.path } }); message.success('已压缩'); load() }
    catch (e: any) { message.error(e.message) }
  }

  async function extract(item: FileItem) {
    try { await api('/api/files/extract', { method: 'POST', body: { archive: item.path } }); message.success('已解压'); load() }
    catch (e: any) { message.error(e.message) }
  }

  async function mkdir() {
    let name = '新文件夹'
    Modal.confirm({
      title: '新建文件夹', content: <Input defaultValue={name} onChange={e => name = e.target.value} />,
      onOk: async () => {
        try { await api('/api/files/mkdir', { method: 'POST', body: { path: path + '/' + name } }); message.success('已创建'); load() }
        catch (e: any) { message.error(e.message) }
      },
    })
  }

  async function mkfile() {
    let name = '新文件.txt'
    Modal.confirm({
      title: '新建文件', content: <Input defaultValue={name} onChange={e => name = e.target.value} />,
      onOk: async () => {
        try { await api('/api/files/write', { method: 'POST', body: { path: path + '/' + name, content: '' } }); message.success('已创建'); load() }
        catch (e: any) { message.error(e.message) }
      },
    })
  }

  async function upload(file: File) {
    const formData = new FormData()
    formData.append('file', file)
    try {
      const csrf = localStorage.getItem('csrf')
      const resp = await fetch(`/api/files/upload?path=${encodeURIComponent(path)}`, {
        method: 'POST', body: formData, credentials: 'same-origin', headers: csrf ? { 'X-CSRF-Token': csrf } : {},
      })
      const d = await resp.json()
      if (d.ok) { message.success(`已上传 ${d.name}`); load() } else { message.error(d.error || '上传失败') }
    } catch (e: any) { message.error(e.message) }
  }

  function contextMenu(item: FileItem) {
    return {
      items: [
        { key: 'open', label: item.isDir ? '打开' : '编辑', onClick: () => open(item) },
        { key: 'rename', label: '重命名 (F2)', onClick: () => rename(item) },
        { key: 'download', label: '下载', onClick: () => { window.open(`/api/files/download?path=${encodeURIComponent(item.path)}`) } },
        !item.isDir && item.name.endsWith('.tar.gz') ? { key: 'extract', label: '解压', onClick: () => extract(item) } : null,
        { key: 'compress', label: '压缩为 tar.gz', onClick: () => compress(item) },
        { type: 'divider' as const },
        { key: 'delete', label: '删除', danger: true, onClick: () => del(item.path) },
      ].filter(Boolean),
    }
  }

  const columns = [
    {
      title: '名称', dataIndex: 'name', render: (name: string, record: FileItem) => (
        <Space>
          <span>{record.isDir ? '📁' : '📄'}</span>
          <a onClick={() => open(record)}>{name}</a>
        </Space>
      ),
    },
    { title: '大小', dataIndex: 'size', width: 100, render: (s: number, r: FileItem) => r.isDir ? '-' : `${(s / 1024).toFixed(1)} KB` },
    { title: '修改时间', dataIndex: 'modTime', width: 180, render: (t: string) => t ? new Date(t).toLocaleString('zh-CN') : '-' },
    {
      title: '操作', width: 280, render: (_: any, record: FileItem) => (
        <Space size="small" wrap>
          <Button size="small" type="link" onClick={() => open(record)}>{record.isDir ? '打开' : '编辑'}</Button>
          <Button size="small" type="link" onClick={() => setClipboard({ path: record.path, op: 'copy' })}>复制</Button>
          <Button size="small" type="link" onClick={() => setClipboard({ path: record.path, op: 'move' })}>剪切</Button>
          <Button size="small" type="link" onClick={() => toggleFav(record.path)}>{favorites.includes(record.path) ? '★' : '☆'}</Button>
          <Button size="small" type="link" onClick={() => showPermissions(record)}>权限</Button>
          <Button size="small" type="link" onClick={() => del(record.path)} danger>删除</Button>
        </Space>
      ),
    },
  ]

  return (
    <>
      <PageHeader title="文件管理" desc="受限根目录浏览/编辑/上传/下载/压缩解压" actions={
        <Space>
          <Button icon={<ReloadOutlined />} onClick={() => load()}>刷新</Button>
          {clipboard && <Button onClick={paste}>粘贴 {clipboard.op === 'copy' ? '(复制)' : '(移动)'}</Button>}
          <Button icon={<FolderAddOutlined />} onClick={mkdir}>新建文件夹</Button>
          <Button icon={<FileAddOutlined />} onClick={mkfile}>新建文件</Button>
          <Button icon={<UploadOutlined />} onClick={() => fileInputRef.current?.click()}>上传</Button>
          <input ref={fileInputRef} type="file" style={{ display: 'none' }} onChange={e => { if (e.target.files?.[0]) { upload(e.target.files[0]); e.target.value = '' } }} />
        </Space>
      } />
      <Card className="section-card">
        <Space style={{ width: '100%' }}>
          {breadcrumb().map((b, i) => (
            <span key={i}>
              <a onClick={() => load(b.path)}>{b.label}</a>
              {i < breadcrumb().length - 1 && ' / '}
            </span>
          ))}
        </Space>
        <Input.Search placeholder="搜索当前目录" value={search} onChange={e => setSearch(e.target.value)} style={{ marginTop: 8 }} allowClear />
      </Card>
      <Card>
        <Table
          rowKey="path"
          dataSource={sorted}
          columns={columns}
          loading={loading}
          pagination={false}
          size="small"
          rowSelection={{
            selectedRowKeys: selected,
            onChange: (keys) => setSelected(keys as string[]),
          }}
          onRow={(record) => ({
            onDoubleClick: () => open(record),
            onContextMenu: (e) => { e.preventDefault() },
          })}
        />
      </Card>
      <Modal
        title={editFile?.path}
        open={!!editFile}
        onOk={saveFile}
        onCancel={() => setEditFile(null)}
        width={900}
        okText="保存"
        cancelText="取消"
        destroyOnClose
      >
        {editFile && (
          <>
            {editFile.encoding && (
              <div style={{ marginBottom: 8 }}>
                <Tag color="blue">编码: {editFile.encoding.encoding}</Tag>
                {editFile.encoding.hasBom && <Tag color="green">BOM</Tag>}
                {editFile.encoding.isIncompatible && <Tag color="orange">编码不兼容</Tag>}
              </div>
            )}
            <div style={{ height: 500, border: '1px solid #d9d9d9' }}>
              <MonacoEditor
                language={getLanguage(editFile.path)}
                value={editContent}
                onChange={(v) => setEditContent(v || '')}
                theme="vs"
                options={{ minimap: { enabled: false }, fontSize: 13, wordWrap: 'on' }}
              />
            </div>
          </>
        )}
      </Modal>
      <Modal title="文件权限" open={permModal.open} onOk={savePermissions} onCancel={() => setPermModal({ open: false, path: '', mode: '' })} okText="保存" cancelText="取消">
        <p>{permModal.path}</p>
        <Input value={permModal.mode} onChange={e => setPermModal({ ...permModal, mode: e.target.value })} placeholder="如 755 或 0644" />
      </Modal>
    </>
  )
}

function getLanguage(path: string): string {
  const ext = path.split('.').pop()?.toLowerCase()
  const map: Record<string, string> = {
    js: 'javascript', ts: 'typescript', json: 'json', html: 'html', css: 'css',
    sh: 'shell', bash: 'shell', yml: 'yaml', yaml: 'yaml', xml: 'xml',
    md: 'markdown', py: 'python', go: 'go', java: 'java', ini: 'ini',
    cfg: 'ini', conf: 'ini', properties: 'ini', txt: 'plaintext',
  }
  return map[ext || ''] || 'plaintext'
}
