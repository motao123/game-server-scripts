import { Button, Card, Form, Input, List, Tag, message } from 'antd'
import { useEffect, useState } from 'react'
import { PageHeader } from '../components/PageHeader'
import { api } from '../api'

export default function DeploymentPage() {
  const [games, setGames] = useState<any[]>([])
  const [status, setStatus] = useState<any>({})
  async function load() { const [g, s] = await Promise.all([api<{ games: any[] }>('/api/games'), api('/api/game-deployment/status')]); setGames(g.games || []); setStatus(s) }
  async function deploy(values: any) { const d = await api<any>('/api/game-deployment/install', { method: 'POST', body: values }); message.success(d.message || '部署任务已创建') }
  useEffect(() => { load() }, [])
  return <>
    <PageHeader title="游戏部署" desc="SteamCMD/模板部署入口" actions={<Button onClick={load}>刷新</Button>} />
    <Card className="section-card" title="部署任务">
      <Form layout="inline" onFinish={deploy}>
        <Form.Item name="gameId" rules={[{ required: true }]}><Input placeholder="gameId，如 palworld" /></Form.Item>
        <Form.Item name="path"><Input placeholder="安装路径" /></Form.Item>
        <Button type="primary" htmlType="submit">创建部署任务</Button>
      </Form>
      <p>SteamCMD: {status.steamcmd || '未检测到'}</p>
    </Card>
    <Card><List dataSource={games} renderItem={game => <List.Item actions={(game.tags || []).map((t: string) => <Tag color="blue" key={t}>{t}</Tag>)}><List.Item.Meta title={`${game.name} (${game.id})`} description={`${game.description} · 端口 ${(game.ports || []).join(', ')}`} /></List.Item>} /></Card>
  </>
}
