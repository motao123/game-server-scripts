import { Card, Input, Button, Typography, message } from 'antd'
import { useState } from 'react'
import { PageHeader } from '../components/PageHeader'
import { api } from '../api'

export default function RconPage() {
  const [command, setCommand] = useState('Info')
  const [output, setOutput] = useState('')
  async function run() { try { const d = await api<{ output: string; error: string }>('/api/rcon/command', { method: 'POST', body: { command } }); setOutput(d.output || d.error || '') } catch (e: any) { message.error(e.message) } }
  return <>
    <PageHeader title="RCON 控制台" desc="实例级 RCON 命令执行，Palworld 默认连接本机 RCON" />
    <Card>
      <Input.Search value={command} onChange={e => setCommand(e.target.value)} enterButton="执行" onSearch={run} />
      <Typography.Paragraph className="console-box">{output}</Typography.Paragraph>
    </Card>
  </>
}
