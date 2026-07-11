import { Button, Card, Table } from 'antd'
import { useEffect, useState } from 'react'
import { PageHeader } from '../components/PageHeader'
import { api } from '../api'

export default function TasksPage() {
  const [tasks, setTasks] = useState<any[]>([])
  async function load() { const d = await api<{ tasks: any[] }>('/api/scheduled-tasks'); setTasks(d.tasks || []) }
  useEffect(() => { load() }, [])
  return <>
    <PageHeader title="计划任务" desc="统一展示定时重启、备份和实例命令任务" actions={<Button onClick={load}>刷新</Button>} />
    <Card><Table rowKey="id" dataSource={tasks} pagination={false} columns={[{ title: '名称', dataIndex: 'name' }, { title: '计划', dataIndex: 'cron' }, { title: '动作', dataIndex: 'action' }, { title: '启用', dataIndex: 'enabled', render: Boolean }]} /></Card>
  </>
}
