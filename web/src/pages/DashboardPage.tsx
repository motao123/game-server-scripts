import { useEffect, useState } from 'react'
import { Button, Card, Col, Input, InputNumber, Row, Space, Statistic, Switch, Table, Tag, message } from 'antd'
import { PageHeader } from '../components/PageHeader'
import { api } from '../api'

type SystemInfo = {
  cpuPercent: number
  memory: { total: number; used: number; percent: number }
  disk: { total: number; used: number; percent: number }
  uptime: number
}

function formatUptime(seconds: number) {
  if (!seconds) return '-'
  const d = Math.floor(seconds / 86400)
  const h = Math.floor((seconds % 86400) / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  if (d > 0) return `${d}天${h}时`
  if (h > 0) return `${h}时${m}分`
  return `${m}分`
}

function formatBytes(b: number) {
  if (!b) return '-'
  if (b < 1048576) return `${(b / 1024).toFixed(1)} KB`
  if (b < 1073741824) return `${(b / 1048576).toFixed(1)} MB`
  return `${(b / 1073741824).toFixed(2)} GB`
}

export default function DashboardPage() {
  const [info, setInfo] = useState<SystemInfo | null>(null)
  const [instances, setInstances] = useState<any[]>([])
  const [ports, setPorts] = useState('')
  const [processes, setProcesses] = useState('')
  const [history, setHistory] = useState<any[]>([])
  const [network, setNetwork] = useState<any[]>([])
  const [alerts, setAlerts] = useState<any[]>([])
  const [rules, setRules] = useState<any[]>([])
  const [portSearch, setPortSearch] = useState('')
  const [procSearch, setProcSearch] = useState('')

  async function refresh() {
    const [sys, inst, p, ps, h, a, r] = await Promise.all([
      api<SystemInfo>('/api/system/info'),
      api<{ instances: any[] }>('/api/instances'),
      api<{ raw: string }>('/api/system/ports'),
      api<{ raw: string }>('/api/system/processes'),
      api<{ points: any[] }>('/api/system/history'),
      api<{ alerts: any[] }>('/api/alerts/status'),
      api<{ rules: any[] }>('/api/alerts/rules'),
    ])
    setInfo(sys); setInstances(inst.instances || []); setPorts(p.raw || ''); setProcesses(ps.raw || '')
    setHistory(h.points || [])
    setAlerts(a.alerts || [])
    setRules(r.rules || [])
  }
  useEffect(() => { refresh(); const id = setInterval(refresh, 5000); return () => clearInterval(id) }, [])

  async function checkNetwork() {
    try {
      const d = await api<{ checks: any[] }>('/api/network/check')
      setNetwork(d.checks || [])
    } catch (e: any) { message.error(e.message) }
  }

  async function saveRules(nextRules = rules) {
    try {
      const d = await api<{ rules: any[] }>('/api/alerts/rules', { method: 'POST', body: { rules: nextRules } })
      setRules(d.rules || [])
      message.success('告警规则已保存')
      refresh()
    } catch (e: any) { message.error(e.message) }
  }

  const runningCount = instances.filter(i => i.status === 'running').length
  const portLines = ports.split('\n').filter(l => !portSearch || l.includes(portSearch))
  const procLines = processes.split('\n').filter(l => !procSearch || l.includes(procSearch)).slice(0, 30)
  const latest = history[history.length - 1]
  const triggeredAlerts = alerts.filter(a => a.triggered)

  return (
    <>
      <PageHeader title="仪表盘" desc="系统资源、实例、端口和进程概览，5 秒自动刷新" actions={<Button onClick={refresh}>刷新</Button>} />
      <Row gutter={[16, 16]}>
        <Col xs={12} md={6}><Card><Statistic title="CPU" value={info?.cpuPercent || 0} precision={1} suffix="%" valueStyle={{ color: (info?.cpuPercent || 0) > 80 ? '#C10015' : '#1976D2' }} /></Card></Col>
        <Col xs={12} md={6}><Card><Statistic title="内存" value={info?.memory.percent || 0} precision={1} suffix="%" valueStyle={{ color: (info?.memory.percent || 0) > 85 ? '#C10015' : '#1976D2' }} /></Card></Col>
        <Col xs={12} md={6}><Card><Statistic title="磁盘" value={info?.disk.percent || 0} precision={1} suffix="%" valueStyle={{ color: (info?.disk.percent || 0) > 90 ? '#C10015' : '#1976D2' }} /></Card></Col>
        <Col xs={12} md={6}><Card><Statistic title="系统运行" value={formatUptime(info?.uptime || 0)} /></Card></Col>
      </Row>
      <Row gutter={[16, 16]} style={{ marginTop: 16 }}>
        <Col xs={24} md={12}>
          <Card title="内存详情" size="small">
            <Statistic title="已用 / 总量" value={`${formatBytes(info?.memory.used || 0)} / ${formatBytes(info?.memory.total || 0)}`} />
          </Card>
        </Col>
        <Col xs={24} md={12}>
          <Card title="磁盘详情" size="small">
            <Statistic title="已用 / 总量" value={`${formatBytes(info?.disk.used || 0)} / ${formatBytes(info?.disk.total || 0)}`} />
          </Card>
        </Col>
      </Row>
      <Row gutter={[16, 16]} style={{ marginTop: 16 }}>
        <Col xs={24} lg={12}>
          <Card title="系统历史" size="small" extra={<Tag>{history.length} 个采样点</Tag>}>
            <Row gutter={12}>
              <Col span={8}><Statistic title="最近 CPU" value={latest?.cpuPercent || 0} precision={1} suffix="%" /></Col>
              <Col span={8}><Statistic title="最近内存" value={latest?.memoryPercent || 0} precision={1} suffix="%" /></Col>
              <Col span={8}><Statistic title="最近磁盘" value={latest?.diskPercent || 0} precision={1} suffix="%" /></Col>
            </Row>
          </Card>
        </Col>
        <Col xs={24} lg={12}>
          <Card title="网络检测" size="small" extra={<Button size="small" onClick={checkNetwork}>检测</Button>}>
            {network.length === 0 ? <div style={{ color: '#999' }}>点击检测 Steam、PaperMC、Mojang、Modrinth、GitHub、Docker Hub 连通性。</div> : network.map(n => (
              <div key={n.id} style={{ display: 'flex', justifyContent: 'space-between', borderBottom: '1px solid #f0f0f0', padding: '4px 0' }}>
                <span>{n.name}</span>
                <span><Tag color={n.ok ? 'green' : 'red'}>{n.ok ? '可达' : '失败'}</Tag>{n.status || '-'} · {n.latencyMs}ms</span>
              </div>
            ))}
          </Card>
        </Col>
      </Row>
      <Card className="section-card" title="告警规则" extra={<Tag color={triggeredAlerts.length ? 'red' : 'green'}>{triggeredAlerts.length ? `${triggeredAlerts.length} 条触发` : '正常'}</Tag>}>
        <Table rowKey="id" dataSource={rules} pagination={false} size="small" columns={[
          { title: '规则', dataIndex: 'name' },
          { title: '指标', dataIndex: 'metric', width: 140 },
          { title: '启用', width: 90, render: (_: any, rule: any, index: number) => <Switch checked={rule.enabled} onChange={checked => { const next = [...rules]; next[index] = { ...rule, enabled: checked }; setRules(next); saveRules(next) }} /> },
          { title: '阈值', width: 140, render: (_: any, rule: any, index: number) => <InputNumber value={rule.threshold} min={0} max={rule.metric === 'networkFailures' ? 20 : 100} onChange={value => { const next = [...rules]; next[index] = { ...rule, threshold: Number(value || 0) }; setRules(next) }} onBlur={() => saveRules()} /> },
          { title: '状态', render: (_: any, rule: any) => {
            const status = alerts.find(a => a.rule?.id === rule.id)
            return <Space><Tag color={status?.triggered ? 'red' : 'green'}>{status?.triggered ? '触发' : '正常'}</Tag><span style={{ color: '#666' }}>{status?.message || '-'}</span></Space>
          } },
        ]} />
      </Card>
      <Card className="section-card" title="实例概览" extra={<Tag color={runningCount > 0 ? 'green' : 'default'}>{runningCount} 个运行中 / {instances.length} 个实例</Tag>}>
        {instances.length === 0 ? (
          <div style={{ textAlign: 'center', color: '#999', padding: 24 }}>暂无实例。前往「实例管理」创建，或安装游戏服务器后会自动导入。</div>
        ) : (
          <Table rowKey="id" dataSource={instances} pagination={false} size="small" columns={[
            { title: '名称', dataIndex: 'name' },
            { title: '类型', dataIndex: 'instanceType' },
            { title: '状态', dataIndex: 'status', render: (s: string) => <Tag color={s === 'running' ? 'green' : s === 'error' ? 'red' : 'default'}>{s}</Tag> },
            { title: '工作目录', dataIndex: 'workingDirectory', ellipsis: true },
          ]} />
        )}
      </Card>
      <Row gutter={[16, 16]}>
        <Col xs={24} lg={12}>
          <Card title="监听端口" size="small" extra={<Input.Search placeholder="搜索端口" value={portSearch} onChange={e => setPortSearch(e.target.value)} size="small" style={{ width: 150 }} allowClear />}>
            <pre className="log-box" style={{ maxHeight: 300 }}>{portLines.join('\n')}</pre>
          </Card>
        </Col>
        <Col xs={24} lg={12}>
          <Card title="进程列表 (Top 30)" size="small" extra={<Input.Search placeholder="搜索进程" value={procSearch} onChange={e => setProcSearch(e.target.value)} size="small" style={{ width: 150 }} allowClear />}>
            <pre className="log-box" style={{ maxHeight: 300 }}>{procLines.join('\n')}</pre>
          </Card>
        </Col>
      </Row>
    </>
  )
}
