import { useEffect, useState } from 'react'
import { Button, Card, Col, Row, Statistic, Table, Tag, message } from 'antd'
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

export default function DashboardPage() {
  const [info, setInfo] = useState<SystemInfo | null>(null)
  const [instances, setInstances] = useState<any[]>([])

  async function refresh() {
    const [sys, inst] = await Promise.all([
      api<SystemInfo>('/api/system/info'),
      api<{ instances: any[] }>('/api/instances')
    ])
    setInfo(sys)
    setInstances(inst.instances || [])
  }
  useEffect(() => { refresh(); const id = setInterval(refresh, 30000); return () => clearInterval(id) }, [])

  const runningCount = instances.filter(i => i.status === 'running').length

  return <>
    <PageHeader title="仪表盘" desc="服务器系统资源与实例概览" actions={<Button onClick={refresh}>刷新</Button>} />
    <Row gutter={[16, 16]}>
      <Col xs={24} md={6}><Card><Statistic title="CPU" value={info?.cpuPercent || 0} precision={1} suffix="%" /></Card></Col>
      <Col xs={24} md={6}><Card><Statistic title="内存" value={info?.memory.percent || 0} precision={1} suffix="%" /></Card></Col>
      <Col xs={24} md={6}><Card><Statistic title="磁盘" value={info?.disk.percent || 0} precision={1} suffix="%" /></Card></Col>
      <Col xs={24} md={6}><Card><Statistic title="系统运行" value={formatUptime(info?.uptime || 0)} /></Card></Col>
    </Row>
    <Card className="section-card" title="实例概览" extra={<Tag color={runningCount > 0 ? 'green' : 'default'}>{runningCount} 个运行中 / {instances.length} 个实例</Tag>}>
      {instances.length === 0 ? (
        <div style={{ textAlign: 'center', color: '#999', padding: 24 }}>
          暂无实例。前往「实例管理」创建，或安装游戏服务器后会自动导入。
        </div>
      ) : (
        <Table
          rowKey="id"
          dataSource={instances}
          pagination={false}
          columns={[
            { title: '名称', dataIndex: 'name' },
            { title: '类型', dataIndex: 'instanceType' },
            { title: '状态', dataIndex: 'status', render: (s: string) => <Tag color={s === 'running' ? 'green' : s === 'stopped' ? 'default' : 'orange'}>{s || 'unknown'}</Tag> },
            { title: '工作目录', dataIndex: 'workingDirectory' }
          ]}
        />
      )}
    </Card>
  </>
}
