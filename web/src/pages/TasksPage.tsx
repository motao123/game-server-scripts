import { Button, Card, Form, Input, Table, message, Modal } from 'antd'
import { useEffect, useState } from 'react'
import { PageHeader } from '../components/PageHeader'
import { api } from '../api'

export default function TasksPage() {
  const [tasks, setTasks] = useState<any[]>([])
  async function load() { const d = await api<{ tasks: any[] }>('/api/scheduled-tasks'); setTasks(d.tasks || []) }
  async function create(values: any) { await api('/api/scheduled-tasks/create', { method: 'POST', body: values }); message.success('已创建'); load() }
  async function remove(id: string) {
    Modal.confirm({
      title: '确认删除任务', content: '', okText: '删除', okType: 'danger', cancelText: '取消',
      onOk: async () => { await api('/api/scheduled-tasks/delete', { method: 'POST', body: { id } }); message.success('已删除'); load() },
    })
  }
  useEffect(() => { load() }, [])
  return <>
    <PageHeader title="计划任务" desc="支持 cron 调度备份、实例动作和 shell 命令" actions={<Button onClick={load}>刷新</Button>} />
    <Card className="section-card">
      <Form layout="inline" onFinish={create}>
        <Form.Item name="name" rules={[{ required: true }]}><Input placeholder="任务名称" /></Form.Item>
        <Form.Item name="cron" rules={[{ required: true }]}><Input placeholder="Cron，如 */30 * * * *" /></Form.Item>
        <Form.Item name="action" rules={[{ required: true }]}><Input placeholder="backup / shell date / instance start <id>" /></Form.Item>
        <Button type="primary" htmlType="submit">创建</Button>
      </Form>
    </Card>
    <Card><Table rowKey="id" dataSource={tasks} pagination={false} columns={[{ title: '名称', dataIndex: 'name' }, { title: '计划', dataIndex: 'cron' }, { title: '动作', dataIndex: 'action' }, { title: '启用', dataIndex: 'enabled', render: Boolean }, { title: '操作', render: (_, r: any) => <Button danger onClick={() => remove(r.id)}>删除</Button> }]} /></Card>
  </>
}
