import { Button, Card, Form, Input, Typography, message } from 'antd'
import { LockOutlined } from '@ant-design/icons'
import { useAuth } from '../store'

export default function LoginPage() {
  const { login } = useAuth()
  async function submit(values: { password: string }) {
    try {
      await login(values.password)
    } catch (error: any) {
      message.error(error.message)
    }
  }
  return (
    <div className="login-page">
      <Card className="login-card">
        <Typography.Title level={3}>GSM Panel</Typography.Title>
        <Typography.Paragraph type="secondary">游戏服务器管理面板</Typography.Paragraph>
        <Form layout="vertical" onFinish={submit}>
          <Form.Item name="password" rules={[{ required: true, message: '请输入 Web 密码' }]}>
            <Input.Password prefix={<LockOutlined />} placeholder="Web 密码" size="large" autoFocus />
          </Form.Item>
          <Button type="primary" htmlType="submit" block size="large">登录</Button>
        </Form>
      </Card>
    </div>
  )
}
