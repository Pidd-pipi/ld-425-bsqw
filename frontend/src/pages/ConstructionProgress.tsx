import { useEffect, useMemo, useState } from 'react'
import { Button, Card, Form, Input, InputNumber, Modal, Popconfirm, Select, Space, Table, Typography, message, Upload } from 'antd'
import { CheckOutlined, PlayCircleOutlined, PlusOutlined, UploadOutlined } from '@ant-design/icons'
import StatusBadge from '@/components/common/StatusBadge'
import Timeline from '@/components/common/Timeline'
import StepIndicator from '@/components/common/StepIndicator'
import StatCard from '@/components/common/StatCard'
import { useConstructionStore } from '@/stores/constructionStore'
import { useProjectStore } from '@/stores/projectStore'
import { useChangeOrderStore } from '@/stores/changeOrderStore'
import { useAuthStore } from '@/stores/authStore'
import { acceptConstruction, createConstruction, updateConstructionStatus } from '@/api/construction'
import { createChangeOrder, reviewChangeOrder } from '@/api/changeOrder'
import { uploadFile } from '@/utils/upload'
import { extractErrorMessage } from '@/utils/request'
import { formatCurrency } from '@/utils/formatBudget'
import { ConstructionStatus, Role, ConstructionName, ChangeOrderStatus } from '@/types/enums'
import type { ChangeOrder, ConstructionNode } from '@/types'

export default function ConstructionProgress() {
  const { nodes, fetchNodes } = useConstructionStore()
  const { projects, fetchProjects } = useProjectStore()
  const { orders, summary, fetchOrders, fetchSummary } = useChangeOrderStore()
  const user = useAuthStore((state) => state.user)
  const [projectId, setProjectId] = useState<number>()
  const [acceptTarget, setAcceptTarget] = useState<ConstructionNode | null>(null)
  const [createOpen, setCreateOpen] = useState(false)
  const [changeOpen, setChangeOpen] = useState(false)
  const [photoUrls, setPhotoUrls] = useState<string[]>([])
  const [form] = Form.useForm()
  const [acceptForm] = Form.useForm()
  const [changeForm] = Form.useForm()

  useEffect(() => {
    fetchProjects()
  }, [fetchProjects])

  useEffect(() => {
    fetchNodes(projectId)
    fetchOrders(projectId)
    fetchSummary(projectId)
  }, [fetchNodes, fetchOrders, fetchSummary, projectId])

  const filtered = useMemo(() => {
    if (!projectId) return nodes
    return nodes.filter((n) => n.project_id === projectId)
  }, [nodes, projectId])

  const filteredOrders = useMemo(() => {
    if (!projectId) return orders
    return orders.filter((o) => o.project_id === projectId)
  }, [orders, projectId])

  const nodeNameMap = useMemo(() => {
    const map = new Map<number, string>()
    nodes.forEach((n) => map.set(n.id, n.name))
    return map
  }, [nodes])

  // 待审批中的节点，同一节点只能有一个待审批变更。
  const pendingNodeIds = useMemo(() => {
    const set = new Set<number>()
    filteredOrders.forEach((o) => {
      if (o.status === ChangeOrderStatus.Pending) set.add(o.node_id)
    })
    return set
  }, [filteredOrders])

  const canOperate = user?.role === Role.Admin || user?.role === Role.Contractor || user?.role === Role.ProjectManager
  const canSubmitChange = user?.role === Role.Admin || user?.role === Role.Contractor
  const canReviewChange = user?.role === Role.Admin || user?.role === Role.ProjectManager || user?.role === Role.Owner

  const refreshAll = async () => {
    await Promise.all([fetchNodes(projectId), fetchOrders(projectId), fetchSummary(projectId)])
  }

  const startNode = async (node: ConstructionNode) => {
    try {
      await updateConstructionStatus(node.id, ConstructionStatus.InProgress)
      message.success('节点已开工')
      await fetchNodes(projectId)
    } catch (error) {
      message.error(extractErrorMessage(error))
    }
  }

  const completeNode = async (node: ConstructionNode) => {
    try {
      await updateConstructionStatus(node.id, ConstructionStatus.Completed)
      message.success('节点已完工')
      await fetchNodes(projectId)
    } catch (error) {
      message.error(extractErrorMessage(error))
    }
  }

  const onAccept = async () => {
    const values = await acceptForm.validateFields()
    if (!acceptTarget) return
    try {
      await acceptConstruction(acceptTarget.id, { ...values, photos: photoUrls })
      message.success('验收完成')
      setAcceptTarget(null)
      setPhotoUrls([])
      acceptForm.resetFields()
      await fetchNodes(projectId)
    } catch (error) {
      message.error(extractErrorMessage(error))
    }
  }

  const onCreate = async () => {
    const values = await form.validateFields()
    try {
      await createConstruction({ ...values, project_id: projectId! })
      message.success('创建成功')
      setCreateOpen(false)
      form.resetFields()
      await fetchNodes(projectId)
    } catch (error) {
      message.error(extractErrorMessage(error))
    }
  }

  const onCreateChange = async () => {
    const values = await changeForm.validateFields()
    try {
      await createChangeOrder({ ...values, project_id: projectId! })
      message.success('变更签证已提交，待审批')
      setChangeOpen(false)
      changeForm.resetFields()
      await refreshAll()
    } catch (error) {
      message.error(extractErrorMessage(error))
    }
  }

  const onReviewChange = async (order: ChangeOrder, approved: boolean) => {
    try {
      await reviewChangeOrder(order.id, { approved })
      message.success(approved ? '已批准，预算与项目汇总已生效' : '已驳回')
      await refreshAll()
    } catch (error) {
      message.error(extractErrorMessage(error))
      await refreshAll()
    }
  }

  return (
    <div>
      <Space style={{ marginBottom: 16 }} wrap>
        <Typography.Title level={3} style={{ margin: 0 }}>施工进度</Typography.Title>
        <Select
          style={{ width: 240 }}
          placeholder="选择项目"
          allowClear
          value={projectId}
          onChange={setProjectId}
          options={projects.map((p) => ({ label: p.name, value: p.id }))}
        />
        {user?.role === Role.Admin || user?.role === Role.ProjectManager ? (
          <Button type="primary" disabled={!projectId} onClick={() => setCreateOpen(true)}>新增节点</Button>
        ) : null}
        {canSubmitChange ? (
          <Button icon={<PlusOutlined />} disabled={!projectId} onClick={() => setChangeOpen(true)}>提交变更签证</Button>
        ) : null}
      </Space>

      <StepIndicator
        current={filtered.filter((n) => n.status === ConstructionStatus.Completed).length}
        items={filtered.map((n) => n.name)}
      />

      <Card style={{ marginTop: 16 }}>
        <Timeline
          items={filtered.map((node) => ({
            title: node.name,
            status: node.status,
            time: `${node.planned_start_date || '-'} ~ ${node.planned_end_date || '-'}`,
            description: `验收状态：${node.acceptance_status}${node.acceptance_note ? ' · ' + node.acceptance_note : ''}`,
          }))}
        />
      </Card>

      <Card title="节点操作" style={{ marginTop: 16 }}>
        <Table
          rowKey="id"
          dataSource={filtered}
          pagination={false}
          columns={[
            { title: '节点名称', dataIndex: 'name' },
            { title: '状态', dataIndex: 'status', render: (v) => <StatusBadge status={v} /> },
            { title: '验收状态', dataIndex: 'acceptance_status', render: (v) => <StatusBadge status={v} /> },
            { title: '计划开始', dataIndex: 'planned_start_date' },
            { title: '计划结束', dataIndex: 'planned_end_date' },
            {
              title: '操作',
              render: (_, record) =>
                canOperate ? (
                  <Space>
                    {record.status === ConstructionStatus.Pending ? (
                      <Button size="small" icon={<PlayCircleOutlined />} onClick={() => startNode(record)}>开工</Button>
                    ) : null}
                    {record.status === ConstructionStatus.InProgress ? (
                      <Button size="small" type="primary" onClick={() => completeNode(record)}>完工</Button>
                    ) : null}
                    {record.status === ConstructionStatus.Completed ? (
                      <Button size="small" icon={<CheckOutlined />} onClick={() => setAcceptTarget(record)}>验收</Button>
                    ) : null}
                  </Space>
                ) : null,
            },
          ]}
        />
      </Card>

      <Card title="变更签证" style={{ marginTop: 16 }}>
        <Space size="middle" style={{ marginBottom: 16 }} wrap>
          <StatCard title="待审批变更金额" value={summary?.pending_amount ?? filteredOrders.filter((o) => o.status === ChangeOrderStatus.Pending).reduce((s, o) => s + o.amount, 0)} prefix="¥" />
          <StatCard title="已批准变更金额" value={summary?.approved_amount ?? filteredOrders.filter((o) => o.status === ChangeOrderStatus.Approved).reduce((s, o) => s + o.amount, 0)} prefix="¥" />
          <StatCard title="剩余可变更额度" value={summary?.remaining_amount ?? 0} prefix="¥" />
        </Space>
        <Table<ChangeOrder>
          rowKey="id"
          dataSource={filteredOrders}
          pagination={false}
          columns={[
            { title: '施工节点', dataIndex: 'node_id', render: (v: number) => nodeNameMap.get(v) || `#${v}` },
            { title: '变更内容', dataIndex: 'title' },
            { title: '增项金额', dataIndex: 'amount', render: (v: number) => formatCurrency(v) },
            { title: '工期影响', dataIndex: 'schedule_impact_days', render: (v: number) => `${v} 天` },
            { title: '状态', dataIndex: 'status', render: (v) => <StatusBadge status={v} /> },
            { title: '审批意见', dataIndex: 'review_note', render: (v: string) => v || '-' },
            {
              title: '操作',
              render: (_, record) =>
                canReviewChange && record.status === ChangeOrderStatus.Pending ? (
                  <Space>
                    <Popconfirm title="批准后将计入已用预算" onConfirm={() => onReviewChange(record, true)}>
                      <Button size="small" type="primary">批准</Button>
                    </Popconfirm>
                    <Popconfirm title="驳回不改预算" onConfirm={() => onReviewChange(record, false)}>
                      <Button size="small" danger>驳回</Button>
                    </Popconfirm>
                  </Space>
                ) : null,
            },
          ]}
        />
      </Card>

      <Modal title="新增施工节点" open={createOpen} onOk={onCreate} onCancel={() => setCreateOpen(false)}>
        <Form form={form} layout="vertical">
          <Form.Item name="name" label="节点名称" rules={[{ required: true }]}>
            <Select options={ConstructionName.map((n) => ({ label: n, value: n }))} />
          </Form.Item>
          <Form.Item name="planned_start_date" label="计划开始日期"><Input placeholder="YYYY-MM-DD" /></Form.Item>
          <Form.Item name="planned_end_date" label="计划结束日期"><Input placeholder="YYYY-MM-DD" /></Form.Item>
        </Form>
      </Modal>

      <Modal title="提交变更签证" open={changeOpen} onOk={onCreateChange} onCancel={() => setChangeOpen(false)} destroyOnClose>
        <Form form={changeForm} layout="vertical" initialValues={{ schedule_impact_days: 0 }}>
          <Form.Item name="node_id" label="施工节点" rules={[{ required: true, message: '请选择施工节点' }]}>
            <Select
              options={filtered.map((n) => ({ label: n.name, value: n.id, disabled: pendingNodeIds.has(n.id) }))}
              placeholder="同一节点仅允许一个待审批变更"
            />
          </Form.Item>
          <Form.Item name="title" label="变更内容" rules={[{ required: true, message: '请填写变更内容' }, { max: 120 }]}>
            <Input placeholder="例如：客厅增加插座与回路" />
          </Form.Item>
          <Form.Item name="amount" label="增项金额（元）" rules={[{ required: true, message: '请填写增项金额' }]}>
            <InputNumber min={0.01} precision={2} style={{ width: '100%' }} placeholder="待审批不计入已用预算" />
          </Form.Item>
          <Form.Item name="schedule_impact_days" label="工期影响（天）">
            <InputNumber min={0} precision={0} style={{ width: '100%' }} />
          </Form.Item>
        </Form>
      </Modal>

      <Modal title="施工验收" open={!!acceptTarget} onOk={onAccept} onCancel={() => setAcceptTarget(null)}>
        <Form form={acceptForm} layout="vertical" initialValues={{ accepted: true }}>
          <Form.Item name="accepted" label="验收结论" rules={[{ required: true }]}>
            <Select options={[{ label: '验收通过', value: true }, { label: '验收不通过', value: false }]} />
          </Form.Item>
          <Form.Item name="note" label="验收说明"><Input.TextArea rows={3} /></Form.Item>
          <Form.Item label="验收照片">
            <Upload
              beforeUpload={async (file) => {
                try {
                  const url = await uploadFile(file)
                  setPhotoUrls((prev) => [...prev, url])
                } catch (error) {
                  message.error(extractErrorMessage(error))
                }
                return false
              }}
            >
              <Button icon={<UploadOutlined />}>上传照片</Button>
            </Upload>
            {photoUrls.map((url) => (
              <div key={url} style={{ marginTop: 8 }}>{url}</div>
            ))}
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}
