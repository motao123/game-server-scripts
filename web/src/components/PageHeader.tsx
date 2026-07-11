import { ReactNode } from 'react'
import { Typography } from 'antd'

export function PageHeader({ title, desc, actions }: { title: string; desc?: string; actions?: ReactNode }) {
  return (
    <div className="page-header">
      <div>
        <Typography.Title level={3}>{title}</Typography.Title>
        {desc && <Typography.Text type="secondary">{desc}</Typography.Text>}
      </div>
      <div>{actions}</div>
    </div>
  )
}
