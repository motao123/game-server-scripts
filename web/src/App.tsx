import { useEffect, useState } from 'react'
import { Button, Layout, Menu, Spin, Typography } from 'antd'
import { CloudServerOutlined, CodeOutlined, ControlOutlined, DatabaseOutlined, DeploymentUnitOutlined, FileTextOutlined, HomeOutlined, LogoutOutlined, PlayCircleOutlined, SafetyCertificateOutlined, SettingOutlined, ToolOutlined } from '@ant-design/icons'
import { useAuth } from './store'
import LoginPage from './pages/LoginPage'
import DashboardPage from './pages/DashboardPage'
import TerminalPage from './pages/TerminalPage'
import InstancesPage from './pages/InstancesPage'
import DeploymentPage from './pages/DeploymentPage'
import FilesPage from './pages/FilesPage'
import TasksPage from './pages/TasksPage'
import EnvironmentPage from './pages/EnvironmentPage'
import RconPage from './pages/RconPage'
import PluginsPage from './pages/PluginsPage'
import SettingsPage from './pages/SettingsPage'
import AboutPage from './pages/AboutPage'

const { Header, Sider, Content } = Layout

const pages = [
  { key: 'dashboard', label: '仪表盘', icon: <HomeOutlined /> },
  { key: 'terminal', label: '终端', icon: <CodeOutlined /> },
  { key: 'instances', label: '实例管理', icon: <CloudServerOutlined /> },
  { key: 'deployment', label: '游戏部署', icon: <DeploymentUnitOutlined /> },
  { key: 'files', label: '文件管理', icon: <FileTextOutlined /> },
  { key: 'tasks', label: '计划任务', icon: <PlayCircleOutlined /> },
  { key: 'environment', label: '环境管理', icon: <ToolOutlined /> },
  { key: 'rcon', label: 'RCON', icon: <ControlOutlined /> },
  { key: 'plugins', label: '插件', icon: <DatabaseOutlined /> },
  { key: 'settings', label: '设置', icon: <SettingOutlined /> },
  { key: 'about', label: '关于', icon: <SafetyCertificateOutlined /> }
]

export default function App() {
  const { authenticated, loading, verify, logout } = useAuth()
  const [page, setPage] = useState('dashboard')

  useEffect(() => { verify() }, [verify])

  if (loading) return <div className="boot"><Spin size="large" /></div>
  if (!authenticated) return <LoginPage />

  return (
    <Layout className="app-shell">
      <Header className="topbar">
        <div className="brand"><span className="brand-mark">GSM</span><Typography.Title level={4}>Game Server Manager</Typography.Title></div>
        <Button icon={<LogoutOutlined />} onClick={logout}>退出</Button>
      </Header>
      <Layout>
        <Sider width={220} className="sidebar">
          <Menu mode="inline" selectedKeys={[page]} items={pages} onClick={({ key }) => setPage(key)} />
        </Sider>
        <Content className="content">
          {page === 'dashboard' && <DashboardPage />}
          {page === 'terminal' && <TerminalPage />}
          {page === 'instances' && <InstancesPage />}
          {page === 'deployment' && <DeploymentPage />}
          {page === 'files' && <FilesPage />}
          {page === 'tasks' && <TasksPage />}
          {page === 'environment' && <EnvironmentPage />}
          {page === 'rcon' && <RconPage />}
          {page === 'plugins' && <PluginsPage />}
          {page === 'settings' && <SettingsPage />}
          {page === 'about' && <AboutPage />}
        </Content>
      </Layout>
    </Layout>
  )
}
