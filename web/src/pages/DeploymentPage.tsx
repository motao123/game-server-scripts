import { Button, Card, Col, Modal, Row, Tag, message } from 'antd'
import { useEffect, useState } from 'react'
import { PageHeader } from '../components/PageHeader'
import { api } from '../api'

export default function DeploymentPage() {
  const [games, setGames] = useState<any[]>([])
  const [status, setStatus] = useState<any>({})
  const [target, setTarget] = useState<any | null>(null)
  const [loading, setLoading] = useState(false)
  async function load() {
    const [g, s] = await Promise.all([api<{ games: any[] }>('/api/games'), api('/api/game-deployment/status')])
    setGames(g.games || []); setStatus(s)
  }
  useEffect(() => { load() }, [])
  async function confirm() {
    if (!target) return
    setLoading(true)
    try {
      const d = await api<any>('/api/game-deployment/install', { method: 'POST', body: { gameId: target.id } })
      message.success(d.message || '部署任务已创建')
      setTarget(null)
    } catch (e: any) {
      message.error(e.message)
    } finally {
      setLoading(false)
    }
  }
  return (
    <>
      <PageHeader title="游戏部署" desc="点击游戏卡片即可部署到默认路径" actions={<Button onClick={load}>刷新</Button>} />
      <Card className="section-card">
        <span>SteamCMD: </span>
        <Tag color={status.steamcmd ? 'green' : 'default'}>{status.steamcmd ? '已安装' : '未检测到'}</Tag>
        <span style={{ marginLeft: 16 }}>共 {games.length} 个游戏模板</span>
      </Card>
      <Row gutter={[16, 16]}>
        {games.map(game => (
          <Col xs={24} sm={12} lg={8} key={game.id}>
            <Card
              title={<span>{game.name} <Tag color="blue">{game.id}</Tag></span>}
              hoverable
              onClick={() => setTarget(game)}
              style={{ height: '100%' }}
            >
              <p style={{ color: '#666', minHeight: 44 }}>{game.description}</p>
              <div style={{ marginBottom: 8 }}>
                {(game.tags || []).map((t: string) => <Tag color="geekblue" key={t} style={{ marginBottom: 4 }}>{t}</Tag>)}
              </div>
              <div style={{ fontSize: 12, color: '#999' }}>
                <div>端口: {(game.ports || []).join(', ')}</div>
                <div>默认路径: {game.defaultPath}</div>
                {game.appId ? <div>Steam AppID: {game.appId}</div> : null}
              </div>
              <Button type="primary" block style={{ marginTop: 12 }} onClick={(e) => { e.stopPropagation(); setTarget(game) }}>部署</Button>
            </Card>
          </Col>
        ))}
      </Row>
      <Modal
        title={target ? `部署 ${target.name}` : ''}
        open={!!target}
        onOk={confirm}
        onCancel={() => setTarget(null)}
        confirmLoading={loading}
        okText="开始部署"
        cancelText="取消"
      >
        {target && (
          <div>
            <p>将部署 <b>{target.name}</b> 到以下默认路径：</p>
            <p style={{ background: '#f5f5f5', padding: 8, borderRadius: 4, fontFamily: 'monospace' }}>{target.defaultPath}</p>
            <p style={{ color: '#999', fontSize: 12 }}>
              {target.appId ? `Steam AppID: ${target.appId}，` : ''}端口: {(target.ports || []).join(', ')}
            </p>
            <p style={{ color: '#F2C037', fontSize: 12 }}>
              注意：部署前请确保 SteamCMD 已安装。如未安装，可在「环境管理」页面获取安装命令。
            </p>
          </div>
        )}
      </Modal>
    </>
  )
}
