import { Button, Card, Descriptions, Form, Input, message, Space, Table, Tag } from 'antd'
import { useEffect, useState } from 'react'
import { PageHeader } from '../components/PageHeader'
import { api } from '../api'

export default function SettingsPage() {
  const [settings, setSettings] = useState<any>({})
  const [catalogs, setCatalogs] = useState<any[]>([])
  const [catalogUrls, setCatalogUrls] = useState<Record<string, string>>({})
  const [catalogLoading, setCatalogLoading] = useState<string>('')
  const [form] = Form.useForm()
  async function load() {
    const [settingsData, catalogData] = await Promise.all([api('/api/settings'), api<{ catalogs: any[] }>('/api/catalogs')])
    setSettings(settingsData)
    setCatalogs(catalogData.catalogs || [])
  }
  useEffect(() => { load() }, [])

  async function changePassword() {
    try {
      const v = await form.validateFields()
      await api('/api/settings/password', { method: 'POST', body: v })
      message.success('密码已修改'); form.resetFields()
    } catch (e: any) { if (e.errorFields?.length) return; message.error(e.message) }
  }

  async function reloadCatalog(type: string) {
    setCatalogLoading(type)
    try {
      const d = await api<{ catalogs: any[] }>('/api/catalogs/reload', { method: 'POST', body: { type } })
      setCatalogs(d.catalogs || [])
      message.success('清单已重载')
    } catch (e: any) { message.error(e.message) } finally { setCatalogLoading('') }
  }

  async function updateCatalog(type: string) {
    const url = catalogUrls[type]
    if (!url) { message.warning('请输入远程清单 URL'); return }
    setCatalogLoading(type)
    try {
      const d = await api<{ catalogs: any[]; result: any }>('/api/catalogs/update', { method: 'POST', body: { type, url } })
      setCatalogs(d.catalogs || [])
      message.success(`清单已更新，共 ${d.result?.count || 0} 条`)
    } catch (e: any) { message.error(e.message) } finally { setCatalogLoading('') }
  }

  return (
    <>
      <PageHeader title="设置" desc="面板配置与密码管理" actions={<Button onClick={load}>刷新</Button>} />
      <Card className="section-card" title="运行配置">
        <Descriptions column={1} bordered items={[
          { key: 'bind', label: '绑定地址', children: settings.bind },
          { key: 'port', label: '端口', children: settings.port },
          { key: 'dataDir', label: '数据目录', children: settings.dataDir },
          { key: 'palServerDir', label: 'Palworld 目录', children: settings.palServerDir },
          { key: 'backupDir', label: '备份目录', children: settings.backupDir },
          { key: 'rconPort', label: 'RCON 端口', children: settings.rconPort },
          { key: 'restApiPort', label: 'REST API 端口', children: settings.restApiPort },
          { key: 'service', label: 'systemd 服务', children: settings.service },
        ]} />
        {settings.fileRoots && (
          <div style={{ marginTop: 12 }}>
            <span>文件管理允许的根目录: </span>
            {(settings.fileRoots as string[]).map(r => <Tag color="blue" key={r}>{r}</Tag>)}
          </div>
        )}
        {settings.securityNotice && <div style={{ marginTop: 12, color: '#F2C037' }}>{settings.securityNotice}</div>}
      </Card>
      <Card className="section-card" title="清单管理">
        <Table rowKey="type" dataSource={catalogs} pagination={false} size="small" columns={[
          { title: '清单', dataIndex: 'name', width: 120 },
          { title: '数量', dataIndex: 'count', width: 90 },
          { title: '状态', width: 90, render: (_: any, row: any) => <Tag color={row.available ? 'green' : 'orange'}>{row.available ? '文件' : '内置'}</Tag> },
          { title: '路径', dataIndex: 'path', ellipsis: true },
          { title: '更新时间', dataIndex: 'updatedAt', width: 190, render: (v: string) => v ? new Date(v).toLocaleString() : '-' },
          { title: '远程更新', width: 360, render: (_: any, row: any) => <Space.Compact style={{ width: '100%' }}><Input placeholder="https://.../catalog.json" value={catalogUrls[row.type] || ''} onChange={e => setCatalogUrls(prev => ({ ...prev, [row.type]: e.target.value }))} /><Button loading={catalogLoading === row.type} onClick={() => updateCatalog(row.type)}>更新</Button></Space.Compact> },
          { title: '操作', width: 90, render: (_: any, row: any) => <Button loading={catalogLoading === row.type} onClick={() => reloadCatalog(row.type)}>重载</Button> },
        ]} />
      </Card>
      <Card title="修改密码">
        <Form form={form} layout="inline" onFinish={changePassword}>
          <Form.Item name="oldPassword" rules={[{ required: true }]}><Input.Password placeholder="旧密码" /></Form.Item>
          <Form.Item name="newPassword" rules={[{ required: true, min: 6 }]}><Input.Password placeholder="新密码 (至少6位)" /></Form.Item>
          <Button type="primary" htmlType="submit">修改</Button>
        </Form>
      </Card>
    </>
  )
}
