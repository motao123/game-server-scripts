import { Button, Card, Col, Input, Modal, Progress, Row, Select, Space, Tabs, Tag, message } from 'antd'
import { useEffect, useRef, useState } from 'react'
import { PageHeader } from '../components/PageHeader'
import { api } from '../api'

type DeployState = {
  taskId: string
  gameName: string
  status: 'running' | 'success' | 'failed'
  output: string
  error: string
  instanceId?: string
}

export default function DeploymentPage() {
  const [games, setGames] = useState<any[]>([])
  const [onlineTemplates, setOnlineTemplates] = useState<any[]>([])
  const [steamcmd, setSteamcmd] = useState<string>('')
  const [target, setTarget] = useState<any | null>(null)
  const [deploy, setDeploy] = useState<DeployState | null>(null)
  const [loading, setLoading] = useState(false)
  const [deployPath, setDeployPath] = useState('')
  const [serverType, setServerType] = useState('')
  const [version, setVersion] = useState('')
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null)

  async function load() {
    const [g, s, o] = await Promise.all([api<{ games: any[] }>('/api/games'), api<any>('/api/steamcmd/status'), api<{ templates: any[] }>('/api/online-templates')])
    setGames(g.games || []); setSteamcmd(s.steamcmd || ''); setOnlineTemplates(o.templates || [])
  }
  useEffect(() => { load(); return () => { if (pollRef.current) clearInterval(pollRef.current) } }, [])

  function openDeploy(game: any) {
    setTarget(game)
    setDeployPath(game.defaultPath || '')
    setServerType(game.serverTypes?.[0] || '')
    setVersion(game.version || '')
  }

  async function startDeploy() {
    if (!target) return
    setLoading(true)
    try {
      const d = await api<any>('/api/game-deployment/install', { method: 'POST', body: { gameId: target.id, path: deployPath, serverType, version } })
      setDeploy({ taskId: d.taskId, gameName: target.name, status: 'running', output: '', error: '' })
      setTarget(null)
      poll(d.taskId)
    } catch (e: any) {
      message.error(e.message)
    } finally {
      setLoading(false)
    }
  }

  async function startOnlineDeploy(template: any) {
    setLoading(true)
    try {
      const d = await api<any>('/api/online-templates/deploy', { method: 'POST', body: { id: template.id, path: template.defaultPath } })
      setDeploy({ taskId: d.taskId, gameName: template.name, status: 'running', output: '', error: '' })
      poll(d.taskId)
    } catch (e: any) {
      message.error(e.message)
    } finally {
      setLoading(false)
    }
  }

  function poll(taskId: string) {
    if (pollRef.current) clearInterval(pollRef.current)
    pollRef.current = setInterval(async () => {
      try {
        const t = await api<any>(`/api/game-deployment/status?taskId=${taskId}`)
        setDeploy(prev => prev ? { ...prev, status: t.status, output: t.output || '', error: t.error || '', instanceId: t.instanceId } : prev)
        if (t.status === 'success' || t.status === 'failed') {
          if (pollRef.current) { clearInterval(pollRef.current); pollRef.current = null }
          if (t.status === 'success') message.success('部署成功，已自动创建实例')
          else message.error('部署失败：' + (t.error || '未知错误'))
        }
      } catch { /* ignore */ }
    }, 1500)
  }

  return (
    <>
      <PageHeader title="游戏部署" desc="游戏模板和在线模板部署，实时显示部署进度" actions={<Button onClick={load}>刷新</Button>} />
      <Card className="section-card">
        <span>SteamCMD: </span>
        <Tag color={steamcmd ? 'green' : 'red'}>{steamcmd ? '已安装' : '未安装（请先在环境管理安装）'}</Tag>
        <span style={{ marginLeft: 16 }}>共 {games.length} 个游戏模板</span>
      </Card>
      <Tabs items={[
        { key: 'games', label: '游戏模板', children: <Row gutter={[16, 16]}>
          {games.map(game => (
            <Col xs={24} sm={12} lg={8} key={game.id}>
              <Card
                title={<span>{game.name} <Tag color="blue">{game.id}</Tag></span>}
                hoverable
                onClick={() => openDeploy(game)}
                style={{ height: '100%' }}
              >
                <p style={{ color: '#666', minHeight: 44 }}>{game.nameCN ? `${game.nameCN} - ` : ''}{game.description}</p>
                <div style={{ marginBottom: 8 }}>
                  {(game.tags || []).map((t: string) => <Tag color="geekblue" key={t} style={{ marginBottom: 4 }}>{t}</Tag>)}
                  {(game.systems || []).map((t: string) => <Tag color="green" key={t} style={{ marginBottom: 4 }}>{t}</Tag>)}
                  {game.memoryGB ? <Tag color="orange" style={{ marginBottom: 4 }}>{game.memoryGB}GB+</Tag> : null}
                </div>
                <div style={{ fontSize: 12, color: '#999' }}>
                  <div>端口: {formatPorts(game)}</div>
                  <div>默认路径: {game.defaultPath}</div>
                  {game.appId ? <div>Steam AppID: {game.appId}</div> : null}
                  {game.storeUrl || game.docsUrl ? <div style={{ marginTop: 4 }}>
                    {game.storeUrl ? <a href={game.storeUrl} target="_blank" onClick={e => e.stopPropagation()}>商店</a> : null}
                    {game.storeUrl && game.docsUrl ? <span> · </span> : null}
                    {game.docsUrl ? <a href={game.docsUrl} target="_blank" onClick={e => e.stopPropagation()}>文档</a> : null}
                  </div> : null}
                </div>
                <Button type="primary" block style={{ marginTop: 12 }} onClick={(e) => { e.stopPropagation(); openDeploy(game) }}>部署</Button>
              </Card>
            </Col>
          ))}
        </Row> },
        { key: 'online', label: '在线模板', children: <Row gutter={[16, 16]}>
          {onlineTemplates.map(template => (
            <Col xs={24} sm={12} lg={8} key={template.id}>
              <Card title={<span>{template.name} <Tag color="purple">{template.id}</Tag></span>} style={{ height: '100%' }}>
                <p style={{ color: '#666', minHeight: 44 }}>{template.description}</p>
                <div style={{ marginBottom: 8 }}>{(template.tags || []).map((t: string) => <Tag key={t}>{t}</Tag>)}</div>
                <div style={{ fontSize: 12, color: '#999' }}>
                  <div>默认路径: {template.defaultPath}</div>
                  <div>版本: {template.version || '-'}</div>
                  <div>作者: {template.author || '-'}</div>
                </div>
                <Button type="primary" block loading={loading} style={{ marginTop: 12 }} onClick={() => startOnlineDeploy(template)}>部署</Button>
              </Card>
            </Col>
          ))}
        </Row> },
      ]} />

      <Modal
        title={target ? `部署 ${target.name}` : ''}
        open={!!target}
        onOk={startDeploy}
        onCancel={() => setTarget(null)}
        confirmLoading={loading}
        okText="开始部署"
        cancelText="取消"
      >
        {target && (
          <div>
            <p>将部署 <b>{target.name}</b> 到以下路径：</p>
            <Input value={deployPath} onChange={e => setDeployPath(e.target.value)} style={{ marginBottom: 12 }} />
            {target.id === 'minecraft-java' && <Space direction="vertical" style={{ width: '100%', marginBottom: 12 }}>
              <Select value={serverType} onChange={setServerType} options={(target.serverTypes || []).map((t: string) => ({ value: t, label: t }))} />
              <Input value={version} onChange={e => setVersion(e.target.value)} placeholder="Minecraft 版本，如 1.21.4" />
            </Space>}
            <p style={{ color: '#999', fontSize: 12 }}>
              {target.appId ? `Steam AppID: ${target.appId}，` : ''}端口: {formatPorts(target)}
            </p>
            {target.tip && <p style={{ color: '#666', fontSize: 12, whiteSpace: 'pre-wrap' }}>{target.tip}</p>}
            {!steamcmd && target.appId ? (
              <p style={{ color: '#C10015', fontSize: 12 }}>
                ⚠ SteamCMD 未安装，部署会失败。请先在「环境管理」页面安装 SteamCMD。
              </p>
            ) : target.id === 'minecraft-java' ? (
              <p style={{ color: '#666', fontSize: 12 }}>
                Minecraft Java 会下载所选服务端到 server.jar，写入 eula.txt，并自动创建可启动实例。
              </p>
            ) : (
              <p style={{ color: '#666', fontSize: 12 }}>
                部署将通过 SteamCMD 下载游戏服务端，安装完成后自动创建实例。可在下方实时查看安装进度。
              </p>
            )}
          </div>
        )}
      </Modal>

      {deploy && (
        <Card title={`部署进度 - ${deploy.gameName}`} className="section-card" style={{ marginTop: 16 }}>
          <div style={{ marginBottom: 12 }}>
            {deploy.status === 'running' && <Progress percent={99} status="active" strokeColor="#1976D2" />}
            {deploy.status === 'success' && <Progress percent={100} status="success" />}
            {deploy.status === 'failed' && <Progress percent={100} status="exception" />}
          </div>
          <div style={{ marginBottom: 8 }}>
            状态:
            {deploy.status === 'running' && <Tag color="processing" style={{ marginLeft: 8 }}>安装中...</Tag>}
            {deploy.status === 'success' && <Tag color="success" style={{ marginLeft: 8 }}>成功</Tag>}
            {deploy.status === 'failed' && <Tag color="error" style={{ marginLeft: 8 }}>失败</Tag>}
            {deploy.instanceId && <Tag color="blue" style={{ marginLeft: 8 }}>实例: {deploy.instanceId.slice(0, 8)}</Tag>}
          </div>
          {deploy.status === 'failed' && deploy.error && (
            <div style={{ marginBottom: 8, color: '#C10015' }}>
              失败原因: {deploy.error}
            </div>
          )}
          <pre className="console-box" style={{ maxHeight: 400 }}>{deploy.output || '等待输出...'}</pre>
          {deploy.status !== 'running' && (
            <Button onClick={() => setDeploy(null)} style={{ marginTop: 8 }}>关闭</Button>
          )}
        </Card>
      )}
    </>
  )
}

function formatPorts(game: any) {
  if (game.portMappings?.length) return game.portMappings.map((p: any) => `${p.port}/${p.protocol || 'tcp/udp'}`).join(', ')
  if (game.ports?.length) return game.ports.join(', ')
  return '未提供'
}
