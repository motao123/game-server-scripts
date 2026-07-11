import { Button, Card, Input, Space, Typography, message } from 'antd'
import { useEffect, useRef, useState } from 'react'
import { PageHeader } from '../components/PageHeader'

export default function TerminalPage() {
  const [connected, setConnected] = useState(false)
  const [sessionId, setSessionId] = useState('')
  const [input, setInput] = useState('')
  const [output, setOutput] = useState('')
  const ws = useRef<WebSocket | null>(null)

  useEffect(() => () => { ws.current?.close() }, [])

  function connect() {
    const proto = location.protocol === 'https:' ? 'wss' : 'ws'
    const socket = new WebSocket(`${proto}://${location.host}/ws`)
    ws.current = socket
    socket.onopen = () => setConnected(true)
    socket.onclose = () => setConnected(false)
    socket.onmessage = (event) => {
      const msg = JSON.parse(event.data)
      if (msg.type === 'terminal-started') setSessionId(msg.sessionId)
      if (msg.type === 'terminal-output') setOutput(prev => prev + msg.data)
      if (msg.type === 'terminal-error') message.error(msg.error)
    }
  }
  function start() {
    if (!ws.current || ws.current.readyState !== WebSocket.OPEN) return message.warning('请先连接')
    ws.current.send(JSON.stringify({ type: 'terminal-start', cols: 120, rows: 32 }))
  }
  function send() {
    if (!sessionId) return
    ws.current?.send(JSON.stringify({ type: 'terminal-input', sessionId, data: input + '\n' }))
    setInput('')
  }
  function close() {
    if (sessionId) ws.current?.send(JSON.stringify({ type: 'terminal-close', sessionId }))
    setSessionId('')
  }

  return <>
    <PageHeader title="终端" desc="真实 PTY 终端，支持输入输出和关闭会话" actions={<Space><Button onClick={connect} disabled={connected}>连接</Button><Button type="primary" onClick={start} disabled={!connected || !!sessionId}>启动 Shell</Button><Button onClick={close} disabled={!sessionId}>关闭</Button></Space>} />
    <Card>
      <Typography.Text type={connected ? 'success' : 'secondary'}>{connected ? `已连接 ${sessionId || ''}` : '未连接'}</Typography.Text>
      <pre className="console-box">{output}</pre>
      <Input.Search value={input} onChange={e => setInput(e.target.value)} onSearch={send} enterButton="发送" disabled={!sessionId} />
    </Card>
  </>
}
