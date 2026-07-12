import { Button, Card, Descriptions, Progress, Space, Tag, message } from 'antd'
import { useEffect, useRef, useState } from 'react'
import { PageHeader } from '../components/PageHeader'
import { api } from '../api'

type TaskState = {
  taskId: string
  pkg: string
  status: 'running' | 'success' | 'failed'
  output: string
  error: string
}

export default function EnvironmentPage() {
  const [env, setEnv] = useState<any>({})
  const [task, setTask] = useState<TaskState | null>(null)
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null)

  async function load() { setEnv(await api('/api/environment/info')) }
  useEffect(() => { load() }, [])

  useEffect(() => () => { if (pollRef.current) clearInterval(pollRef.current) }, [])

  async function install(pkg: string) {
    if (task?.status === 'running') {
      message.warning('已有安装任务在执行')
      return
    }
    try {
      const d = await api<any>('/api/environment/install', { method: 'POST', body: { package: pkg } })
      setTask({ taskId: d.taskId, pkg, status: 'running', output: '', error: '' })
      poll(d.taskId)
    } catch (e: any) {
      message.error(e.message)
    }
  }

  function poll(taskId: string) {
    if (pollRef.current) clearInterval(pollRef.current)
    pollRef.current = setInterval(async () => {
      try {
        const t = await api<any>(`/api/environment/install/status?taskId=${taskId}`)
        setTask(prev => prev ? { ...prev, status: t.status, output: t.output || '', error: t.error || '' } : prev)
        if (t.status === 'success' || t.status === 'failed') {
          if (pollRef.current) { clearInterval(pollRef.current); pollRef.current = null }
          load()
          if (t.status === 'success') message.success('安装成功')
          else message.error('安装失败：' + (t.error || '未知错误'))
        }
      } catch { /* ignore */ }
    }, 1500)
  }

  const javaPackages = (env.javaVersions || []).map((j: any) => ({ key: j.package, label: `Java ${j.version}`, desc: j.path ? `已安装: ${j.path}` : 'Minecraft Java 服务端可选运行时', cmd: `openjdk-${j.version}-jre-headless`, installed: !!j.path }))
  const packages = [
    ...javaPackages,
    { key: 'steamcmd', label: 'SteamCMD', desc: 'Palworld/Valheim/Terraria 等 Steam 游戏需要', cmd: 'steamcmd', installed: !!env.steamcmd },
    { key: 'tools', label: '常用工具', desc: 'curl/wget/tar/gzip/unzip', cmd: 'curl wget tar gzip unzip', installed: !!env.tools },
  ]

  return (
    <>
      <PageHeader title="环境管理" desc="系统、Java、SteamCMD 和工具链检测与安装" actions={<Button onClick={load}>刷新</Button>} />
      <Card className="section-card">
        <Descriptions column={1} bordered items={[
          { key: 'os', label: '操作系统', children: String(env.os || '-') },
          { key: 'arch', label: '架构', children: String(env.arch || '-') },
          { key: 'java', label: '当前 Java', children: env.java ? <Tag color="green">已安装: {String(env.java)}</Tag> : <Tag color="default">未安装</Tag> },
          { key: 'javaVersions', label: 'Java 多版本', children: <Space wrap>{(env.javaVersions || []).map((j: any) => <Tag key={j.version} color={j.path ? 'green' : 'default'}>Java {j.version}{j.path ? ' 已安装' : ' 未安装'}</Tag>)}</Space> },
          { key: 'steamcmd', label: 'SteamCMD', children: env.steamcmd ? <Tag color="green">已安装: {String(env.steamcmd)}</Tag> : <Tag color="default">未安装</Tag> },
          { key: 'tools', label: '常用工具', children: env.tools ? <Tag color="green">已安装</Tag> : <Tag color="default">未完整安装</Tag> },
        ]} />
      </Card>
      <Card title="安装环境">
        <Space direction="vertical" style={{ width: '100%' }}>
          {packages.map(p => (
            <div key={p.key} style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '8px 0', borderBottom: '1px solid #f0f0f0' }}>
              <div>
                <b>{p.label}</b>
                <span style={{ color: '#999', marginLeft: 8 }}>{p.desc}</span>
                <div style={{ fontSize: 12, color: '#ccc', fontFamily: 'monospace' }}>{p.cmd}</div>
              </div>
              <Button
                type="primary"
                loading={task?.status === 'running' && task?.pkg === p.key}
                disabled={task?.status === 'running' && task?.pkg !== p.key}
                onClick={() => install(p.key)}
              >
                {p.installed ? '重装' : '安装'}
              </Button>
            </div>
          ))}
        </Space>
      </Card>
      {task && (
        <Card title={`安装进度 - ${task.pkg}`} className="section-card">
          <div style={{ marginBottom: 12 }}>
            {task.status === 'running' && <Progress percent={99} status="active" strokeColor="#1976D2" />}
            {task.status === 'success' && <Progress percent={100} status="success" />}
            {task.status === 'failed' && <Progress percent={100} status="exception" />}
          </div>
          <div style={{ marginBottom: 8 }}>
            状态:
            {task.status === 'running' && <Tag color="processing" style={{ marginLeft: 8 }}>安装中...</Tag>}
            {task.status === 'success' && <Tag color="success" style={{ marginLeft: 8 }}>成功</Tag>}
            {task.status === 'failed' && <Tag color="error" style={{ marginLeft: 8 }}>失败</Tag>}
          </div>
          {task.status === 'failed' && task.error && (
            <div style={{ marginBottom: 8, color: '#C10015' }}>
              失败原因: {task.error}
            </div>
          )}
          <pre className="console-box" style={{ maxHeight: 300 }}>{task.output || '等待输出...'}</pre>
        </Card>
      )}
    </>
  )
}
