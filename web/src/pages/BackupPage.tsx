import { Button, Card, Col, Form, Input, InputNumber, List, Modal, Row, Space, Tag, message } from 'antd'
import { useEffect, useState } from 'react'
import { PageHeader } from '../components/PageHeader'
import { api } from '../api'

type BackupGroup = { name: string; files: any[]; totalSize: number; sourcePath: string }

export default function BackupPage() {
  const [groups, setGroups] = useState<BackupGroup[]>([])
  const [modalOpen, setModalOpen] = useState(false)
  const [form] = Form.useForm()

  async function load() { const d = await api<{ groups: BackupGroup[] }>('/api/backup/groups'); setGroups(d.groups || []) }
  useEffect(() => { load() }, [])

  async function create() {
    try {
      const v = await form.validateFields()
      const check = await api<any>('/api/backup/create-preflight', { method: 'POST', body: v })
      if (!check.ready) { message.error((check.problems || ['备份预检未通过']).join('；')); return }
      await api('/api/backup/create-generic', { method: 'POST', body: v })
      message.success('备份已创建'); setModalOpen(false); form.resetFields(); load()
    } catch (e: any) { if (e.errorFields?.length) return; message.error(e.message) }
  }

  async function restore(backupName: string, fileName: string) {
    try {
      const check = await api<any>('/api/backup/restore-preflight', { method: 'POST', body: { backupName, fileName } })
      if (!check.ready) { message.error((check.problems || ['归档预检未通过']).join('；')); return }
      const warning = (check.warnings || []).join('；')
      Modal.confirm({
        title: '确认恢复备份',
        content: <div><div>{backupName}/{fileName}</div><div>恢复目标：{check.sourcePath}</div><div>归档条目：{check.archiveEntries}</div>{warning && <div style={{ color: '#C10015', marginTop: 8 }}>{warning}</div>}</div>,
        okText: warning ? '确认覆盖恢复' : '恢复', okType: 'danger', cancelText: '取消',
        onOk: async () => { try { await api('/api/backup/restore-generic', { method: 'POST', body: { backupName, fileName, overwrite: !!warning } }); message.success('已恢复'); load() } catch (e: any) { message.error(e.message) } },
      })
    } catch (e: any) { message.error(e.message) }
  }

  async function del(backupName: string, fileName: string) {
    Modal.confirm({
      title: '确认删除', content: fileName, okText: '删除', okType: 'danger', cancelText: '取消',
      onOk: async () => { await api('/api/backup/delete-file', { method: 'POST', body: { backupName, fileName } }); message.success('已删除'); load() },
    })
  }

  return (
    <>
      <PageHeader title="通用备份" desc="任意目录的 tar.gz 备份，支持分组和保留策略" actions={<Button onClick={load}>刷新</Button>} />
      <Card extra={<Button type="primary" onClick={() => setModalOpen(true)}>创建备份</Button>}>
        {groups.length === 0 ? <div style={{ textAlign: 'center', color: '#999', padding: 24 }}>暂无备份组</div> :
          <Row gutter={[16, 16]}>
            {groups.map(g => (
              <Col xs={24} md={12} lg={8} key={g.name}>
                <Card title={g.name} extra={<Tag>{(g.totalSize / 1048576).toFixed(2)} MB</Tag>}>
                  <div style={{ fontSize: 12, color: '#999', marginBottom: 8 }}>源: {g.sourcePath}</div>
                  <List size="small" dataSource={g.files} renderItem={(f: any) => (
                    <List.Item actions={[
                      <Button size="small" type="link" href={`/api/backup/download?backupName=${g.name}&fileName=${f.name}`}>下载</Button>,
                      <Button size="small" type="link" onClick={() => restore(g.name, f.name)}>恢复</Button>,
                      <Button size="small" type="link" danger onClick={() => del(g.name, f.name)}>删除</Button>,
                    ]}>
                      <div>{f.name}<br /><span style={{ fontSize: 11, color: '#999' }}>{new Date(f.time * 1000).toLocaleString('zh-CN')}</span></div>
                    </List.Item>
                  )} />
                </Card>
              </Col>
            ))}
          </Row>
        }
      </Card>
      <Modal title="创建备份" open={modalOpen} onOk={create} onCancel={() => setModalOpen(false)} okText="创建" cancelText="取消">
        <Form form={form} layout="vertical">
          <Form.Item name="backupName" label="备份组名称" rules={[{ required: true }]}><Input placeholder="如 palworld-daily" /></Form.Item>
          <Form.Item name="sourcePath" label="源路径" rules={[{ required: true }]}><Input placeholder="/home/steam/..." /></Form.Item>
          <Form.Item name="maxKeep" label="最大保留数 (0=不限)" initialValue={10}><InputNumber min={0} /></Form.Item>
        </Form>
      </Modal>
    </>
  )
}
