import { Button, Card, Empty, Form, Input, List, Modal, Space, Switch, Tabs, Tag, message } from 'antd'
import { CloudDownloadOutlined, DeleteOutlined, PlusOutlined } from '@ant-design/icons'
import { useEffect, useState } from 'react'
import { PageHeader } from '../components/PageHeader'
import { api } from '../api'

export default function PluginsPage() {
  const [plugins, setPlugins] = useState<any[]>([])
  const [catalog, setCatalog] = useState<any[]>([])
  const [modalOpen, setModalOpen] = useState(false)
  const [form] = Form.useForm()
  async function load() {
    const [installed, market] = await Promise.all([
      api<{ plugins: any[] }>('/api/plugins'),
      api<{ plugins: any[] }>('/api/plugins/catalog'),
    ])
    setPlugins(installed.plugins || [])
    setCatalog(market.plugins || [])
  }
  useEffect(() => { load() }, [])
  async function toggle(id: string, enabled: boolean) {
    try { await api('/api/plugins/toggle', { method: 'POST', body: { id, enabled } }); message.success(enabled ? '已启用' : '已禁用'); load() } catch (e: any) { message.error(e.message) }
  }
  async function create() {
    try {
      const v = await form.validateFields()
      await api('/api/plugins/create', { method: 'POST', body: v })
      message.success('已创建'); setModalOpen(false); form.resetFields(); load()
    } catch (e: any) { if (e.errorFields?.length) return; message.error(e.message) }
  }
  async function del(name: string) {
    Modal.confirm({
      title: '确认删除插件', content: name, okText: '删除', okType: 'danger', cancelText: '取消',
      onOk: async () => { await api('/api/plugins/delete', { method: 'POST', body: { name } }); message.success('已删除'); load() },
    })
  }
  async function install(id: string) {
    try { await api('/api/plugins/install', { method: 'POST', body: { id } }); message.success('已安装'); load() } catch (e: any) { message.error(e.message) }
  }
  function tags(items?: string[]) {
    return (items || []).map(item => <Tag key={item}>{item}</Tag>)
  }
  return (
    <>
      <PageHeader title="插件" desc="插件元数据、启用状态和本地市场安装" actions={<Button icon={<PlusOutlined />} type="primary" onClick={() => setModalOpen(true)}>新建插件</Button>} />
      <Card>
        <Tabs items={[
          { key: 'installed', label: '已安装', children: plugins.length === 0 ? <Empty description="未发现插件" /> : (
            <List dataSource={plugins} renderItem={(p: any) => (
              <List.Item actions={[
                <Switch checked={p.enabled} onChange={(v) => toggle(p.id, v)} />,
                <Button danger icon={<DeleteOutlined />} onClick={() => del(p.id)}>删除</Button>,
              ]}>
                <List.Item.Meta
                  title={<Space wrap>{p.displayName || p.name} {p.version && <Tag>{p.version}</Tag>} {p.enabled && <Tag color="green">启用</Tag>} {tags(p.tags)}</Space>}
                  description={<Space direction="vertical" size={2}><span>{p.description || '-'}</span><span style={{ color: '#666' }}>作者: {p.author || '-'} · {p.path}</span></Space>}
                />
              </List.Item>
            )} />
          ) },
          { key: 'catalog', label: '插件市场', children: catalog.length === 0 ? <Empty description="未发现市场插件" /> : (
            <List dataSource={catalog} renderItem={(p: any) => (
              <List.Item actions={[
                <Button type="primary" icon={<CloudDownloadOutlined />} disabled={p.installed} onClick={() => install(p.id)}>{p.installed ? '已安装' : '安装'}</Button>,
              ]}>
                <List.Item.Meta
                  title={<Space wrap>{p.displayName || p.name} {p.version && <Tag>{p.version}</Tag>} {tags(p.tags)} {tags(p.capabilities)}</Space>}
                  description={<Space direction="vertical" size={2}><span>{p.description || '-'}</span><span style={{ color: '#666' }}>作者: {p.author || '-'}</span></Space>}
                />
              </List.Item>
            )} />
          ) },
        ]} />
      </Card>
      <Modal title="新建插件" open={modalOpen} onOk={create} onCancel={() => setModalOpen(false)} okText="创建" cancelText="取消">
        <Form form={form} layout="vertical">
          <Form.Item name="name" label="目录名" rules={[{ required: true, pattern: /^[a-zA-Z0-9_-]+$/, message: '只允许字母数字下划线短横线' }]}><Input placeholder="my-plugin" /></Form.Item>
          <Form.Item name="displayName" label="显示名"><Input /></Form.Item>
          <Form.Item name="description" label="描述"><Input.TextArea /></Form.Item>
          <Form.Item name="version" label="版本" initialValue="1.0.0"><Input /></Form.Item>
          <Form.Item name="author" label="作者"><Input /></Form.Item>
        </Form>
      </Modal>
    </>
  )
}
