import { Card, Descriptions, Button } from 'antd'
import { useEffect, useState } from 'react'
import { PageHeader } from '../components/PageHeader'
import { api } from '../api'

export default function SettingsPage() {
  const [settings, setSettings] = useState<any>({})
  async function load() { setSettings(await api('/api/settings')) }
  useEffect(() => { load() }, [])
  return <>
    <PageHeader title="设置" desc="面板运行参数、路径和安全配置" actions={<Button onClick={load}>刷新</Button>} />
    <Card><Descriptions column={1} bordered items={Object.entries(settings).map(([key, value]) => ({ key, label: key, children: String(value) }))} /></Card>
  </>
}
