import { useEffect, useMemo, useState } from 'react'
import { Button, Card, Form, Input, InputNumber, Modal, Select, Space, Table, Tag, Typography, message, Upload } from 'antd'
import { CheckOutlined, PlayCircleOutlined, UploadOutlined } from '@ant-design/icons'
import StatusBadge from '@/components/common/StatusBadge'
import Timeline from '@/components/common/Timeline'
import StepIndicator from '@/components/common/StepIndicator'
import { useConstructionStore } from '@/stores/constructionStore'
import { useProjectStore } from '@/stores/projectStore'
import { useBudgetStore } from '@/stores/budgetStore'
import { useChangeOrderStore } from '@/stores/changeOrderStore'
import { useAuthStore } from '@/stores/authStore'
import { acceptConstruction, createConstruction, updateConstructionStatus } from '@/api/construction'
import { createChangeOrder, reviewChangeOrder } from '@/api/change'
import { uploadFile } from '@/utils/upload'
import { extractErrorMessage } from '@/utils/request'
import { formatCurrency } from '@/utils/formatBudget'
import { ConstructionStatus, Role, ConstructionName, ChangeOrderStatus } from '@/types/enums'
import type { ChangeOrder, ConstructionNode } from '@/types'

interface ReviewTarget {
  order: ChangeOrder
  approved: boolean
}

export default function ConstructionProgress() {
  const { nodes, fetchNodes } = useConstructionStore()
  const { projects, fetchProjects } = useProjectStore()
  const { budgets, fetchBudgets } = useBudgetStore()
  const { orders, fetchOrders } = useChangeOrderStore()
  const user = useAuthStore((state) => state.user)
  const [projectId, setProjectId] = useState<number>()
  const [acceptTarget, setAcceptTarget] = useState<ConstructionNode | null>(null)
  const [createOpen, setCreateOpen] = useState(false)
  const [changeOpen, setChangeOpen] = useState(false)
  const [changeNode, setChangeNode] = useState<ConstructionNode | null>(null)
  const [reviewTarget, setReviewTarget] = useState<ReviewTarget | null>(null)
  const [photoUrls, setPhotoUrls] = useState<string[]>([])
  const [form] = Form.useForm()
  const [acceptForm] = Form.useForm()
  const [changeForm] = Form.useForm()
  const [reviewForm] = Form.useForm()

  useEffect(() => {
    fetchProjects()
  }, [fetchProjects])

  useEffect(() => {
    fetchNodes(projectId)
    fetchOrders(projectId)
    fetchBudgets(projectId)
  }, [fetchNodes, fetchOrders, fetchBudgets, projectId])

  const filtered = useMemo(() => {
    if (!projectId) return nodes
    return nodes.filter((n) => n.project_id === projectId)
  }, [nodes, projectId])

  const projectOrders = useMemo(() => {
    if (!projectId) return orders
    return orders.filter((o) => o.project_id === projectId)
  }, [orders, projectId])

  const pendingOrders = useMemo(
    () => projectOrders.filter((o) => o.status === ChangeOrderStatus.Pending),
    [projectOrders],
  )
  const approvedOrders = useMemo(
    () => projectOrders.filter((o) => o.status === ChangeOrderStatus.Approved),
    [projectOrders],
  )
  const pendingAmount = useMemo(() => pendingOrders.reduce((sum, o) => sum + o.amount, 0), [pendingOrders])
  const approvedAmount = useMemo(() => approvedOrders.reduce((sum, o) => sum + o.amount, 0), [approvedOrders])

  const currentProject = useMemo(() => projects.find((p) => p.id === projectId), [projects, projectId])
  const projectBudgets = useMemo(
    () => budgets.filter((b) => b.project_id === projectId),
    [budgets, projectId],
  )
  const usedBudget = useMemo(
    () => projectBudgets.reduce((sum, b) => sum + b.actual_amount, 0),
    [projectBudgets],
  )
  const remainingBudget = Math.max((currentProject?.contract_amount ?? 0) - usedBudget, 0)

  const canOperate = user?.role === Role.Admin || user?.role === Role.Contractor || user?.role === Role.ProjectManager
  const canSubmitChange = user?.role === Role.Admin || user?.role === Role.Contractor || user?.role === Role.ProjectManager
  const canReviewChange = user?.role === Role.Admin || user?.role === Role.ProjectManager || user?.role === Role.Owner

  const nodeNameById = useMemo(() => {
    const map = new Map<number, string>()
    filtered.forEach((n) => map.set(n.id, n.name))
    return map
  }, [filtered])

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

  const openChangeModal = (node: ConstructionNode) => {
    setChangeNode(node)
    changeForm.resetFields()
    setChangeOpen(true)
  }

  const onSubmitChange = async () => {
    const values = await changeForm.validateFields()
    if (!changeNode || !projectId) return
    try {
      await createChangeOrder({
        project_id: projectId,
        node_id: changeNode.id,
        amount: values.amount,
        schedule_impact: values.schedule_impact ?? 0,
        reason: values.reason,
      })
      message.success('变更签证已提交，等待审批')
      setChangeOpen(false)
      setChangeNode(null)
      changeForm.resetFields()
      await Promise.all([fetchOrders(projectId), fetchNodes(projectId)])
    } catch (error) {
      message.error(extractErrorMessage(error))
    }
  }

  const openReviewModal = (order: ChangeOrder, approved: boolean) => {
    setReviewTarget({ order, approved })
    reviewForm.resetFields()
  }

  const onReview = async () => {
    const values = await reviewForm.validateFields()
    if (!reviewTarget) return
    try {
      const result = await reviewChangeOrder(reviewTarget.order.id, {
        approved: reviewTarget.approved,
        comment: values.comment,
      })
      if (result.status === ChangeOrderStatus.Rejected && reviewTarget.approved) {
        message.warning('累计变更额超过合同额与已用预算差额，整单已拒绝')
      } else {
        message.success(reviewTarget.approved ? '变更已批准' : '变更已驳回')
      }
      setReviewTarget(null)
      reviewForm.resetFields()
      await Promise.all([fetchOrders(projectId), fetchBudgets(projectId), fetchProjects()])
    } catch (error) {
      message.error(extractErrorMessage(error))
    }
  }

  const pendingNodeIds = useMemo(() => new Set(pendingOrders.map((o) => o.node_id)), [pendingOrders])

  const columns = [
    { title: '节点名称', dataIndex: 'name' },
    { title: '状态', dataIndex: 'status', render: (v: string) => <StatusBadge status={v} /> },
    { title: '验收状态', dataIndex: 'acceptance_status', render: (v: string) => <StatusBadge status={v} /> },
    { title: '计划开始', dataIndex: 'planned_start_date' },
    { title: '计划结束', dataIndex: 'planned_end_date' },
    {
      title: '操作',
      render: (_: unknown, record: ConstructionNode) => (
        <Space wrap>
          {record.status === ConstructionStatus.Pending ? (
            <Button size="small" icon={<PlayCircleOutlined />} disabled={!canOperate} onClick={() => startNode(record)}>开工</Button>
          ) : null}
          {record.status === ConstructionStatus.InProgress ? (
            <Button size="small" type="primary" disabled={!canOperate} onClick={() => completeNode(record)}>完工</Button>
          ) : null}
          {record.status === ConstructionStatus.Completed ? (
            <Button size="small" icon={<CheckOutlined />} disabled={!canOperate} onClick={() => setAcceptTarget(record)}>验收</Button>
          ) : null}
          {canSubmitChange ? (
            pendingNodeIds.has(record.id) ? (
              <Tag color="orange">变更待审批</Tag>
            ) : (
              <Button size="small" type="dashed" disabled={!projectId} onClick={() => openChangeModal(record)}>提交变更</Button>
            )
          ) : null}
        </Space>
      ),
    },
  ]

  const orderColumns = [
    { title: 'ID', dataIndex: 'id', width: 60 },
    { title: '施工节点', render: (_: unknown, record: ChangeOrder) => nodeNameById.get(record.node_id) || `节点 #${record.node_id}` },
    { title: '增项金额', dataIndex: 'amount', render: (v: number) => formatCurrency(v) },
    { title: '工期影响（天）', dataIndex: 'schedule_impact' },
    { title: '变更事由', dataIndex: 'reason' },
    { title: '状态', dataIndex: 'status', render: (v: string) => <StatusBadge status={v} /> },
    { title: '审批意见', dataIndex: 'review_comment', render: (v: string) => v || '-' },
    {
      title: '审批操作',
      render: (_: unknown, record: ChangeOrder) =>
        record.status === ChangeOrderStatus.Pending && canReviewChange ? (
          <Space>
            <Button size="small" type="primary" onClick={() => openReviewModal(record, true)}>批准</Button>
            <Button size="small" danger onClick={() => openReviewModal(record, false)}>驳回</Button>
          </Space>
        ) : null,
    },
  ]

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

      <Card title="变更签证汇总" style={{ marginTop: 16 }}>
        <Space size="large" wrap>
          <span>合同金额：<b>{formatCurrency(currentProject?.contract_amount ?? 0)}</b></span>
          <span>已用预算：<b>{formatCurrency(usedBudget)}</b></span>
          <span>合同剩余额度：<b style={{ color: '#1677ff' }}>{formatCurrency(remainingBudget)}</b></span>
          <span>待审批变更金额：<b style={{ color: '#d48806' }}>{formatCurrency(pendingAmount)}</b>（{pendingOrders.length} 笔，不计入已用预算）</span>
          <span>已批准变更金额：<b style={{ color: '#389e0d' }}>{formatCurrency(approvedAmount)}</b>（{approvedOrders.length} 笔）</span>
          {currentProject ? (
            <span>累计工期顺延：<b>{currentProject.schedule_delta} 天</b></span>
          ) : null}
        </Space>
      </Card>

      <Card title="节点操作" style={{ marginTop: 16 }}>
        <Table rowKey="id" dataSource={filtered} pagination={false} columns={columns} />
      </Card>

      <Card title="变更签证" style={{ marginTop: 16 }}>
        <Table rowKey="id" dataSource={projectOrders} pagination={false} columns={orderColumns} />
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

      <Modal
        title={`提交变更签证 - ${changeNode?.name ?? ''}`}
        open={changeOpen}
        onOk={onSubmitChange}
        onCancel={() => setChangeOpen(false)}
        okText="提交"
      >
        <Form form={changeForm} layout="vertical" initialValues={{ schedule_impact: 0 }}>
          <Form.Item name="amount" label="增项金额（元）" rules={[{ required: true, message: '请输入增项金额' }]}>
            <InputNumber min={0.01} precision={2} style={{ width: '100%' }} placeholder="本次变更增加金额" />
          </Form.Item>
          <Form.Item name="schedule_impact" label="工期影响（顺延天数）" rules={[{ required: true }]}>
            <InputNumber min={0} precision={0} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="reason" label="变更事由" rules={[{ required: true, message: '请填写变更事由' }]}>
            <Input.TextArea rows={3} maxLength={500} showCount />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title={reviewTarget?.approved ? '批准变更签证' : '驳回变更签证'}
        open={!!reviewTarget}
        onOk={onReview}
        onCancel={() => setReviewTarget(null)}
        okText={reviewTarget?.approved ? '确认批准' : '确认驳回'}
        okButtonProps={{ danger: !reviewTarget?.approved }}
      >
        {reviewTarget ? (
          <Space direction="vertical" style={{ marginBottom: 12 }}>
            <span>节点：{nodeNameById.get(reviewTarget.order.node_id) || `节点 #${reviewTarget.order.node_id}`}</span>
            <span>增项金额：{formatCurrency(reviewTarget.order.amount)}</span>
            <span>工期影响：{reviewTarget.order.schedule_impact} 天</span>
            {reviewTarget.approved ? (
              <span style={{ color: '#d48806' }}>
                批准后累计变更额不得超过合同剩余额度 {formatCurrency(remainingBudget)}，超出将整单拒绝
              </span>
            ) : null}
          </Space>
        ) : null}
        <Form form={reviewForm} layout="vertical">
          <Form.Item name="comment" label="审批意见">
            <Input.TextArea rows={3} maxLength={500} showCount placeholder="可填写审批说明" />
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
