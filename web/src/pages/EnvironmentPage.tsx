import { Card, Descriptions, Button } from 'antd'
import { useEffect, useState } from 'react'
import { PageHeader } from '../components/PageHeader'
import { api } from '../api'

export default function EnvironmentPage() {
  const [env, setEnv] = useState<any>({})
  async function load() { setEnv(await api('/api/environment/info')) }
  useEffect(() => { load() }, [])
  return <>
    <PageHeader title="环境管理" desc="系统、Java、SteamCMD 和服务器依赖检测" actions={<Button onClick={load}>刷新</Button>} />
    <Card><Descriptions column={1} bordered items={Object.entries(env).map(([key, value]) => ({ key, label: key, children: String(value || '-') }))} /></Card>
  </>
}
