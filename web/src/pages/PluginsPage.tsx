import { Card, Empty } from 'antd'
import { PageHeader } from '../components/PageHeader'

export default function PluginsPage() {
  return <>
    <PageHeader title="插件" desc="插件目录扫描、启用状态和 API 桥接预留" />
    <Card><Empty description="插件运行时已预留，默认不执行不可信插件代码" /></Card>
  </>
}
