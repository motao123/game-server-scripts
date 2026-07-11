import { Button, Card, Empty, List, Switch, message } from 'antd'
import { useEffect, useState } from 'react'
import { PageHeader } from '../components/PageHeader'
import { api } from '../api'

export default function PluginsPage() {
  const [plugins, setPlugins] = useState<any[]>([])
  async function load() { const d = await api<{ plugins: any[] }>('/api/plugins'); setPlugins(d.plugins || []) }
  useEffect(() => { load() }, [])
  async function toggle(id: string, enabled: boolean) {
    try { await api('/api/plugins/toggle', { method: 'POST', body: { id, enabled } }); message.success(enabled ? '已启用' : '已禁用'); load() } catch (e: any) { message.error(e.message) }
  }
  return (
    <>
      <PageHeader title="插件" desc="扫描 data/plugins/*/plugin.json，仅管理元数据，不执行插件代码" actions={<Button onClick={load}>刷新</Button>} />
      <Card>
        {plugins.length === 0 ? (
          <Empty description="未发现插件，请在 data/plugins/<插件名>/plugin.json 创建清单" />
        ) : (
          <List
            dataSource={plugins}
            renderItem={(p: any) => (
              <List.Item actions={[<Switch checked={p.enabled} onChange={(v) => toggle(p.id, v)} />]}>
                <List.Item.Meta title={`${p.name} ${p.version || ''}`} description={`${p.description || '-'} · 作者：${p.author || '-'} · 路径：${p.path}`} />
              </List.Item>
            )}
          />
        )}
      </Card>
    </>
  )
}
