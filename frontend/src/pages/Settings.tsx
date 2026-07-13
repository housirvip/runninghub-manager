import { useEffect, useState } from 'react'
import { settingsApi } from '@/lib/api'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { toast } from 'sonner'

export function SettingsPage() {
  const [strategy, setStrategy] = useState('')
  const [tickInput, setTickInput] = useState('')
  const [currentTick, setCurrentTick] = useState(0)
  const [pollInterval, setPollInterval] = useState('')
  const [pollMaxAttempts, setPollMaxAttempts] = useState('')

  useEffect(() => {
    const fetchSettings = async () => {
      try {
        const [strategyRes, tickRes, pollRes] = await Promise.all([
          settingsApi.getStrategy(),
          settingsApi.getTick(),
          settingsApi.getPoll(),
        ])
        setStrategy(strategyRes.data.data.strategy)
        setTickInput(String(tickRes.data.data.tickMs))
        setCurrentTick(tickRes.data.data.tickMs)
        setPollInterval(String(pollRes.data.data.pollInterval))
        setPollMaxAttempts(String(pollRes.data.data.pollMaxAttempts))
      } catch (e) {
        console.error('Failed to fetch settings', e)
      }
    }
    fetchSettings()
  }, [])

  const handleStrategyChange = async (newStrategy: string) => {
    try {
      await settingsApi.setStrategy(newStrategy)
      setStrategy(newStrategy)
      toast.success('调度策略已更新')
    } catch {
      toast.error('更新失败')
    }
  }

  const handleTickChange = async () => {
    const ms = parseInt(tickInput)
    if (isNaN(ms) || ms < 100 || ms > 60000) {
      toast.error('调度间隔需在 100~60000ms 之间')
      return
    }
    try {
      await settingsApi.setTick(ms)
      setCurrentTick(ms)
      toast.success('调度间隔已更新')
    } catch {
      toast.error('更新失败')
    }
  }

  const handlePollChange = async () => {
    const interval = parseInt(pollInterval)
    const attempts = parseInt(pollMaxAttempts)
    if (isNaN(interval) || interval < 1 || interval > 60) {
      toast.error('轮询间隔需在 1~60 秒之间')
      return
    }
    if (isNaN(attempts) || attempts < 1 || attempts > 10000) {
      toast.error('最大轮询次数需在 1~10000 之间')
      return
    }
    try {
      await settingsApi.setPoll({ pollInterval: interval, pollMaxAttempts: attempts })
      toast.success('轮询配置已更新')
    } catch {
      toast.error('更新失败')
    }
  }

  return (
    <div className="space-y-6">
      <h2 className="text-2xl font-bold">系统设置</h2>

      {/* Schedule Strategy */}
      <Card>
        <CardHeader>
          <CardTitle>调度策略</CardTitle>
          <CardDescription>
            控制任务分配到各个 Worker 的方式
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex items-center gap-3">
            <Button
              variant={strategy === 'least-loaded' ? 'default' : 'outline'}
              size="sm"
              onClick={() => handleStrategyChange('least-loaded')}
            >
              负载均衡
            </Button>
            <Button
              variant={strategy === 'fill-first' ? 'default' : 'outline'}
              size="sm"
              onClick={() => handleStrategyChange('fill-first')}
            >
              填满优先
            </Button>
          </div>
          <p className="text-sm text-muted-foreground">
            {strategy === 'least-loaded'
              ? '负载均衡：优先将任务分配给当前负载最低的 Worker，使各 Worker 负载均匀。'
              : '填满优先：优先填满一个 Worker 的并发后再分配到下一个，适合节省 API Key 额度。'}
          </p>
        </CardContent>
      </Card>

      {/* Scheduler Tick */}
      <Card>
        <CardHeader>
          <CardTitle>调度间隔</CardTitle>
          <CardDescription>
            调度器多久检查一次待分配的任务
          </CardDescription>
        </CardHeader>
        <CardContent>
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
            <span className="text-sm text-muted-foreground">毫秒</span>
            <Button size="sm" onClick={handleTickChange}>
              应用
            </Button>
          </div>
          <p className="text-sm text-muted-foreground mt-2">
            当前值: {currentTick}ms（范围 100~60000ms）。值越小调度越及时，但 CPU 开销越高。
          </p>
        </CardContent>
      </Card>

      {/* Poll Config */}
      <Card>
        <CardHeader>
          <CardTitle>任务轮询配置</CardTitle>
          <CardDescription>
            Worker 提交任务到 RunningHub 后，轮询结果的频率和次数上限
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="flex items-center gap-3 flex-wrap">
            <div className="flex items-center gap-2">
              <span className="text-sm text-muted-foreground">每</span>
              <Input
                type="number"
                min={1}
                max={60}
                value={pollInterval}
                onChange={(e) => setPollInterval(e.target.value)}
                className="w-20"
              />
              <span className="text-sm text-muted-foreground">秒查询一次</span>
            </div>
            <div className="flex items-center gap-2">
              <span className="text-sm text-muted-foreground">最多查询</span>
              <Input
                type="number"
                min={1}
                max={10000}
                value={pollMaxAttempts}
                onChange={(e) => setPollMaxAttempts(e.target.value)}
                className="w-24"
              />
              <span className="text-sm text-muted-foreground">次</span>
            </div>
            <Button size="sm" onClick={handlePollChange}>
              应用
            </Button>
          </div>
          {pollInterval && pollMaxAttempts && (
            <p className="text-sm text-muted-foreground mt-2">
              任务超时时间 ≈ {Math.round(parseInt(pollInterval) * parseInt(pollMaxAttempts) / 60)} 分钟
            </p>
          )}
        </CardContent>
      </Card>

      {/* System Info */}
      <Card>
        <CardHeader>
          <CardTitle>系统信息</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4 text-sm">
            <div className="flex justify-between border-b pb-2">
              <span className="text-muted-foreground">版本</span>
              <span className="font-mono">1.0.0</span>
            </div>
            <div className="flex justify-between border-b pb-2">
              <span className="text-muted-foreground">数据库</span>
              <span className="font-mono">SQLite</span>
            </div>
            <div className="flex justify-between border-b pb-2">
              <span className="text-muted-foreground">后端地址</span>
              <span className="font-mono">{window.location.origin}</span>
            </div>
            <div className="flex justify-between border-b pb-2">
              <span className="text-muted-foreground">前端框架</span>
              <span className="font-mono">React + Vite</span>
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
