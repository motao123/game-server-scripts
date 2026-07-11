import { Card, Typography } from 'antd'
import { PageHeader } from '../components/PageHeader'

export default function TerminalPage() {
  return <>
    <PageHeader title="终端" desc="WebSocket 终端模块已预留，后端当前提供会话入口和鉴权通道" />
    <Card><Typography.Paragraph>第一版已建立 `/ws` 鉴权连接和终端会话 API，后续接入 PTY 后可在此显示 xterm.js 终端。</Typography.Paragraph></Card>
  </>
}
