import { useEffect, useState } from 'react'
import { settingsApi } from '@/lib/api'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { toast } from 'sonner'
import { Save } from 'lucide-react'

export function SettingsPage() {
  const [strategy, setStrategy] = useState('')
  const [tickMs, setTickMs] = useState('')
  const [pollInterval, setPollInterval] = useState('')
  const [pollMaxAttempts, setPollMaxAttempts] = useState('')
  const [localTaskTimeout, setLocalTaskTimeout] = useState('')
  const [saving, setSaving] = useState(false)
  const [dirty, setDirty] = useState(false)

  // Snapshot of last-saved values to detect changes
  const [saved, setSaved] = useState({
    strategy: '', tickMs: '', pollInterval: '', pollMaxAttempts: '', localTaskTimeout: '',
  })

  useEffect(() => {
    const fetchSettings = async () => {
      try {
        const res = await settingsApi.getAll()
        const d = res.data.data
        const values = {
          strategy: d.strategy,
          tickMs: String(d.tickMs),
          pollInterval: String(d.pollInterval),
          pollMaxAttempts: String(d.pollMaxAttempts),
          localTaskTimeout: String(d.localTaskTimeout),
        }
        setStrategy(values.strategy)
        setTickMs(values.tickMs)
        setPollInterval(values.pollInterval)
        setPollMaxAttempts(values.pollMaxAttempts)
        setLocalTaskTimeout(values.localTaskTimeout)
        setSaved(values)
        setDirty(false)
      } catch (e) {
        console.error('Failed to fetch settings', e)
      }
    }
    fetchSettings()
  }, [])

  // Track dirty state
  useEffect(() => {
    const current = { strategy, tickMs, pollInterval, pollMaxAttempts, localTaskTimeout }
    const changed = Object.keys(current).some(
      (k) => current[k as keyof typeof current] !== saved[k as keyof typeof saved]
    )
    setDirty(changed)
  }, [strategy, tickMs, pollInterval, pollMaxAttempts, localTaskTimeout, saved])

  const handleSave = async () => {
    // Validate
    const tick = parseInt(tickMs)
    if (isNaN(tick) || tick < 100 || tick > 60000) {
      toast.error('调度间隔需在 100~60000ms 之间')
      return
    }
    const interval = parseInt(pollInterval)
    if (isNaN(interval) || interval < 1 || interval > 60) {
      toast.error('轮询间隔需在 1~60 秒之间')
      return
    }
    const attempts = parseInt(pollMaxAttempts)
    if (isNaN(attempts) || attempts < 1 || attempts > 10000) {
      toast.error('最大轮询次数需在 1~10000 之间')
      return
    }
    const timeout = parseInt(localTaskTimeout)
    if (isNaN(timeout) || timeout < 1 || timeout > 1440) {
      toast.error('本地任务超时需在 1~1440 分钟之间')
      return
    }

    setSaving(true)
    try {
      await settingsApi.saveAll({
        strategy,
        tickMs: tick,
        pollInterval: interval,
        pollMaxAttempts: attempts,
        localTaskTimeout: timeout,
      })
      const newSaved = { strategy, tickMs, pollInterval, pollMaxAttempts, localTaskTimeout }
      setSaved(newSaved)
      setDirty(false)
      toast.success('设置已保存')
    } catch {
      toast.error('保存失败')
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h2 className="text-2xl font-bold">系统设置</h2>
        <Button onClick={handleSave} disabled={!dirty || saving}>
          <Save className="h-4 w-4 mr-2" />
          {saving ? '保存中...' : '保存设置'}
        </Button>
      </div>

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
              onClick={() => setStrategy('least-loaded')}
            >
              负载均衡
            </Button>
            <Button
              variant={strategy === 'fill-first' ? 'default' : 'outline'}
              size="sm"
              onClick={() => setStrategy('fill-first')}
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
              value={tickMs}
              onChange={(e) => setTickMs(e.target.value)}
              className="w-32"
            />
            <span className="text-sm text-muted-foreground">毫秒</span>
          </div>
          <p className="text-sm text-muted-foreground mt-2">
            范围 100~60000ms。值越小调度越及时，但 CPU 开销越高。
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
        <CardContent className="space-y-3">
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
          </div>
          {pollInterval && pollMaxAttempts && (
            <p className="text-sm text-muted-foreground">
              远程任务超时 ≈ {Math.round(parseInt(pollInterval) * parseInt(pollMaxAttempts) / 60)} 分钟
            </p>
          )}
        </CardContent>
      </Card>

      {/* Local Task Timeout */}
      <Card>
        <CardHeader>
          <CardTitle>本地任务超时</CardTitle>
          <CardDescription>
            自定义 App 执行的最大允许时间，超时后任务将被强制终止
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="flex items-center gap-3">
            <Input
              type="number"
              min={1}
              max={1440}
              value={localTaskTimeout}
              onChange={(e) => setLocalTaskTimeout(e.target.value)}
              className="w-24"
            />
            <span className="text-sm text-muted-foreground">分钟</span>
          </div>
          <p className="text-sm text-muted-foreground mt-2">
            范围 1~1440 分钟（最大 24 小时）
          </p>
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
