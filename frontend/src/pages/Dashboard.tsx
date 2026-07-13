import { useEffect, useState } from 'react'
import { dashboardApi } from '@/lib/api'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  LineChart,
  Line,
  AreaChart,
  Area,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  Legend,
} from 'recharts'

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
    currentTasks: number
    maxConcurrency: number
    available: boolean
  }>
}

interface ChartData {
  taskTrend: Array<{ date: string; total: number; success: number; failed: number }>
  apiCallTrend: Array<{ date: string; total: number; proxy: number; management: number }>
  hourlyToday: Array<{ hour: string; tasks: number; apiCalls: number }>
}

interface LogEntry {
  id: number
  method: string
  path: string
  statusCode: number
  latency: number
  clientIP: string
  username: string
  createdAt: string
}

export function DashboardPage() {
  const [stats, setStats] = useState<Stats | null>(null)
  const [chartData, setChartData] = useState<ChartData | null>(null)
  const [logs, setLogs] = useState<LogEntry[]>([])

  useEffect(() => {
    const fetchAll = async () => {
      try {
        const [statsRes, chartsRes, logsRes] = await Promise.all([
          dashboardApi.stats(),
          dashboardApi.charts(7),
          dashboardApi.logs({ page: 1, pageSize: 20 }),
        ])
        setStats(statsRes.data.data)
        setChartData(chartsRes.data.data)
        setLogs(logsRes.data.data.logs || [])
      } catch (e) {
        console.error('Failed to fetch dashboard data', e)
      }
    }
    fetchAll()
    const interval = setInterval(fetchAll, 30000)
    return () => clearInterval(interval)
  }, [])

  if (!stats) {
    return <div className="text-muted-foreground">加载中...</div>
  }

  return (
    <div className="space-y-6">
      <h2 className="text-2xl font-bold">仪表盘</h2>

      {/* Stats cards */}
      <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-6 gap-4">
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
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">活跃 Keys</CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-2xl font-bold">{stats.activeKeys} / {stats.totalKeys}</p>
          </CardContent>
        </Card>
      </div>

      {/* Charts row */}
      {chartData && (
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
          {/* Task trend chart */}
          <Card>
            <CardHeader>
              <CardTitle className="text-base">任务趋势（近7天）</CardTitle>
            </CardHeader>
            <CardContent>
              <ResponsiveContainer width="100%" height={240}>
                <LineChart data={chartData.taskTrend}>
                  <CartesianGrid strokeDasharray="3 3" className="stroke-muted" />
                  <XAxis
                    dataKey="date"
                    tick={{ fontSize: 12 }}
                    tickFormatter={(v) => v.slice(5)}
                  />
                  <YAxis tick={{ fontSize: 12 }} allowDecimals={false} />
                  <Tooltip />
                  <Legend />
                  <Line
                    type="monotone"
                    dataKey="total"
                    name="总计"
                    stroke="hsl(var(--primary))"
                    strokeWidth={2}
                    dot={false}
                  />
                  <Line
                    type="monotone"
                    dataKey="success"
                    name="成功"
                    stroke="hsl(142, 76%, 36%)"
                    strokeWidth={2}
                    dot={false}
                  />
                  <Line
                    type="monotone"
                    dataKey="failed"
                    name="失败"
                    stroke="hsl(var(--destructive))"
                    strokeWidth={2}
                    dot={false}
                  />
                </LineChart>
              </ResponsiveContainer>
            </CardContent>
          </Card>

          {/* API call trend chart */}
          <Card>
            <CardHeader>
              <CardTitle className="text-base">API 调用量（近7天）</CardTitle>
            </CardHeader>
            <CardContent>
              <ResponsiveContainer width="100%" height={240}>
                <LineChart data={chartData.apiCallTrend}>
                  <CartesianGrid strokeDasharray="3 3" className="stroke-muted" />
                  <XAxis
                    dataKey="date"
                    tick={{ fontSize: 12 }}
                    tickFormatter={(v) => v.slice(5)}
                  />
                  <YAxis tick={{ fontSize: 12 }} allowDecimals={false} />
                  <Tooltip />
                  <Legend />
                  <Line
                    type="monotone"
                    dataKey="total"
                    name="总计"
                    stroke="hsl(var(--primary))"
                    strokeWidth={2}
                    dot={false}
                  />
                  <Line
                    type="monotone"
                    dataKey="proxy"
                    name="代理"
                    stroke="hsl(221, 83%, 53%)"
                    strokeWidth={2}
                    dot={false}
                  />
                  <Line
                    type="monotone"
                    dataKey="management"
                    name="管理"
                    stroke="hsl(262, 83%, 58%)"
                    strokeWidth={2}
                    dot={false}
                  />
                </LineChart>
              </ResponsiveContainer>
            </CardContent>
          </Card>
        </div>
      )}

      {/* Hourly chart for today */}
      {chartData && (
        <Card>
          <CardHeader>
            <CardTitle className="text-base">今日调用量（按小时）</CardTitle>
          </CardHeader>
          <CardContent>
            <ResponsiveContainer width="100%" height={200}>
              <AreaChart data={chartData.hourlyToday}>
                <CartesianGrid strokeDasharray="3 3" className="stroke-muted" />
                <XAxis dataKey="hour" tick={{ fontSize: 11 }} />
                <YAxis tick={{ fontSize: 12 }} allowDecimals={false} />
                <Tooltip />
                <Legend />
                <Area
                  type="monotone"
                  dataKey="apiCalls"
                  name="API 调用"
                  stroke="hsl(var(--primary))"
                  fill="hsl(var(--primary) / 0.1)"
                  strokeWidth={2}
                />
                <Area
                  type="monotone"
                  dataKey="tasks"
                  name="任务"
                  stroke="hsl(142, 76%, 36%)"
                  fill="hsl(142, 76%, 36%, 0.1)"
                  strokeWidth={2}
                />
              </AreaChart>
            </ResponsiveContainer>
          </CardContent>
        </Card>
      )}

      {/* Worker status */}
      <Card>
        <CardHeader>
          <CardTitle className="text-base">Worker 状态</CardTitle>
        </CardHeader>
        <CardContent>
          {stats.workerStatus.length === 0 ? (
            <p className="text-muted-foreground text-sm">暂无活跃的 Worker</p>
          ) : (
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">
              {stats.workerStatus.map((worker) => (
                <div
                  key={worker.apiKeyId}
                  className="flex items-center justify-between p-3 border rounded-md"
                >
                  <div className="flex items-center gap-3">
                    <span className="font-medium text-sm">{worker.name}</span>
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

      {/* Recent request logs */}
      <Card>
        <CardHeader>
          <CardTitle className="text-base">最近请求日志</CardTitle>
        </CardHeader>
        <CardContent>
          {logs.length === 0 ? (
            <p className="text-muted-foreground text-sm">暂无请求记录</p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="w-[140px]">时间</TableHead>
                  <TableHead className="w-[70px]">方法</TableHead>
                  <TableHead>路径</TableHead>
                  <TableHead className="w-[70px]">状态</TableHead>
                  <TableHead className="w-[80px]">耗时</TableHead>
                  <TableHead className="w-[80px]">用户</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {logs.map((log) => (
                  <TableRow key={log.id}>
                    <TableCell className="text-xs text-muted-foreground">
                      {new Date(log.createdAt).toLocaleString('zh-CN', {
                        month: '2-digit',
                        day: '2-digit',
                        hour: '2-digit',
                        minute: '2-digit',
                        second: '2-digit',
                      })}
                    </TableCell>
                    <TableCell>
                      <Badge variant="outline" className="font-mono text-xs">
                        {log.method}
                      </Badge>
                    </TableCell>
                    <TableCell className="font-mono text-xs max-w-[300px] truncate">
                      {log.path}
                    </TableCell>
                    <TableCell>
                      <Badge
                        variant={
                          log.statusCode < 300
                            ? 'default'
                            : log.statusCode < 500
                              ? 'secondary'
                              : 'destructive'
                        }
                      >
                        {log.statusCode}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-xs text-muted-foreground">
                      {log.latency}ms
                    </TableCell>
                    <TableCell className="text-xs text-muted-foreground">
                      {log.username || '-'}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
