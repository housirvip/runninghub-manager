import { useEffect, useState } from 'react'
import { taskApi } from '@/lib/api'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent } from '@/components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { toast } from 'sonner'

interface TaskItem {
  id: number
  webappId: string
  status: string
  isLocal?: boolean
  apiKeyName?: string
  nodeInfoList: string
  results?: string
  errorMessage?: string
  rhTaskId?: string
  createdAt: string
  dispatchedAt?: string
  completedAt?: string
}

interface TaskListResponse {
  items: TaskItem[]
  total: number
  page: number
  pageSize: number
}

const statusConfig: Record<string, { label: string; variant: 'default' | 'secondary' | 'destructive' | 'outline' }> = {
  PENDING: { label: '等待中', variant: 'secondary' },
  DISPATCHED: { label: '已分配', variant: 'outline' },
  RUNNING: { label: '运行中', variant: 'default' },
  SUCCESS: { label: '成功', variant: 'default' },
  FAILED: { label: '失败', variant: 'destructive' },
  CANCELLED: { label: '已取消', variant: 'secondary' },
  QUEUED: { label: '排队中', variant: 'outline' },
}

const tabs = ['', 'PENDING', 'RUNNING', 'SUCCESS', 'FAILED', 'CANCELLED']
const tabLabels = ['全部', '等待中', '运行中', '成功', '失败', '已取消']

export function TasksPage() {
  const [tasks, setTasks] = useState<TaskListResponse | null>(null)
  const [status, setStatus] = useState('')
  const [page, setPage] = useState(1)
  const [selectedTask, setSelectedTask] = useState<TaskItem | null>(null)

  const fetchTasks = async () => {
    try {
      const res = await taskApi.list({ page, pageSize: 20, status: status || undefined })
      setTasks(res.data.data)
    } catch (err) {
      console.error('Failed to fetch tasks:', err)
    }
  }

  useEffect(() => {
    fetchTasks()
    const interval = setInterval(fetchTasks, 5000)
    return () => clearInterval(interval)
  }, [page, status])

  const handleCancel = async (taskId: number) => {
    if (!confirm('确认取消此任务？')) return
    try {
      await taskApi.cancel(taskId)
      toast.success('任务已取消')
      fetchTasks()
    } catch (err: unknown) {
      const error = err as { response?: { data?: { message?: string } } }
      toast.error(error.response?.data?.message || '取消失败')
    }
  }

  const totalPages = tasks ? Math.ceil(tasks.total / tasks.pageSize) : 0

  return (
    <div className="space-y-6">
      <h2 className="text-2xl font-bold">任务列表</h2>

      {/* Status tabs */}
      <div className="flex gap-2 flex-wrap">
        {tabs.map((tab, i) => (
          <Button
            key={tab || 'all'}
            variant={status === tab ? 'default' : 'outline'}
            size="sm"
            onClick={() => { setStatus(tab); setPage(1) }}
          >
            {tabLabels[i]}
          </Button>
        ))}
      </div>

      {/* Task list */}
      <div className="space-y-3">
        {!tasks || tasks.items.length === 0 ? (
          <Card>
            <CardContent className="py-8 text-center text-muted-foreground">
              暂无任务
            </CardContent>
          </Card>
        ) : (
          tasks.items.map((task) => {
            const cfg = statusConfig[task.status] || { label: task.status, variant: 'secondary' as const }
            return (
              <Card
                key={task.id}
                className="cursor-pointer hover:border-primary/50 transition-colors"
                onClick={() => setSelectedTask(task)}
              >
                <CardContent className="py-4">
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-3">
                      <span className="font-mono text-sm">#{task.id}</span>
                      <Badge variant={cfg.variant}>{cfg.label}</Badge>
                      {task.isLocal ? (
                        <Badge variant="outline" className="bg-green-50 text-green-700 border-green-300">本地应用</Badge>
                      ) : task.apiKeyName ? (
                        <span className="text-xs text-muted-foreground">{task.apiKeyName}</span>
                      ) : null}
                    </div>
                    <div className="flex items-center gap-3">
                      <span className="text-xs text-muted-foreground">
                        {new Date(task.createdAt).toLocaleString()}
                      </span>
                      {(['PENDING', 'RUNNING', 'DISPATCHED', 'QUEUED'].includes(task.status)) && (
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={(e: React.MouseEvent) => { e.stopPropagation(); handleCancel(task.id) }}
                        >
                          取消
                        </Button>
                      )}
                    </div>
                  </div>
                  <div className="mt-2 text-sm text-muted-foreground">
                    WebApp ID: {task.webappId}
                  </div>
                </CardContent>
              </Card>
            )
          })
        )}
      </div>

      {/* Pagination */}
      {totalPages > 1 && (
        <div className="flex items-center justify-center gap-2">
          <Button
            variant="outline"
            size="sm"
            disabled={page <= 1}
            onClick={() => setPage(page - 1)}
          >
            上一页
          </Button>
          <span className="text-sm text-muted-foreground">
            {page} / {totalPages}
          </span>
          <Button
            variant="outline"
            size="sm"
            disabled={page >= totalPages}
            onClick={() => setPage(page + 1)}
          >
            下一页
          </Button>
        </div>
      )}

      {/* Task detail dialog */}
      <Dialog open={!!selectedTask} onOpenChange={(open: boolean) => { if (!open) setSelectedTask(null) }}>
        <DialogContent className="max-w-2xl max-h-[80vh] overflow-auto">
          <DialogHeader>
            <DialogTitle>任务详情</DialogTitle>
          </DialogHeader>
          {selectedTask && (
            <div className="space-y-4 text-sm">
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <span className="text-muted-foreground">任务 ID:</span>
                  <p className="font-mono break-all">{selectedTask.id}</p>
                </div>
                <div>
                  <span className="text-muted-foreground">状态:</span>
                  <p><Badge variant={statusConfig[selectedTask.status]?.variant || 'secondary'}>
                    {statusConfig[selectedTask.status]?.label || selectedTask.status}
                  </Badge></p>
                </div>
                <div>
                  <span className="text-muted-foreground">WebApp ID:</span>
                  <p className="font-mono">{selectedTask.webappId}</p>
                </div>
                <div>
                  <span className="text-muted-foreground">执行方式:</span>
                  <p>{selectedTask.isLocal ? '本地应用' : (selectedTask.apiKeyName || '-')}</p>
                </div>
                {selectedTask.rhTaskId && (
                  <div>
                    <span className="text-muted-foreground">RunningHub Task ID:</span>
                    <p className="font-mono">{selectedTask.rhTaskId}</p>
                  </div>
                )}
                <div>
                  <span className="text-muted-foreground">创建时间:</span>
                  <p>{new Date(selectedTask.createdAt).toLocaleString()}</p>
                </div>
                {selectedTask.dispatchedAt && (
                  <div>
                    <span className="text-muted-foreground">调度时间:</span>
                    <p>{new Date(selectedTask.dispatchedAt).toLocaleString()}</p>
                  </div>
                )}
                {selectedTask.completedAt && (
                  <div>
                    <span className="text-muted-foreground">完成时间:</span>
                    <p>{new Date(selectedTask.completedAt).toLocaleString()}</p>
                  </div>
                )}
              </div>

              {selectedTask.errorMessage && (
                <div>
                  <span className="text-muted-foreground">错误信息:</span>
                  <p className="text-destructive mt-1 p-2 bg-destructive/10 rounded">
                    {selectedTask.errorMessage}
                  </p>
                </div>
              )}

              {selectedTask.results && (
                <div>
                  <span className="text-muted-foreground">结果:</span>
                  <div className="mt-1 p-2 bg-muted rounded">
                    {(() => {
                      try {
                        const results = JSON.parse(selectedTask.results!)
                        if (Array.isArray(results)) {
                          return results.map((r: { url?: string }, i: number) => (
                            <div key={i} className="break-all">
                              {r.url ? (
                                <a href={r.url} target="_blank" rel="noreferrer" className="text-blue-600 underline">
                                  {r.url}
                                </a>
                              ) : (
                                JSON.stringify(r)
                              )}
                            </div>
                          ))
                        }
                        return <pre className="whitespace-pre-wrap">{JSON.stringify(results, null, 2)}</pre>
                      } catch {
                        return <pre className="whitespace-pre-wrap">{selectedTask.results}</pre>
                      }
                    })()}
                  </div>
                </div>
              )}

              <div>
                <span className="text-muted-foreground">NodeInfoList:</span>
                <pre className="mt-1 p-2 bg-muted rounded text-xs overflow-auto max-h-48 whitespace-pre-wrap">
                  {(() => {
                    try {
                      return JSON.stringify(JSON.parse(selectedTask.nodeInfoList), null, 2)
                    } catch {
                      return selectedTask.nodeInfoList
                    }
                  })()}
                </pre>
              </div>
            </div>
          )}
        </DialogContent>
      </Dialog>
    </div>
  )
}
