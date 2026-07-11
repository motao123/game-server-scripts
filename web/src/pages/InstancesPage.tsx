import { Button, Card, Form, Input, List, Tabs, Tag, Upload, message, Modal } from 'antd'
import { UploadOutlined } from '@ant-design/icons'
import { useEffect, useState } from 'react'
import { PageHeader } from '../components/PageHeader'
import { api } from '../api'

export default function InstancesPage() {
  const [players, setPlayers] = useState<any[]>([])
  const [saves, setSaves] = useState<any[]>([])
  const [whitelist, setWhitelist] = useState<any[]>([])
  const [banlist, setBanlist] = useState<any[]>([])
  const [config, setConfig] = useState<any>({ categories: [] })
  async function refresh() {
    const [p, s, w, b, c] = await Promise.all([
      api<{ players: any[] }>('/api/players'), api<{ saves: any[] }>('/api/saves'), api<{ whitelist: any[] }>('/api/whitelist'), api<{ banlist: any[] }>('/api/banlist'), api<any>('/api/config')
    ])
    setPlayers(p.players || []); setSaves(s.saves || []); setWhitelist(w.whitelist || []); setBanlist(b.banlist || []); setConfig(c)
  }
  useEffect(() => { refresh() }, [])
  async function post(path: string, body: any, ok: string) { try { const d = await api<any>(path, { method: 'POST', body }); message.success(d.message || ok); refresh() } catch (e: any) { message.error(e.message) } }
  return <>
    <PageHeader title="Palworld 实例" desc="玩家、存档、配置、白名单和封禁管理" actions={<Button onClick={refresh}>刷新</Button>} />
    <Tabs items={[
      { key: 'players', label: '玩家', children: <Card><List dataSource={players} renderItem={(p) => <List.Item actions={[<Button onClick={() => post('/api/kick', { steamid: p.steamid }, '已踢出')}>踢出</Button>, <Button danger onClick={() => post('/api/ban', { steamid: p.steamid }, '已封禁')}>封禁</Button>]}><List.Item.Meta title={p.name || p.steamid} description={`SteamID: ${p.steamid || '-'} PlayerUID: ${p.playeruid || '-'}`} /></List.Item>} /></Card> },
      { key: 'saves', label: '存档', children: <Saves saves={saves} refresh={refresh} /> },
      { key: 'config', label: '配置', children: <ConfigView config={config} /> },
      { key: 'whitelist', label: '白名单', children: <Whitelist whitelist={whitelist} post={post} /> },
      { key: 'banlist', label: '封禁', children: <Card><List dataSource={banlist} renderItem={(b) => <List.Item actions={[<Button onClick={() => post('/api/banlist/unban', { steamid: b.steamid }, '已解封')}>解封</Button>]}>{b.steamid}</List.Item>} /></Card> }
    ]} />
  </>
}

function Saves({ saves, refresh }: { saves: any[]; refresh: () => void }) {
  async function backup() { await api('/api/saves/backup', { method: 'POST', body: {} }); message.success('备份指令已发送'); refresh() }
  async function remove(name: string) { await api('/api/saves/delete', { method: 'POST', body: { name } }); message.success('已删除'); refresh() }
  async function upload(file: File) {
    const content = await new Promise<string>((resolve, reject) => { const r = new FileReader(); r.onload = () => resolve(String(r.result).split(',')[1]); r.onerror = reject; r.readAsDataURL(file) })
    await api('/api/saves/upload', { method: 'POST', body: { name: file.name, content } }); message.success('已上传'); refresh()
  }
  return <Card title="存档备份" extra={<><Button onClick={backup}>立即备份</Button><Upload showUploadList={false} beforeUpload={(file) => { upload(file); return false }}><Button icon={<UploadOutlined />}>上传</Button></Upload></>}>
    <List dataSource={saves} renderItem={(s) => <List.Item actions={[<Button href={`/api/saves/download?name=${encodeURIComponent(s.name)}`}>下载</Button>, <Button onClick={() => Modal.confirm({ title: '确认恢复？', content: s.name, onOk: () => api('/api/saves/restore', { method: 'POST', body: { name: s.name } }) })}>恢复</Button>, <Button danger onClick={() => remove(s.name)}>删除</Button>]}><List.Item.Meta title={s.name} description={`${new Date(s.time * 1000).toLocaleString()} · ${(s.size / 1048576).toFixed(2)} MB`} /></List.Item>} />
  </Card>
}

function ConfigView({ config }: { config: any }) {
  return <Card>{(config.categories || []).map((cat: any) => <Card key={cat.name} type="inner" title={cat.name} className="section-card"><div className="config-grid">{cat.items.map((it: any) => <div key={it.key}><Tag color="blue">{it.label}</Tag><span>{String(it.value)}</span></div>)}</div></Card>)}</Card>
}

function Whitelist({ whitelist, post }: { whitelist: any[]; post: (p: string, b: any, ok: string) => void }) {
  return <Card title="白名单" extra={<Button onClick={() => post('/api/whitelist/check', {}, '已检查')}>立即检查</Button>}>
    <Form layout="inline" onFinish={(v) => post('/api/whitelist/add', v, '已添加')}><Form.Item name="name"><Input placeholder="名称" /></Form.Item><Form.Item name="steamid"><Input placeholder="SteamID" /></Form.Item><Form.Item name="playeruid"><Input placeholder="PlayerUID" /></Form.Item><Button htmlType="submit" type="primary">添加</Button></Form>
    <List dataSource={whitelist} renderItem={(w) => <List.Item actions={[<Button onClick={() => post('/api/whitelist/remove', { steamid: w.steamid }, '已移除')}>移除</Button>]}>{w.name || '-'} · {w.steamid || '*'} · {w.playeruid || '*'}</List.Item>} />
  </Card>
}
