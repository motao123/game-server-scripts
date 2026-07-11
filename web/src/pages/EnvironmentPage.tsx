import { Button, Card, Descriptions, Space, message } from 'antd'
import { useEffect, useState } from 'react'
import { PageHeader } from '../components/PageHeader'
import { api } from '../api'

export default function EnvironmentPage() {
  const [env, setEnv] = useState<any>({})
  async function load() { setEnv(await api('/api/environment/info')) }
  useEffect(() => { load() }, [])
  async function install(pkg: string) {
    try { const d = await api<any>('/api/environment/install', { method: 'POST', body: { package: pkg } }); message.info(d.command || d.message) } catch (e: any) { message.error(e.message) }
  }
  return <>
    <PageHeader title="环境管理" desc="系统、Java、SteamCMD 和工具链检测" actions={<Button onClick={load}>刷新</Button>} />
    <Card className="section-card"><Descriptions column={1} bordered items={Object.entries(env).map(([key, value]) => ({ key, label: key, children: String(value || '-') }))} /></Card>
    <Card title="安装环境">
      <Space>
        <Button onClick={() => install('java')}>安装 Java 17</Button>
        <Button onClick={() => install('steamcmd')}>安装 SteamCMD</Button>
        <Button onClick={() => install('tools')}>安装 curl/wget/tar/unzip</Button>
      </Space>
      <p style={{ marginTop: 12, color: '#666' }}>出于安全考虑，面板不会直接执行 apt-get，会返回建议命令供你在服务器手动执行。</p>
    </Card>
  </>
}
