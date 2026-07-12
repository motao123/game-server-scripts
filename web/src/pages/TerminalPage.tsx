import { Button, Card, Input, Space, Tag, message } from 'antd'
import { useEffect, useRef, useState } from 'react'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import { WebLinksAddon } from '@xterm/addon-web-links'
import '@xterm/xterm/css/xterm.css'
import { PageHeader } from '../components/PageHeader'

type Session = { id: string; terminal: Terminal; fitAddon: FitAddon; name: string }

export default function TerminalPage() {
  const [sessions, setSessions] = useState<Session[]>([])
  const [activeId, setActiveId] = useState<string>('')
  const [connected, setConnected] = useState(false)
  const [input, setInput] = useState('')
  const ws = useRef<WebSocket | null>(null)
  const containerRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    const proto = location.protocol === 'https:' ? 'wss' : 'ws'
    const socket = new WebSocket(`${proto}://${location.host}/ws`)
    ws.current = socket
    socket.onopen = () => setConnected(true)
    socket.onclose = () => setConnected(false)
    socket.onmessage = (event) => {
      const msg = JSON.parse(event.data)
      handleMsg(msg)
    }
    return () => { socket.close(); sessions.forEach(s => s.terminal.dispose()) }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  function handleMsg(msg: any) {
    if (msg.type === 'terminal-started') {
      setSessions(prev => {
        const last = prev[prev.length - 1]
        if (last) {
          last.id = msg.sessionId
          last.terminal.writeln('\r\n\x1b[32m[会话已启动]\x1b[0m')
        }
        return [...prev]
      })
    } else if (msg.type === 'terminal-output') {
      const s = sessions.find(s => s.id === msg.sessionId)
      if (s) {
        if (msg.isHistorical) s.terminal.clear()
        s.terminal.write(msg.data)
      }
    } else if (msg.type === 'terminal-exit' || msg.type === 'terminal-closed') {
      const s = sessions.find(s => s.id === msg.sessionId)
      if (s) s.terminal.writeln('\r\n\x1b[31m[会话已结束]\x1b[0m')
    } else if (msg.type === 'session-reconnected') {
      message.success('会话已重连')
    }
  }

  function createSession() {
    const terminal = new Terminal({
      theme: { background: '#1e1e2e', foreground: '#cdd6f4', cursor: '#f5e0dc' },
      fontSize: 14,
      cursorBlink: true,
      scrollback: 1000,
      convertEol: true,
    })
    const fitAddon = new FitAddon()
    terminal.loadAddon(fitAddon)
    terminal.loadAddon(new WebLinksAddon())
    const name = `终端 ${sessions.length + 1}`
    const session: Session = { id: '', terminal, fitAddon, name }
    terminal.onData((data) => {
      if (session.id && ws.current?.readyState === WebSocket.OPEN) {
        ws.current.send(JSON.stringify({ type: 'terminal-input', sessionId: session.id, data }))
      }
    })
    setSessions(prev => [...prev, session])
    setActiveId(sessions.length.toString())
    setTimeout(() => {
      if (containerRef.current) {
        terminal.open(containerRef.current)
        fitAddon.fit()
        terminal.focus()
      }
      if (ws.current?.readyState === WebSocket.OPEN) {
        ws.current.send(JSON.stringify({ type: 'terminal-start', cols: terminal.cols, rows: terminal.rows }))
      }
    }, 100)
  }

  function closeSession(index: number) {
    const s = sessions[index]
    if (s.id && ws.current?.readyState === WebSocket.OPEN) {
      ws.current.send(JSON.stringify({ type: 'terminal-close', sessionId: s.id }))
    }
    s.terminal.dispose()
    setSessions(prev => prev.filter((_, i) => i !== index))
    if (activeId === index.toString() && sessions.length > 1) {
      setActiveId('0')
    }
  }

  function fitActive() {
    const s = sessions[parseInt(activeId)]
    if (s) s.fitAddon.fit()
  }

  useEffect(() => {
    const handler = () => fitActive()
    window.addEventListener('resize', handler)
    return () => window.removeEventListener('resize', handler)
  }, [activeId, sessions])

  return (
    <>
      <PageHeader title="终端" desc="基于 xterm.js 的真实 PTY 终端，支持多标签" actions={
        <Space>
          <Tag color={connected ? 'green' : 'default'}>{connected ? '已连接' : '未连接'}</Tag>
          <Button type="primary" onClick={createSession} disabled={!connected}>新建终端</Button>
          <Button onClick={fitActive}>自适应</Button>
        </Space>
      } />
      <div style={{ display: 'flex', gap: 8 }}>
        <Card style={{ width: 180 }} title="会话列表" bodyStyle={{ padding: 0 }}>
          {sessions.length === 0 ? <div style={{ padding: 16, color: '#999' }}>无会话</div> :
            sessions.map((s, i) => (
              <div key={i} style={{
                padding: '8px 12px', cursor: 'pointer', display: 'flex', justifyContent: 'space-between',
                background: activeId === i.toString() ? '#e6f4ff' : 'transparent',
              }} onClick={() => setActiveId(i.toString())}>
                <span>{s.name}</span>
                <Button size="small" type="text" danger onClick={(e) => { e.stopPropagation(); closeSession(i) }}>×</Button>
              </div>
            ))
          }
        </Card>
        <Card style={{ flex: 1 }} title={sessions[parseInt(activeId)]?.name || '终端'}>
          <div ref={containerRef} style={{ minHeight: 400, background: '#1e1e2e' }} />
        </Card>
      </div>
    </>
  )
}
