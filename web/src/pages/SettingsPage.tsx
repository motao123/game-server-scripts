import { Button, Card, Descriptions, Form, Input, message, Tag } from 'antd'
import { useEffect, useState } from 'react'
import { PageHeader } from '../components/PageHeader'
import { api } from '../api'

export default function SettingsPage() {
  const [settings, setSettings] = useState<any>({})
  const [form] = Form.useForm()
  async function load() { setSettings(await api('/api/settings')) }
  useEffect(() => { load() }, [])

  async function changePassword() {
    try {
      const v = await form.validateFields()
      await api('/api/settings/password', { method: 'POST', body: v })
      message.success('密码已修改'); form.resetFields()
    } catch (e: any) { if (e.errorFields?.length) return; message.error(e.message) }
  }

  return (
    <>
      <PageHeader title="设置" desc="面板配置与密码管理" actions={<Button onClick={load}>刷新</Button>} />
      <Card className="section-card" title="运行配置">
        <Descriptions column={1} bordered items={[
          { key: 'bind', label: '绑定地址', children: settings.bind },
          { key: 'port', label: '端口', children: settings.port },
          { key: 'dataDir', label: '数据目录', children: settings.dataDir },
          { key: 'palServerDir', label: 'Palworld 目录', children: settings.palServerDir },
          { key: 'backupDir', label: '备份目录', children: settings.backupDir },
          { key: 'rconPort', label: 'RCON 端口', children: settings.rconPort },
          { key: 'restApiPort', label: 'REST API 端口', children: settings.restApiPort },
          { key: 'service', label: 'systemd 服务', children: settings.service },
        ]} />
        {settings.fileRoots && (
          <div style={{ marginTop: 12 }}>
            <span>文件管理允许的根目录: </span>
            {(settings.fileRoots as string[]).map(r => <Tag color="blue" key={r}>{r}</Tag>)}
          </div>
        )}
        {settings.securityNotice && <div style={{ marginTop: 12, color: '#F2C037' }}>{settings.securityNotice}</div>}
      </Card>
      <Card title="修改密码">
        <Form form={form} layout="inline" onFinish={changePassword}>
          <Form.Item name="oldPassword" rules={[{ required: true }]}><Input.Password placeholder="旧密码" /></Form.Item>
          <Form.Item name="newPassword" rules={[{ required: true, min: 6 }]}><Input.Password placeholder="新密码 (至少6位)" /></Form.Item>
          <Button type="primary" htmlType="submit">修改</Button>
        </Form>
      </Card>
    </>
  )
}
