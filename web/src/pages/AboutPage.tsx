import { Card, Typography } from 'antd'
import { PageHeader } from '../components/PageHeader'

export default function AboutPage() {
  return <>
    <PageHeader title="关于" desc="Go + React 重写版，功能参考 GameServerManager，样式参考 palworld-go" />
    <Card>
      <Typography.Paragraph>该面板整合游戏实例、部署、终端、文件、计划任务、环境、RCON、备份、插件和设置模块。</Typography.Paragraph>
    </Card>
  </>
}
