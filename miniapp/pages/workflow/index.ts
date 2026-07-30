import type { WorkflowRun } from '../../services/api'
import { presentStatus, presentWorkflowNode, type StatusTone } from '../../domain/presenters'
import { api } from '../../services/client'

let workflowTimer: number | undefined

Page({
  data: {
    projectID: '', loading: true, error: '', run: null as WorkflowRun | null,
    nodes: [] as Array<{ name: string; label: string; stateLabel: string; stateTone: StatusTone; active: boolean }>
  },
  onLoad(options: Record<string, string | undefined>) { this.setData({ projectID: options.projectID || '' }); void this.load() },
  onUnload() { if (workflowTimer !== undefined) clearTimeout(workflowTimer) },
  async load() {
    try {
      const run = await api.getWorkflow(this.data.projectID)
      const nodes = run.nodes.map((node) => {
        const status = presentStatus(node.state)
        return { name: node.name, label: presentWorkflowNode(node.name), stateLabel: status.label, stateTone: status.tone, active: node.state === 'running' }
      })
      this.setData({ run, nodes, loading: false, error: '' })
      if (run.state === 'queued' || run.state === 'running') workflowTimer = setTimeout(() => void this.load(), 2000) as unknown as number
    } catch (error) {
      this.setData({ loading: false, error: error instanceof Error ? error.message : '成书进度加载失败' })
    }
  },
  openResult() { void wx.redirectTo({ url: `/pages/result/index?id=${this.data.projectID}` }) }
})
