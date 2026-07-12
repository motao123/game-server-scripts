import { Button, Card, Form, Input, Modal, Select, Space, Switch, Table, Tag, message } from 'antd'
import { useEffect, useState } from 'react'
import { PageHeader } from '../components/PageHeader'
import { api } from '../api'

type TaskForm = {
  id?: string
  name: string
  cron: string
  type: string
  enabled: boolean
  action?: string
  instanceId?: string
  command?: string
  systemAction?: string
}

export default function TasksPage() {
  const [tasks, setTasks] = useState<any[]>([])
  const [instances, setInstances] = useState<any[]>([])
  const [modalOpen, setModalOpen] = useState(false)
  const [editing, setEditing] = useState<any>(null)
  const [form] = Form.useForm()
  const taskType = Form.useWatch('type', form)

  async function load() {
    const [t, i] = await Promise.all([api<{ tasks: any[] }>('/api/scheduled-tasks'), api<{ instances: any[] }>('/api/instances')])
    setTasks(t.tasks || [])
    setInstances(i.instances || [])
  }
  useEffect(() => { load() }, [])

  function openCreate() {
    setEditing(null)
    form.resetFields()
    form.setFieldsValue({ type: 'shell', enabled: true, cron: '*/30 * * * *', action: 'shell date' })
    setModalOpen(true)
  }

  function openEdit(task: any) {
    setEditing(task)
    form.resetFields()
    form.setFieldsValue({ ...parseAction(task.action), ...task })
    setModalOpen(true)
  }

  async function submit() {
    try {
      const values = await form.validateFields()
      const body = {
        id: editing?.id,
        name: values.name,
        cron: values.cron,
        enabled: values.enabled !== false,
        action: buildAction(values),
      }
      await api('/api/scheduled-tasks/create', { method: 'POST', body })
      message.success(editing ? '已更新' : '已创建')
      setModalOpen(false)
      load()
    } catch (e: any) {
      if (e.errorFields?.length) return
      message.error(e.message)
    }
  }

  function buildAction(v: TaskForm): string {
    switch (v.type) {
      case 'power': return `power ${v.action} ${v.instanceId}`
      case 'command': return `command ${v.instanceId} ${v.command}`
      case 'backup': return 'backup'
      case 'system': return `system ${v.systemAction}`
      default: return v.action || 'shell date'
    }
  }

  function parseAction(action: string = ''): Partial<TaskForm> {
    const parts = action.split(/\s+/)
    if (parts[0] === 'power') return { type: 'power', action: parts[1], instanceId: parts[2] }
    if (parts[0] === 'command') return { type: 'command', instanceId: parts[1], command: parts.slice(2).join(' ') }
    if (parts[0] === 'backup') return { type: 'backup' }
    if (parts[0] === 'system') return { type: 'system', systemAction: parts[1] }
    return { type: 'shell', action }
  }

  async function toggle(task: any, enabled: boolean) {
    try {
      await api('/api/scheduled-tasks/toggle', { method: 'POST', body: { id: task.id, enabled } })
      message.success(enabled ? '已启用' : '已停用')
      load()
    } catch (e: any) { message.error(e.message) }
  }

  async function run(task: any) {
    try {
      await api('/api/scheduled-tasks/run', { method: 'POST', body: { id: task.id } })
      message.success('任务已触发')
      setTimeout(load, 800)
    } catch (e: any) { message.error(e.message) }
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
    { title: '动作', dataIndex: 'action', ellipsis: true },
    { title: '启用', dataIndex: 'enabled', width: 90, render: (v: boolean, r: any) => <Switch checked={v} onChange={(checked) => toggle(r, checked)} /> },
    { title: '下次运行', dataIndex: 'nextRun', render: formatTime },
    { title: '上次运行', dataIndex: 'lastRun', render: formatTime },
    { title: '错误', dataIndex: 'lastError', render: (v: string) => v ? <Tag color="red">{v}</Tag> : <Tag color="green">无</Tag> },
    { title: '操作', width: 170, render: (_: any, r: any) => (
      <Space size="small">
        <Button size="small" type="link" onClick={() => run(r)}>立即执行</Button>
        <Button size="small" type="link" onClick={() => openEdit(r)}>编辑</Button>
        <Button size="small" type="link" danger onClick={() => remove(r.id)}>删除</Button>
      </Space>
    ) },
  ]

  return (
    <>
      <PageHeader title="计划任务" desc="支持编辑、启停、立即执行和运行时间记录" actions={<Space><Button onClick={load}>刷新</Button><Button type="primary" onClick={openCreate}>新建任务</Button></Space>} />
      <Card>
        <Table rowKey="id" dataSource={tasks} columns={columns} pagination={false} />
      </Card>
      <Modal title={editing ? '编辑任务' : '新建任务'} open={modalOpen} onOk={submit} onCancel={() => setModalOpen(false)} width={640} okText="保存" cancelText="取消">
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
          {(taskType === 'power' || taskType === 'command') && <Form.Item name="instanceId" label="实例" rules={[{ required: true }]}><Select options={instances.map(i => ({ value: i.id, label: i.name }))} /></Form.Item>}
          {taskType === 'power' && <Form.Item name="action" label="动作" rules={[{ required: true }]}><Select options={[{value:'start',label:'启动'},{value:'stop',label:'停止'},{value:'restart',label:'重启'}]} /></Form.Item>}
          {taskType === 'command' && <Form.Item name="command" label="命令内容" rules={[{ required: true }]}><Input /></Form.Item>}
          {taskType === 'system' && <Form.Item name="systemAction" label="系统动作" rules={[{ required: true }]}><Select options={[{value:'steam_update',label:'更新 Steam 游戏列表'}]} /></Form.Item>}
          {taskType === 'shell' && <Form.Item name="action" label="Shell 命令" rules={[{ required: true }]}><Input placeholder="shell date" /></Form.Item>}
          <Form.Item name="enabled" label="启用" valuePropName="checked"><Switch /></Form.Item>
        </Form>
      </Modal>
    </>
  )
}

function formatTime(value: string) {
  return value ? new Date(value).toLocaleString('zh-CN') : '-'
}
