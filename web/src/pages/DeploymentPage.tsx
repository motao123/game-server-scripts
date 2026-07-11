import { Card, List, Tag } from 'antd'
import { PageHeader } from '../components/PageHeader'

const games = ['Palworld', 'Minecraft Java', 'Minecraft Bedrock', 'Valheim', 'Terraria', 'SteamCMD 通用游戏']

export default function DeploymentPage() {
  return <>
    <PageHeader title="游戏部署" desc="参考 GameServerManager 的 SteamCMD/在线部署流程，当前提供本仓库支持游戏的部署入口" />
    <Card>
      <List dataSource={games} renderItem={game => <List.Item actions={[<Tag color="blue">模板</Tag>]}>{game}</List.Item>} />
    </Card>
  </>
}
