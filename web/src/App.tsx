import { useEffect } from 'react'
import { HashRouter, Routes, Route, Navigate } from 'react-router-dom'
import { Layout, Menu, Spin } from 'antd'
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
import BackupPage from './pages/BackupPage'

const { Header, Sider, Content } = Layout

const menuItems = [
  { key: '/dashboard', label: '仪表盘', icon: <HomeOutlined /> },
  { key: '/terminal', label: '终端', icon: <CodeOutlined /> },
  { key: '/instances', label: '实例管理', icon: <CloudServerOutlined /> },
  { key: '/deployment', label: '游戏部署', icon: <DeploymentUnitOutlined /> },
  { key: '/files', label: '文件管理', icon: <FileTextOutlined /> },
  { key: '/backup', label: '备份管理', icon: <DatabaseOutlined /> },
  { key: '/tasks', label: '计划任务', icon: <PlayCircleOutlined /> },
  { key: '/environment', label: '环境管理', icon: <ToolOutlined /> },
  { key: '/rcon', label: 'RCON', icon: <ControlOutlined /> },
  { key: '/plugins', label: '插件', icon: <DatabaseOutlined /> },
  { key: '/settings', label: '设置', icon: <SettingOutlined /> },
  { key: '/about', label: '关于', icon: <SafetyCertificateOutlined /> },
]

export default function App() {
  const { authenticated, loading, verify, logout } = useAuth()

  useEffect(() => { verify() }, [verify])

  if (loading) return <div className="boot"><Spin size="large" /></div>
  if (!authenticated) return <LoginPage />

  return (
    <HashRouter>
      <Layout className="app-shell">
        <Header className="topbar">
          <div className="brand"><span className="brand-mark">GSM</span></div>
          <Menu mode="horizontal" items={menuItems} selectedKeys={[]} onClick={({ key }) => window.location.hash = key} style={{ flex: 1, border: 'none', background: 'transparent' }} />
          <LogoutOutlined onClick={logout} style={{ fontSize: 18, cursor: 'pointer' }} />
        </Header>
        <Layout>
          <Sider width={200} className="sidebar">
            <Menu mode="inline" items={menuItems} onClick={({ key }) => window.location.hash = key} />
          </Sider>
          <Content className="content">
            <Routes>
              <Route path="/" element={<Navigate to="/dashboard" replace />} />
              <Route path="/dashboard" element={<DashboardPage />} />
              <Route path="/terminal" element={<TerminalPage />} />
              <Route path="/instances" element={<InstancesPage />} />
              <Route path="/deployment" element={<DeploymentPage />} />
              <Route path="/files" element={<FilesPage />} />
              <Route path="/backup" element={<BackupPage />} />
              <Route path="/tasks" element={<TasksPage />} />
              <Route path="/environment" element={<EnvironmentPage />} />
              <Route path="/rcon" element={<RconPage />} />
              <Route path="/plugins" element={<PluginsPage />} />
              <Route path="/settings" element={<SettingsPage />} />
              <Route path="/about" element={<AboutPage />} />
              <Route path="*" element={<Navigate to="/dashboard" replace />} />
            </Routes>
          </Content>
        </Layout>
      </Layout>
    </HashRouter>
  )
}
