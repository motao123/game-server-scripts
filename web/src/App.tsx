import { useEffect, useState } from 'react'
import { HashRouter, Routes, Route, Navigate } from 'react-router-dom'
import { Button, ConfigProvider, Layout, Menu, Spin, theme as antdTheme } from 'antd'
import { CloudServerOutlined, CodeOutlined, ControlOutlined, DatabaseOutlined, DeploymentUnitOutlined, FileTextOutlined, HomeOutlined, LogoutOutlined, MenuFoldOutlined, MenuUnfoldOutlined, MoonOutlined, PlayCircleOutlined, SafetyCertificateOutlined, SettingOutlined, SunOutlined, ToolOutlined } from '@ant-design/icons'
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
  const [dark, setDark] = useState(() => localStorage.getItem('theme') === 'dark')
  const [collapsed, setCollapsed] = useState(false)

  useEffect(() => { verify() }, [verify])
  useEffect(() => { localStorage.setItem('theme', dark ? 'dark' : 'light') }, [dark])

  if (loading) return <div className="boot"><Spin size="large" /></div>
  if (!authenticated) return <LoginPage dark={dark} />

  return (
    <ConfigProvider theme={{ algorithm: dark ? antdTheme.darkAlgorithm : antdTheme.defaultAlgorithm, token: { colorPrimary: '#1976D2', borderRadius: 8 } }}>
      <HashRouter>
        <Layout className="app-shell">
          <Header className="topbar">
            <Button type="text" icon={collapsed ? <MenuUnfoldOutlined /> : <MenuFoldOutlined />} onClick={() => setCollapsed(!collapsed)} />
            <div className="brand"><span className="brand-mark">GSM</span></div>
            <div style={{ flex: 1 }} />
            <Button type="text" icon={dark ? <SunOutlined /> : <MoonOutlined />} onClick={() => setDark(!dark)} />
            <Button type="text" icon={<LogoutOutlined />} onClick={logout} />
          </Header>
          <Layout>
            <Sider width={collapsed ? 0 : 200} className="sidebar" collapsible collapsed={collapsed} trigger={null}>
              <Menu mode="inline" items={menuItems} theme={dark ? 'dark' : 'light'} onClick={({ key }) => window.location.hash = key} />
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
    </ConfigProvider>
  )
}
