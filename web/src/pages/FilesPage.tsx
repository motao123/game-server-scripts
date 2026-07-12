import { Button, Card, Dropdown, Input, Modal, Progress, Space, Table, Tag, message } from 'antd'
import { DownloadOutlined, FileAddOutlined, FolderAddOutlined, ReloadOutlined, UploadOutlined } from '@ant-design/icons'
import { useEffect, useRef, useState } from 'react'
import MonacoEditor from '@monaco-editor/react'
import { PageHeader } from '../components/PageHeader'
import { api } from '../api'

type FileItem = { name: string; path: string; isDir: boolean; size: number; modTime: string }
type FileTask = { id: string; type: string; name: string; status: string; progress: number; message?: string; error?: string; updatedAt: string }
type SearchItem = { name: string; path: string; isDir: boolean; size: number }

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
  const [tasks, setTasks] = useState<FileTask[]>([])
  const [uploading, setUploading] = useState<{ name: string; progress: number } | null>(null)
  const [searchResults, setSearchResults] = useState<SearchItem[]>([])
  const [searching, setSearching] = useState(false)
  const [searchTruncated, setSearchTruncated] = useState(false)
  const fileInputRef = useRef<HTMLInputElement>(null)

  async function load(p?: string) {
    setLoading(true)
    try {
      const d = await api<{ path: string; items: FileItem[] }>('/api/files/list' + (p ? `?path=${encodeURIComponent(p)}` : ''))
      setPath(d.path)
      setItems(d.items || [])
    } catch (e: any) { message.error(e.message) } finally { setLoading(false) }
  }
  useEffect(() => { load(); loadFavorites(); loadTasks() }, [])
  useEffect(() => {
    const timer = setInterval(loadTasks, 2000)
    return () => clearInterval(timer)
  }, [])
  async function loadFavorites() { try { const d = await api<{ favorites: string[] }>('/api/files/favorites'); setFavorites(d.favorites || []) } catch {} }
  async function loadTasks() { try { const d = await api<{ tasks: FileTask[] }>('/api/files/tasks'); setTasks(d.tasks || []) } catch {} }
  async function toggleFav(p: string) {
    if (favorites.includes(p)) await api('/api/files/favorites?path=' + encodeURIComponent(p), { method: 'DELETE' })
    else await api('/api/files/favorites', { method: 'POST', body: { path: p } })
    loadFavorites()
  }
  async function paste() {
    if (!clipboard) return
    const name = clipboard.path.split('/').pop()
    const dst = path + '/' + name
    const run = async (overwrite = false) => {
      if (clipboard.op === 'copy') await api('/api/files/copy', { method: 'POST', body: { src: clipboard.path, dst, overwrite } })
      else await api('/api/files/move', { method: 'POST', body: { src: clipboard.path, dst, overwrite } })
      message.success(clipboard.op === 'copy' ? '已加入复制任务' : '已加入移动任务')
      setClipboard(null); load(); loadTasks()
    }
    try {
      const check = await api<{ conflicts: string[] }>('/api/files/conflicts', { method: 'POST', body: { items: [clipboard.path], dest: path } })
      if ((check.conflicts || []).length) {
        Modal.confirm({ title: '目标已存在', content: <div>{check.conflicts.map(p => <div key={p}>{p}</div>)}</div>, okText: '覆盖', cancelText: '取消', onOk: () => run(true) })
      } else {
        await run(false)
      }
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
    try { await api('/api/files/compress', { method: 'POST', body: { path: item.path } }); message.success('已加入压缩任务'); loadTasks() }
    catch (e: any) { message.error(e.message) }
  }

  async function extract(item: FileItem) {
    try { await api('/api/files/extract', { method: 'POST', body: { archive: item.path } }); message.success('已加入解压任务'); loadTasks() }
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
    const chunkSize = 8 * 1024 * 1024
    const totalChunks = Math.max(1, Math.ceil(file.size / chunkSize))
    const uploadId = `${Date.now()}-${Math.random().toString(16).slice(2)}-${file.name.replace(/[^\w.-]/g, '_')}`
    try {
      const csrf = localStorage.getItem('csrf')
      setUploading({ name: file.name, progress: 0 })
      for (let i = 0; i < totalChunks; i++) {
        const formData = new FormData()
        formData.append('uploadId', uploadId)
        formData.append('chunkIndex', String(i))
        formData.append('chunk', file.slice(i * chunkSize, Math.min(file.size, (i + 1) * chunkSize)))
        const resp = await fetch(`/api/files/upload-chunk?path=${encodeURIComponent(path)}`, {
          method: 'POST', body: formData, credentials: 'same-origin', headers: csrf ? { 'X-CSRF-Token': csrf } : {},
        })
        const d = await resp.json()
        if (!d.ok) throw new Error(d.error || '分片上传失败')
        setUploading({ name: file.name, progress: Math.round(((i + 1) / totalChunks) * 90) })
      }
      const finish = async (overwrite = false) => {
        await api('/api/files/upload-complete', { method: 'POST', body: { path, uploadId, fileName: file.name, totalChunks, overwrite } })
        message.success(`已加入合并任务 ${file.name}`)
        setUploading(null)
        loadTasks()
      }
      const check = await api<{ conflicts: string[] }>('/api/files/conflicts', { method: 'POST', body: { items: [path + '/' + file.name], dest: path } })
      if ((check.conflicts || []).length) {
        Modal.confirm({ title: '文件已存在', content: file.name, okText: '覆盖', cancelText: '取消', onOk: () => finish(true), onCancel: () => setUploading(null) })
      } else {
        await finish(false)
      }
    } catch (e: any) { setUploading(null); message.error(e.message) }
  }

  async function runSearch(value: string) {
    setSearch(value)
    setSearchResults([])
    setSearchTruncated(false)
    if (!value.trim()) return
    setSearching(true)
    try {
      const d = await api<{ items: SearchItem[]; truncated: boolean }>(`/api/files/search?path=${encodeURIComponent(path)}&q=${encodeURIComponent(value.trim())}`)
      setSearchResults(d.items || [])
      setSearchTruncated(!!d.truncated)
    } catch (e: any) { message.error(e.message) }
    finally { setSearching(false) }
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
        <Input.Search placeholder="搜索当前目录，回车后递归搜索" value={search} onChange={e => { setSearch(e.target.value); if (!e.target.value) setSearchResults([]) }} onSearch={runSearch} loading={searching} style={{ marginTop: 8 }} allowClear />
      </Card>
      {searchResults.length > 0 && <Card title="搜索结果" extra={searchTruncated ? <Tag color="orange">结果已截断</Tag> : undefined}>
        <ListSearchResults items={searchResults} open={open} />
      </Card>}
      {(uploading || tasks.length > 0) && <Card title="后台文件任务" extra={<Button size="small" onClick={loadTasks}>刷新</Button>}>
        {uploading && <div style={{ marginBottom: 12 }}>
          <Space style={{ width: '100%', justifyContent: 'space-between' }}><span>{uploading.name}</span><Tag color="blue">上传分片</Tag></Space>
          <Progress percent={uploading.progress} size="small" />
        </div>}
        {tasks.slice(0, 8).map(t => <div key={t.id} style={{ marginBottom: 12 }}>
          <Space style={{ width: '100%', justifyContent: 'space-between' }}>
            <span>{taskLabel(t.type)}：{t.name}</span>
            <Tag color={t.status === 'done' ? 'green' : t.status === 'error' ? 'red' : t.status === 'running' ? 'blue' : 'default'}>{statusLabel(t.status)}</Tag>
          </Space>
          <Progress percent={t.progress || 0} size="small" status={t.status === 'error' ? 'exception' : t.status === 'done' ? 'success' : 'active'} />
          {(t.error || t.message) && <div style={{ color: t.error ? '#cf1322' : '#666' }}>{t.error || t.message}</div>}
        </div>)}
      </Card>}
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

function ListSearchResults({ items, open }: { items: SearchItem[]; open: (item: FileItem) => void }) {
  return <Table
    rowKey="path"
    dataSource={items}
    pagination={{ pageSize: 8 }}
    size="small"
    columns={[
      { title: '名称', dataIndex: 'name', render: (name: string, record: SearchItem) => <Space><span>{record.isDir ? '📁' : '📄'}</span><a onClick={() => open({ ...record, modTime: '' })}>{name}</a></Space> },
      { title: '路径', dataIndex: 'path' },
      { title: '大小', dataIndex: 'size', width: 100, render: (s: number, r: SearchItem) => r.isDir ? '-' : `${(s / 1024).toFixed(1)} KB` },
    ]}
  />
}

function taskLabel(type: string): string {
  const map: Record<string, string> = { upload: '上传', copy: '复制', move: '移动', compress: '压缩', extract: '解压' }
  return map[type] || type
}

function statusLabel(status: string): string {
  const map: Record<string, string> = { queued: '排队中', running: '执行中', done: '完成', error: '失败' }
  return map[status] || status
}
