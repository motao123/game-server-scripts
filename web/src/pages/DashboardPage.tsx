import { useEffect, useState } from 'react'
import { Button, Card, Col, Row, Statistic, Tag, Typography, message } from 'antd'
import { PageHeader } from '../components/PageHeader'
import { api } from '../api'

type SystemInfo = { cpuPercent: number; memory: { total: number; used: number; percent: number }; disk: { total: number; used: number; percent: number }; uptime: number }
type Status = { active: boolean; uptime: string }

export default function DashboardPage() {
  const [info, setInfo] = useState<SystemInfo | null>(null)
  const [status, setStatus] = useState<Status | null>(null)
  const [players, setPlayers] = useState<any[]>([])
  const [logs, setLogs] = useState('')

  async function refresh() {
    const [sys, st, ps, lg] = await Promise.all([
      api<SystemInfo>('/api/system/info'), api<Status>('/api/status'), api<{ players: any[] }>('/api/players'), api<{ logs: string }>('/api/logs')
    ])
    setInfo(sys); setStatus(st); setPlayers(ps.players || []); setLogs(lg.logs || '')
  }
  async function action(path: string, label: string) {
    try { await api(path, { method: 'POST', body: {} }); message.success(`${label}已发送`); refresh() } catch (e: any) { message.error(e.message) }
  }
  useEffect(() => { refresh(); const id = setInterval(refresh, 30000); return () => clearInterval(id) }, [])

  return <>
    <PageHeader title="仪表盘" desc="服务器状态、系统资源、玩家和日志" actions={<Button onClick={refresh}>刷新</Button>} />
    <Row gutter={[16, 16]}>
      <Col xs={24} md={6}><Card><Statistic title="Palworld" value={status?.active ? '运行中' : '已停止'} valueStyle={{ color: status?.active ? '#21BA45' : '#C10015' }} /></Card></Col>
      <Col xs={24} md={6}><Card><Statistic title="CPU" value={info?.cpuPercent || 0} precision={1} suffix="%" /></Card></Col>
      <Col xs={24} md={6}><Card><Statistic title="内存" value={info?.memory.percent || 0} precision={1} suffix="%" /></Card></Col>
      <Col xs={24} md={6}><Card><Statistic title="磁盘" value={info?.disk.percent || 0} precision={1} suffix="%" /></Card></Col>
    </Row>
    <Card className="section-card" title="服务控制">
      <Button type="primary" onClick={() => action('/api/start', '启动')}>启动</Button>
      <Button onClick={() => action('/api/stop', '停止')}>停止</Button>
      <Button onClick={() => action('/api/restart', '重启')}>重启</Button>
      <Button onClick={() => action('/api/save', '保存')}>保存存档</Button>
    </Card>
    <Row gutter={[16, 16]}>
      <Col xs={24} lg={10}><Card title="在线玩家"><Typography.Text>{players.length} 人在线</Typography.Text><div className="tag-list">{players.map((p) => <Tag key={p.steamid || p.name}>{p.name || p.steamid}</Tag>)}</div></Card></Col>
      <Col xs={24} lg={14}><Card title="最近日志"><pre className="log-box">{logs}</pre></Card></Col>
    </Row>
  </>
}
