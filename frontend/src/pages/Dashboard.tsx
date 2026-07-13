import { useEffect, useState } from 'react'
import { dashboardApi, settingsApi } from '@/lib/api'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { toast } from 'sonner'

interface Stats {
  totalTasks: number
  pendingTasks: number
  runningTasks: number
  successTasks: number
  failedTasks: number
  totalKeys: number
  activeKeys: number
  scheduleStrategy: string
  schedulerTick: number
  workerStatus: Array<{
    apiKeyId: number
    name: string
    maxConcurrency: number
    currentTasks: number
    available: boolean
  }>
}

export function DashboardPage() {
  const [stats, setStats] = useState<Stats | null>(null)
  const [tickInput, setTickInput] = useState('')
  const [pollInterval, setPollInterval] = useState('')
  const [pollMaxAttempts, setPollMaxAttempts] = useState('')

  const fetchStats = async () => {
    try {
      const res = await dashboardApi.stats()
      setStats(res.data.data)
      if (!tickInput) {
        setTickInput(String(res.data.data.schedulerTick))
      }
    } catch (err) {
      console.error('Failed to fetch stats:', err)
    }
  }

  const fetchPollConfig = async () => {
    try {
      const res = await settingsApi.getPoll()
      setPollInterval(String(res.data.data.pollInterval))
      setPollMaxAttempts(String(res.data.data.pollMaxAttempts))
    } catch (err) {
      console.error('Failed to fetch poll config:', err)
    }
  }

  useEffect(() => {
    fetchStats()
    fetchPollConfig()
    const interval = setInterval(fetchStats, 5000)
    return () => clearInterval(interval)
  }, [])

  const handleStrategyChange = async (strategy: string) => {
    try {
      await settingsApi.setStrategy(strategy)
      toast.success(`调度策略已切换为: ${strategy === 'least-loaded' ? '负载均衡' : '填满优先'}`)
      fetchStats()
    } catch {
      toast.error('切换策略失败')
    }
  }

  const handleTickChange = async () => {
    const ms = parseInt(tickInput)
    if (isNaN(ms) || ms < 100 || ms > 60000) {
      toast.error('调度间隔需在 100ms ~ 60000ms 之间')
      return
    }
    try {
      await settingsApi.setTick(ms)
      toast.success(`调度间隔已设置为 ${ms}ms`)
      fetchStats()
    } catch {
      toast.error('设置调度间隔失败')
    }
  }

  const handlePollChange = async () => {
    const interval = parseInt(pollInterval)
    const maxAttempts = parseInt(pollMaxAttempts)
    if (isNaN(interval) || interval < 1 || interval > 60) {
      toast.error('轮询间隔需在 1~60 秒之间')
      return
    }
    if (isNaN(maxAttempts) || maxAttempts < 1 || maxAttempts > 10000) {
      toast.error('最大查询次数需在 1~10000 之间')
      return
    }
    try {
      await settingsApi.setPoll({ pollInterval: interval, pollMaxAttempts: maxAttempts })
      toast.success(`轮询配置已更新：每 ${interval}s 查询一次，最多 ${maxAttempts} 次`)
    } catch {
      toast.error('设置轮询配置失败')
    }
  }

  if (!stats) {
    return <div className="text-muted-foreground">加载中...</div>
  }

  return (
    <div className="space-y-6">
      <h2 className="text-2xl font-bold">仪表盘</h2>

      {/* Stats cards */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-5 gap-4">
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">总任务</CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-2xl font-bold">{stats.totalTasks}</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">等待中</CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-2xl font-bold text-yellow-600">{stats.pendingTasks}</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">运行中</CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-2xl font-bold text-blue-600">{stats.runningTasks}</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">已完成</CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-2xl font-bold text-green-600">{stats.successTasks}</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">失败</CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-2xl font-bold text-red-600">{stats.failedTasks}</p>
          </CardContent>
        </Card>
      </div>

      {/* Scheduler Settings */}
      <Card>
        <CardHeader>
          <CardTitle>调度设置</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          {/* Strategy */}
          <div>
            <p className="text-sm font-medium mb-2">调度策略</p>
            <div className="flex items-center gap-3">
              <Button
                variant={stats.scheduleStrategy === 'least-loaded' ? 'default' : 'outline'}
                size="sm"
                onClick={() => handleStrategyChange('least-loaded')}
              >
                负载均衡
              </Button>
              <Button
                variant={stats.scheduleStrategy === 'fill-first' ? 'default' : 'outline'}
                size="sm"
                onClick={() => handleStrategyChange('fill-first')}
              >
                填满优先
              </Button>
              <span className="text-sm text-muted-foreground ml-2">
                {stats.scheduleStrategy === 'least-loaded'
                  ? '优先分配给负载最低的 Worker'
                  : '优先填满一个 Worker 再用下一个'}
              </span>
            </div>
          </div>
          {/* Tick interval */}
          <div>
            <p className="text-sm font-medium mb-2">调度间隔</p>
            <div className="flex items-center gap-3">
              <Input
                type="number"
                min={100}
                max={60000}
                step={100}
                value={tickInput}
                onChange={(e) => setTickInput(e.target.value)}
                className="w-32"
              />
              <span className="text-sm text-muted-foreground">ms</span>
              <Button size="sm" onClick={handleTickChange}>
                应用
              </Button>
              <span className="text-sm text-muted-foreground ml-2">
                当前每 {stats.schedulerTick}ms 检查一次待分配任务（范围 100~60000）
              </span>
            </div>
          </div>
          {/* Poll config */}
          <div>
            <p className="text-sm font-medium mb-2">任务轮询配置</p>
            <div className="flex items-center gap-3 flex-wrap">
              <span className="text-sm text-muted-foreground">每</span>
              <Input
                type="number"
                min={1}
                max={60}
                value={pollInterval}
                onChange={(e) => setPollInterval(e.target.value)}
                className="w-20"
              />
              <span className="text-sm text-muted-foreground">秒查询一次，最多查询</span>
              <Input
                type="number"
                min={1}
                max={10000}
                value={pollMaxAttempts}
                onChange={(e) => setPollMaxAttempts(e.target.value)}
                className="w-24"
              />
              <span className="text-sm text-muted-foreground">次</span>
              <Button size="sm" onClick={handlePollChange}>
                应用
              </Button>
              {pollInterval && pollMaxAttempts && (
                <span className="text-sm text-muted-foreground ml-2">
                  超时时间 ≈ {Math.round(parseInt(pollInterval) * parseInt(pollMaxAttempts) / 60)} 分钟
                </span>
              )}
            </div>
          </div>
        </CardContent>
      </Card>

      {/* API Keys status */}
      <Card>
        <CardHeader>
          <CardTitle>Worker 状态</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="text-sm text-muted-foreground mb-4">
            活跃 Keys: {stats.activeKeys} / {stats.totalKeys}
          </div>
          {stats.workerStatus.length === 0 ? (
            <p className="text-muted-foreground text-sm">暂无活跃的 Worker</p>
          ) : (
            <div className="space-y-3">
              {stats.workerStatus.map((worker) => (
                <div
                  key={worker.apiKeyId}
                  className="flex items-center justify-between p-3 border rounded-md"
                >
                  <div className="flex items-center gap-3">
                    <span className="font-medium">{worker.name}</span>
                    <Badge variant={worker.available ? 'default' : 'secondary'}>
                      {worker.currentTasks} / {worker.maxConcurrency}
                    </Badge>
                  </div>
                  <Badge variant={worker.available ? 'outline' : 'destructive'}>
                    {worker.available ? '空闲' : '满载'}
                  </Badge>
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
