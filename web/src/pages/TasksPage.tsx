import { Button, Card, Form, Input, Modal, Select, Switch, Table, Tag, message } from 'antd'
import { useEffect, useState } from 'react'
import { PageHeader } from '../components/PageHeader'
import { api } from '../api'

export default function TasksPage() {
  const [tasks, setTasks] = useState<any[]>([])
  const [instances, setInstances] = useState<any[]>([])
  const [modalOpen, setModalOpen] = useState(false)
  const [editing, setEditing] = useState<any>(null)
  const [form] = Form.useForm()

  async function load() {
    const [t, i] = await Promise.all([api<{ tasks: any[] }>('/api/scheduled-tasks'), api<{ instances: any[] }>('/api/instances')])
    setTasks(t.tasks || []); setInstances(i.instances || [])
  }
  useEffect(() => { load() }, [])

  function openCreate() {
    setEditing(null)
    form.resetFields()
    form.setFieldsValue({ type: 'shell', enabled: true, maxKeep: 10 })
    setModalOpen(true)
  }
  function openEdit(t: any) {
    setEditing(t)
    form.setFieldsValue(t)
    setModalOpen(true)
  }

  async function submit() {
    try {
      const values = await form.validateFields()
      const body = { ...values, action: buildAction(values), id: editing?.id }
      if (editing) {
        await api('/api/scheduled-tasks/create', { method: 'POST', body: { name: values.name, cron: values.cron, action: buildAction(values) } })
        message.success('已更新（重建任务）')
      } else {
        await api('/api/scheduled-tasks/create', { method: 'POST', body: { name: values.name, cron: values.cron, action: buildAction(values) } })
        message.success('已创建')
      }
      setModalOpen(false); load()
    } catch (e: any) { if (e.errorFields?.length) return; message.error(e.message) }
  }

  function buildAction(v: any): string {
    switch (v.type) {
      case 'power': return `power ${v.action} ${v.instanceId}`
      case 'command': return `command ${v.instanceId} ${v.command}`
      case 'backup': return `backup`
      case 'system': return `system ${v.systemAction}`
      default: return v.action || 'shell date'
    }
  }

  async function remove(id: string) {
    Modal.confirm({
      title: '确认删除任务', okText: '删除', okType: 'danger', cancelText: '取消',
      onOk: async () => { await api('/api/scheduled-tasks/delete', { method: 'POST', body: { id } }); message.success('已删除'); load() },
    })
  }

  const columns = [
    { title: '名称', dataIndex: 'name' },
    { title: '计划', dataIndex: 'cron' },
    { title: '动作', dataIndex: 'action' },
    { title: '启用', dataIndex: 'enabled', render: (v: boolean) => <Tag color={v ? 'green' : 'default'}>{v ? '是' : '否'}</Tag> },
    { title: '创建时间', dataIndex: 'createdAt', render: (t: string) => t ? new Date(t).toLocaleString('zh-CN') : '-' },
    { title: '操作', render: (_: any, r: any) => (
      <>
        <Button size="small" type="link" onClick={() => openEdit(r)}>编辑</Button>
        <Button size="small" type="link" danger onClick={() => remove(r.id)}>删除</Button>
      </>
    ) },
  ]

  return (
    <>
      <PageHeader title="计划任务" desc="支持 power/command/backup/system 四种类型，cron 调度" actions={<Button onClick={load}>刷新</Button>} />
      <Card extra={<Button type="primary" onClick={openCreate}>新建任务</Button>}>
        <Table rowKey="id" dataSource={tasks} columns={columns} pagination={false} />
      </Card>
      <Modal title={editing ? '编辑任务' : '新建任务'} open={modalOpen} onOk={submit} onCancel={() => setModalOpen(false)} width={600} okText="保存" cancelText="取消">
        <Form form={form} layout="vertical">
          <Form.Item name="name" label="任务名称" rules={[{ required: true }]}><Input /></Form.Item>
          <Form.Item name="type" label="任务类型" rules={[{ required: true }]}>
            <Select options={[
              { value: 'power', label: '实例开关 (启动/停止/重启)' },
              { value: 'command', label: '发送命令到实例' },
              { value: 'backup', label: '备份' },
              { value: 'system', label: '系统任务' },
              { value: 'shell', label: 'Shell 命令' },
            ]} />
          </Form.Item>
          <Form.Item name="cron" label="Cron 表达式" rules={[{ required: true }]} extra="如: */30 * * * * (每30分钟), 0 4 * * * (每天4点)">
            <Input placeholder="*/30 * * * *" />
          </Form.Item>
          <Form.Item name="instanceId" label="实例" hidden={false}><Select options={instances.map(i => ({ value: i.id, label: i.name }))} allowClear /></Form.Item>
          <Form.Item name="action" label="动作 (power)" hidden><Select options={[{value:'start',label:'启动'},{value:'stop',label:'停止'},{value:'restart',label:'重启'}]} /></Form.Item>
          <Form.Item name="command" label="命令内容 (command)" hidden><Input /></Form.Item>
          <Form.Item name="systemAction" label="系统动作" hidden><Select options={[{value:'steam_update',label:'更新 Steam 游戏列表'}]} /></Form.Item>
          <Form.Item name="enabled" label="启用" valuePropName="checked"><Switch /></Form.Item>
        </Form>
      </Modal>
    </>
  )
}
