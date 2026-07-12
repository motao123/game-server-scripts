import { Button, Card, Form, Input, InputNumber, List, Select, Space, Tag, message } from 'antd'
import { useEffect, useRef, useState } from 'react'
import { PageHeader } from '../components/PageHeader'
import { api } from '../api'

type HistoryItem = { type: 'cmd' | 'resp' | 'info'; text: string; time: string }

export default function RconPage() {
  const [instances, setInstances] = useState<any[]>([])
  const [instanceId, setInstanceId] = useState('')
  const [config, setConfig] = useState<any>({ host: '127.0.0.1', port: 25575, password: '', timeout: 5 })
  const [connected, setConnected] = useState(false)
  const [history, setHistory] = useState<HistoryItem[]>([])
  const [command, setCommand] = useState('')
  const bottomRef = useRef<HTMLDivElement>(null)

  async function loadInstances() { const d = await api<{ instances: any[] }>('/api/instances'); setInstances(d.instances || []) }
  useEffect(() => { loadInstances() }, [])
  useEffect(() => { bottomRef.current?.scrollIntoView({ behavior: 'smooth' }) }, [history])

  async function loadConfig(id: string) {
    setInstanceId(id)
    setConnected(false)
    setHistory([])
    if (!id) return
    try { const d = await api<any>(`/api/rcon/config?instanceId=${id}`); setConfig(d) }
    catch (e: any) { message.error(e.message) }
  }

  async function saveConfig() {
    try { await api('/api/rcon/config/save', { method: 'POST', body: { instanceId, config } }); message.success('配置已保存') }
    catch (e: any) { message.error(e.message) }
  }

  async function connect() {
    if (!instanceId) { message.warning('请先选择实例'); return }
    setHistory(h => [...h, { type: 'info', text: `连接 ${config.host}:${config.port}...`, time: now() }])
    try {
      await api('/api/rcon/connect', { method: 'POST', body: { instanceId } })
      setConnected(true); setHistory(h => [...h, { type: 'info', text: '已连接', time: now() }])
    } catch (e: any) { setHistory(h => [...h, { type: 'info', text: '连接失败: ' + e.message, time: now() }]) }
  }

  async function disconnect() {
    try { await api('/api/rcon/disconnect', { method: 'POST', body: { instanceId } }); setConnected(false); setHistory(h => [...h, { type: 'info', text: '已断开', time: now() }]) }
    catch (e: any) { message.error(e.message) }
  }

  async function execute() {
    if (!command.trim() || !connected) return
    const cmd = command
    setHistory(h => [...h, { type: 'cmd', text: cmd, time: now() }])
    setCommand('')
    try {
      const d = await api<any>('/api/rcon/command-instance', { method: 'POST', body: { instanceId, command: cmd } })
      setHistory(h => [...h, { type: 'resp', text: d.response || d.error || '(空)', time: now() }])
    } catch (e: any) { setHistory(h => [...h, { type: 'resp', text: '错误: ' + e.message, time: now() }]) }
  }

  function now() { return new Date().toLocaleTimeString('zh-CN') }

  return (
    <>
      <PageHeader title="RCON 控制台" desc="实例级 RCON 连接管理，支持配置保存和命令历史" />
      <Card className="section-card">
        <Space>
          <Select style={{ width: 250 }} placeholder="选择实例" value={instanceId || undefined} onChange={loadConfig} options={instances.map(i => ({ value: i.id, label: i.name }))} />
          <Tag color={connected ? 'green' : 'default'}>{connected ? '已连接' : '未连接'}</Tag>
          {!connected ? <Button type="primary" onClick={connect}>连接</Button> : <Button danger onClick={disconnect}>断开</Button>}
        </Space>
      </Card>
      {instanceId && (
        <Card title="RCON 配置" className="section-card">
          <Form layout="inline">
            <Form.Item label="Host"><Input value={config.host} onChange={e => setConfig({ ...config, host: e.target.value })} /></Form.Item>
            <Form.Item label="Port"><InputNumber value={config.port} onChange={(v: any) => setConfig({ ...config, port: v })} /></Form.Item>
            <Form.Item label="Password"><Input.Password value={config.password} onChange={e => setConfig({ ...config, password: e.target.value })} /></Form.Item>
            <Form.Item label="Timeout(s)"><InputNumber value={config.timeout} onChange={(v: any) => setConfig({ ...config, timeout: v })} /></Form.Item>
            <Button onClick={saveConfig}>保存配置</Button>
          </Form>
        </Card>
      )}
      <Card title="控制台">
        <div style={{ background: '#1e1e2e', color: '#cdd6f4', padding: 12, borderRadius: 8, minHeight: 300, maxHeight: 400, overflow: 'auto', fontFamily: 'monospace', fontSize: 13 }}>
          {history.length === 0 ? <span style={{ color: '#666' }}>等待命令...</span> :
            history.map((h, i) => (
              <div key={i} style={{ marginBottom: 4 }}>
                <span style={{ color: '#666' }}>[{h.time}] </span>
                {h.type === 'cmd' && <span style={{ color: '#89b4fa' }}>&gt; {h.text}</span>}
                {h.type === 'resp' && <span style={{ color: '#a6e3a1' }}>{h.text}</span>}
                {h.type === 'info' && <span style={{ color: '#f9e2af' }}>{h.text}</span>}
              </div>
            ))
          }
          <div ref={bottomRef} />
        </div>
        <Input.Search style={{ marginTop: 8 }} value={command} onChange={e => setCommand(e.target.value)} onSearch={execute} enterButton="发送" placeholder="输入 RCON 命令，如 Info / ShowPlayers / Save" disabled={!connected} />
      </Card>
    </>
  )
}
