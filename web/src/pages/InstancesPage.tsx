import { Button, Card, Form, Input, List, Tabs, Tag, Upload, message, Modal, Table, Space, Select } from 'antd'
import { UploadOutlined } from '@ant-design/icons'
import { useEffect, useState } from 'react'
import { PageHeader } from '../components/PageHeader'
import { api } from '../api'
export default function InstancesPage() {
  const [instances, setInstances] = useState<any[]>([])
  const [players, setPlayers] = useState<any[]>([])
  const [saves, setSaves] = useState<any[]>([])
  const [whitelist, setWhitelist] = useState<any[]>([])
  const [banlist, setBanlist] = useState<any[]>([])
  const [config, setConfig] = useState<any>({ categories: [] })
  async function refresh() {
    const i = await api<{ instances: any[] }>('/api/instances')
    setInstances(i.instances || [])
    const optional = await Promise.allSettled([api<{ players: any[] }>('/api/players'), api<{ saves: any[] }>('/api/saves'), api<{ whitelist: any[] }>('/api/whitelist'), api<{ banlist: any[] }>('/api/banlist'), api<any>('/api/config')])
    const [p, s, w, b, c] = optional
    if (p.status === 'fulfilled') setPlayers(p.value.players || [])
    if (s.status === 'fulfilled') setSaves(s.value.saves || [])
    if (w.status === 'fulfilled') setWhitelist(w.value.whitelist || [])
    if (b.status === 'fulfilled') setBanlist(b.value.banlist || [])
    if (c.status === 'fulfilled') setConfig(c.value)
  }
  useEffect(() => { refresh() }, [])
  async function post(path: string, body: any, ok: string) { try { const d = await api<any>(path, { method: 'POST', body }); if (d.ok === false) throw new Error(d.error || d.message); message.success(d.message || ok); refresh() } catch (e: any) { message.error(e.message) } }
  return <>
    <PageHeader title="实例管理" desc="实例启停、控制台、游戏配置与 Palworld 运维" actions={<Button onClick={refresh}>刷新</Button>} />
    <Tabs items={[
      { key: 'instances', label: '实例', children: <GenericInstances instances={instances} post={post} /> },
      { key: 'game-config', label: '游戏配置', children: <GameConfig instances={instances} /> },
      { key: 'players', label: '玩家', children: <Card><List dataSource={players} renderItem={(p) => <List.Item actions={[<Button onClick={() => post('/api/kick', { steamid: p.steamid }, '已踢出')}>踢出</Button>, <Button danger onClick={() => post('/api/ban', { steamid: p.steamid }, '已封禁')}>封禁</Button>]}><List.Item.Meta title={p.name || p.steamid} description={`SteamID: ${p.steamid || '-'} PlayerUID: ${p.playeruid || '-'}`} /></List.Item>} /></Card> },
      { key: 'saves', label: '存档', children: <Saves saves={saves} refresh={refresh} /> },
      { key: 'config', label: '配置', children: <ConfigView config={config} /> },
      { key: 'whitelist', label: '白名单', children: <Whitelist whitelist={whitelist} post={post} /> },
      { key: 'banlist', label: '封禁', children: <Card><List dataSource={banlist} renderItem={(b) => <List.Item actions={[<Button onClick={() => post('/api/banlist/unban', { steamid: b.steamid }, '已解封')}>解封</Button>]}>{b.steamid}</List.Item>} /></Card> }
    ]} />
  </>
}

function GenericInstances({ instances, post }: { instances: any[]; post: (p: string, b: any, ok: string) => void }) {
  const [editModal, setEditModal] = useState<{ open: boolean; data: any }>({ open: false, data: null })
  const [editForm] = Form.useForm()
  const [consoleModal, setConsoleModal] = useState<{ open: boolean; instance: any; logs: string; input: string }>({ open: false, instance: null, logs: '', input: '' })
  const [stopping, setStopping] = useState<string>('')
  const [starting, setStarting] = useState<string>('')
  function delInstance(r: any) {
    Modal.confirm({
      title: '确认删除实例', content: `${r.name} (${r.id})`, okText: '删除', okType: 'danger', cancelText: '取消',
      onOk: () => post('/api/instances/delete', { id: r.id }, '已删除'),
    })
  }
  function openEdit(r: any) { editForm.setFieldsValue(r); setEditModal({ open: true, data: r }) }
  async function openConsole(r: any) {
    try {
      const d = await api<{ logs: string }>(`/api/instances/logs?id=${r.id}&tail=1`)
      setConsoleModal({ open: true, instance: r, logs: d.logs || '(无日志)', input: '' })
    } catch (e: any) { message.error(e.message) }
  }
  async function refreshConsole() {
    if (!consoleModal.instance) return
    try {
      const d = await api<{ logs: string }>(`/api/instances/logs?id=${consoleModal.instance.id}&tail=1`)
      setConsoleModal(prev => ({ ...prev, logs: d.logs || '(无日志)' }))
    } catch (e: any) { message.error(e.message) }
  }
  async function sendConsoleInput() {
    if (!consoleModal.instance || !consoleModal.input.trim()) return
    try {
      await api('/api/instances/input', { method: 'POST', body: { id: consoleModal.instance.id, data: consoleModal.input + '\n' } })
      setConsoleModal(prev => ({ ...prev, input: '' }))
      setTimeout(refreshConsole, 500)
    } catch (e: any) { message.error(e.message) }
  }
  async function stopInstance(r: any) {
    setStopping(r.id)
    try { await api('/api/instances/stop', { method: 'POST', body: { id: r.id } }); message.success('已停止') } catch (e: any) { message.error(e.message) }
    finally { setStopping('') }
  }
  async function startInstance(r: any) {
    setStarting(r.id)
    try {
      const check = await api<{ ready: boolean; command: string; problems: string[]; warnings: string[] }>(`/api/instances/readiness?id=${encodeURIComponent(r.id)}`)
      if (!check.ready) {
        Modal.error({ title: '实例暂不可启动', content: <div>{(check.problems || []).map((p) => <div key={p}>{p}</div>)}</div> })
        return
      }
      const run = () => post('/api/instances/start', { id: r.id }, '已启动')
      if ((check.warnings || []).length) {
        Modal.confirm({ title: '确认启动实例', content: <div>{check.warnings.map((p) => <div key={p}>{p}</div>)}<div>启动命令：{check.command}</div></div>, okText: '启动', cancelText: '取消', onOk: run })
      } else {
        await run()
      }
    } catch (e: any) { message.error(e.message) }
    finally { setStarting('') }
  }
  async function saveEdit() {
    try {
      const v = await editForm.validateFields()
      await api('/api/instances/update', { method: 'POST', body: { ...v, id: editModal.data.id } })
      message.success('已更新'); setEditModal({ open: false, data: null })
    } catch (e: any) { if (e.errorFields?.length) return; message.error(e.message) }
  }
  return <Card title="通用实例" extra={<span>支持自定义工作目录、启动命令、停止命令</span>}>
    <Form layout="inline" onFinish={(v) => post('/api/instances/create', { ...v, instanceType: 'generic' }, '已创建实例')}>
      <Form.Item name="name" rules={[{ required: true, message: '请输入实例名称' }]}><Input placeholder="实例名称" /></Form.Item>
      <Form.Item name="workingDirectory" rules={[{ required: true, message: '请输入工作目录' }]}><Input placeholder="工作目录" /></Form.Item>
      <Form.Item name="startCommand" rules={[{ required: true, message: '请输入启动命令' }]}><Input placeholder="启动命令" /></Form.Item>
      <Form.Item name="stopCommand"><Input placeholder="停止命令" /></Form.Item>
      <Button htmlType="submit" type="primary">创建</Button>
    </Form>
    <Table rowKey="id" dataSource={instances} pagination={false} className="section-card" columns={[
      { title: '名称', dataIndex: 'name' }, { title: '类型', dataIndex: 'instanceType' }, { title: '状态', dataIndex: 'status', render: (s: string) => <Tag color={s === 'running' ? 'green' : s === 'error' ? 'red' : 'default'}>{s}</Tag> }, { title: 'PID', dataIndex: 'pid', width: 80, render: (p: number) => p || '-' },
      { title: '操作', render: (_, r: any) => <Space><Button onClick={() => startInstance(r)} disabled={r.status === 'running'} loading={starting === r.id}>启动</Button><Button onClick={() => stopInstance(r)} disabled={r.status !== 'running'} loading={stopping === r.id}>停止</Button><Button onClick={() => post('/api/instances/restart', { id: r.id }, '已重启')} disabled={r.status !== 'running'}>重启</Button><Button onClick={() => openConsole(r)} disabled={r.status !== 'running' && !r.lastStarted}>控制台</Button><Button onClick={() => openEdit(r)}>编辑</Button><Button danger onClick={() => delInstance(r)}>删除</Button></Space> }
    ]} />
    <Modal title="编辑实例" open={editModal.open} onOk={saveEdit} onCancel={() => setEditModal({ open: false, data: null })} okText="保存" cancelText="取消">
      <Form form={editForm} layout="vertical">
        <Form.Item name="name" label="名称" rules={[{ required: true, message: '请输入实例名称' }]}><Input /></Form.Item>
        <Form.Item name="workingDirectory" label="工作目录" rules={[{ required: true, message: '请输入工作目录' }]}><Input disabled={editModal.data?.status === 'running' || editModal.data?.status === 'starting'} /></Form.Item>
        <Form.Item name="startCommand" label="启动命令" rules={[{ required: editForm.getFieldValue('instanceType') !== 'minecraft-java', message: '请输入启动命令' }]}><Input disabled={editModal.data?.status === 'running' || editModal.data?.status === 'starting'} /></Form.Item>
        <Form.Item name="stopCommand" label="停止命令"><Select options={[{value:'ctrl+c',label:'Ctrl+C'},{value:'stop',label:'stop'},{value:'exit',label:'exit'},{value:'quit',label:'quit'}]} /></Form.Item>
        <Form.Item name="instanceType" label="类型"><Select options={[{value:'generic',label:'通用'},{value:'palworld',label:'Palworld'},{value:'minecraft-java',label:'Minecraft Java'},{value:'minecraft-bedrock',label:'Minecraft Bedrock'},{value:'valheim',label:'Valheim'},{value:'terraria',label:'Terraria'}]} /></Form.Item>
      </Form>
    </Modal>
    <Modal title={consoleModal.instance ? `实例控制台 - ${consoleModal.instance.name}` : '实例控制台'} open={consoleModal.open} onOk={() => setConsoleModal({ open: false, instance: null, logs: '', input: '' })} onCancel={() => setConsoleModal({ open: false, instance: null, logs: '', input: '' })} okText="关闭" cancelText="取消" width={900}>
      <Space style={{ marginBottom: 8 }}>
        <Button onClick={refreshConsole}>刷新日志</Button>
        {consoleModal.instance?.pid ? <Tag color="green">PID {consoleModal.instance.pid}</Tag> : <Tag>未运行</Tag>}
      </Space>
      <pre className="log-box" style={{ maxHeight: 420 }}>{consoleModal.logs}</pre>
      <Input.Search value={consoleModal.input} onChange={e => setConsoleModal(prev => ({ ...prev, input: e.target.value }))} onSearch={sendConsoleInput} enterButton="发送" placeholder="发送命令到实例 stdin" disabled={consoleModal.instance?.status !== 'running'} />
    </Modal>
  </Card>
}

function Saves({ saves, refresh }: { saves: any[]; refresh: () => void }) {
  async function backup() { await api('/api/saves/backup', { method: 'POST', body: {} }); message.success('备份指令已发送'); refresh() }
  async function remove(name: string) { await api('/api/saves/delete', { method: 'POST', body: { name } }); message.success('已删除'); refresh() }
  async function upload(file: File) {
    const content = await new Promise<string>((resolve, reject) => { const r = new FileReader(); r.onload = () => resolve(String(r.result).split(',')[1]); r.onerror = reject; r.readAsDataURL(file) })
    await api('/api/saves/upload', { method: 'POST', body: { name: file.name, content } }); message.success('已上传'); refresh()
  }
  return <Card title="存档备份" extra={<><Button onClick={backup}>立即备份</Button><Upload showUploadList={false} beforeUpload={(file) => { upload(file); return false }}><Button icon={<UploadOutlined />}>上传</Button></Upload></>}>
    <List dataSource={saves} renderItem={(s) => <List.Item actions={[<Button href={`/api/saves/download?name=${encodeURIComponent(s.name)}`}>下载</Button>, <Button onClick={() => Modal.confirm({ title: '确认恢复？', content: s.name, onOk: () => api('/api/saves/restore', { method: 'POST', body: { name: s.name } }) })}>恢复</Button>, <Button danger onClick={() => remove(s.name)}>删除</Button>]}><List.Item.Meta title={s.name} description={`${new Date(s.time * 1000).toLocaleString()} · ${(s.size / 1048576).toFixed(2)} MB`} /></List.Item>} />
  </Card>
}

function ConfigView({ config }: { config: any }) {
  return <Card>{(config.categories || []).map((cat: any) => <Card key={cat.name} type="inner" title={cat.name} className="section-card"><div className="config-grid">{cat.items.map((it: any) => <div key={it.key}><Tag color="blue">{it.label}</Tag><span>{String(it.value)}</span></div>)}</div></Card>)}</Card>
}

function GameConfig({ instances }: { instances: any[] }) {
  const [templates, setTemplates] = useState<any[]>([])
  const [instanceId, setInstanceId] = useState<string>('')
  const [templateId, setTemplateId] = useState<string>('')
  const [loaded, setLoaded] = useState<any>(null)
  const [form] = Form.useForm()

  useEffect(() => { api<{ templates: any[] }>('/api/game-config/templates').then(d => setTemplates(d.templates || [])).catch(e => message.error(e.message)) }, [])

  const instance = instances.find(i => i.id === instanceId)
  const availableTemplates = templates.filter(t => !instance || !t.instanceType || t.instanceType === instance.instanceType)

  async function loadConfig() {
    if (!instanceId || !templateId) return message.warning('请选择实例和配置模板')
    try {
      const d = await api<any>(`/api/game-config/read?instanceId=${encodeURIComponent(instanceId)}&templateId=${encodeURIComponent(templateId)}`)
      setLoaded(d)
      form.setFieldsValue(d.values || {})
    } catch (e: any) { message.error(e.message) }
  }

  async function saveConfig() {
    if (!loaded) return
    try {
      const values = await form.validateFields()
      await api('/api/game-config/save', { method: 'POST', body: { instanceId, templateId, values } })
      message.success('配置已保存')
    } catch (e: any) { if (e.errorFields?.length) return; message.error(e.message) }
  }

  return <Card title="通用游戏配置编辑器" extra={<Button onClick={loadConfig}>读取配置</Button>}>
    <Space wrap style={{ marginBottom: 16 }}>
      <Select style={{ width: 260 }} placeholder="选择实例" value={instanceId || undefined} onChange={(v) => { setInstanceId(v); setTemplateId(''); setLoaded(null); form.resetFields() }} options={instances.map(i => ({ value: i.id, label: `${i.name} (${i.instanceType})` }))} />
      <Select style={{ width: 300 }} placeholder={instance ? '选择配置模板' : '请先选择实例'} value={templateId || undefined} disabled={!instance || availableTemplates.length === 0} onChange={(v) => { setTemplateId(v); setLoaded(null); form.resetFields() }} options={availableTemplates.map(t => ({ value: t.id, label: t.name }))} />
      {loaded?.path && <Tag color="blue">{loaded.path}</Tag>}
      {loaded?.template?.format && <Tag>{loaded.template.format}</Tag>}
    </Space>
    {instance && availableTemplates.length === 0 && <div style={{ color: '#999' }}>当前实例类型暂无通用配置模板，可在文件管理中直接编辑配置文件。</div>}
    {loaded ? <Form form={form} layout="vertical">
      {(loaded.template.fields || []).map((field: any) => <Form.Item key={field.key} name={field.key} label={field.label} extra={field.description || fieldHint(field)} rules={fieldRules(field)}>
        {field.type === 'bool' ? <Select options={[{ value: 'true', label: 'true' }, { value: 'false', label: 'false' }]} /> :
          field.type === 'password' ? <Input.Password /> :
          field.type === 'select' ? <Select options={(field.options || []).map((o: any) => ({ value: o.value, label: o.label }))} /> :
          field.type === 'number' ? <Input type="number" min={field.min} max={field.max} /> : <Input />}
      </Form.Item>)}
      <Button type="primary" onClick={saveConfig}>保存配置</Button>
    </Form> : <div style={{ color: '#999' }}>选择实例和模板后读取配置。未存在的配置文件会使用模板默认值。</div>}
  </Card>
}

function fieldRules(field: any) {
  const rules: any[] = []
  if (field.required) rules.push({ required: true, message: `${field.label}不能为空` })
  if (field.type === 'number') {
    rules.push({
      validator: (_: any, value: string) => {
        if (value === undefined || value === '') return Promise.resolve()
        const n = Number(value)
        if (Number.isNaN(n)) return Promise.reject(new Error(`${field.label}必须是数字`))
        if (field.min !== undefined && field.min !== '' && n < Number(field.min)) return Promise.reject(new Error(`${field.label}不能小于 ${field.min}`))
        if (field.max !== undefined && field.max !== '' && n > Number(field.max)) return Promise.reject(new Error(`${field.label}不能大于 ${field.max}`))
        return Promise.resolve()
      }
    })
  }
  return rules
}

function fieldHint(field: any) {
  const parts = []
  if (field.min !== undefined && field.min !== '') parts.push(`最小 ${field.min}`)
  if (field.max !== undefined && field.max !== '') parts.push(`最大 ${field.max}`)
  if (field.options?.length) parts.push(`可选 ${field.options.map((o: any) => o.value).join(' / ')}`)
  return parts.join('，')
}

function Whitelist({ whitelist, post }: { whitelist: any[]; post: (p: string, b: any, ok: string) => void }) {
  return <Card title="白名单" extra={<Button onClick={() => post('/api/whitelist/check', {}, '已检查')}>立即检查</Button>}>
    <Form layout="inline" onFinish={(v) => post('/api/whitelist/add', v, '已添加')}><Form.Item name="name"><Input placeholder="名称" /></Form.Item><Form.Item name="steamid"><Input placeholder="SteamID" /></Form.Item><Form.Item name="playeruid"><Input placeholder="PlayerUID" /></Form.Item><Button htmlType="submit" type="primary">添加</Button></Form>
    <List dataSource={whitelist} renderItem={(w) => <List.Item actions={[<Button onClick={() => post('/api/whitelist/remove', { steamid: w.steamid }, '已移除')}>移除</Button>]}>{w.name || '-'} · {w.steamid || '*'} · {w.playeruid || '*'}</List.Item>} />
  </Card>
}
