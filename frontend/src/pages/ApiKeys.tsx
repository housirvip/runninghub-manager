import { useEffect, useState } from 'react'
import { apiKeyApi } from '@/lib/api'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Switch } from '@/components/ui/switch'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { toast } from 'sonner'
import { Plus, Pencil, Trash2 } from 'lucide-react'

interface ApiKeyItem {
  id: number
  name: string
  apiKey: string
  baseUrl: string
  maxConcurrency: number
  isActive: boolean
  currentTasks: number
  createdAt: string
}

const RH_DOMAINS = [
  { value: 'https://www.runninghub.cn', label: 'www.runninghub.cn (中文站)' },
  { value: 'https://www.runninghub.ai', label: 'www.runninghub.ai (国际站)' },
]

export function ApiKeysPage() {
  const [keys, setKeys] = useState<ApiKeyItem[]>([])
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editingKey, setEditingKey] = useState<ApiKeyItem | null>(null)
  const [form, setForm] = useState({ name: '', apiKey: '', baseUrl: 'https://www.runninghub.cn', maxConcurrency: 3 })

  const fetchKeys = async () => {
    try {
      const res = await apiKeyApi.list()
      setKeys(res.data.data)
    } catch (err) {
      console.error('Failed to fetch keys:', err)
    }
  }

  useEffect(() => {
    fetchKeys()
  }, [])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    try {
      if (editingKey) {
        await apiKeyApi.update(editingKey.id, {
          name: form.name,
          baseUrl: form.baseUrl,
          maxConcurrency: form.maxConcurrency,
        })
        toast.success('API Key 更新成功')
      } else {
        await apiKeyApi.create(form)
        toast.success('API Key 创建成功')
      }
      setDialogOpen(false)
      setEditingKey(null)
      setForm({ name: '', apiKey: '', baseUrl: 'https://www.runninghub.cn', maxConcurrency: 3 })
      fetchKeys()
    } catch (err: unknown) {
      const error = err as { response?: { data?: { message?: string } } }
      toast.error(error.response?.data?.message || '操作失败')
    }
  }

  const handleEdit = (key: ApiKeyItem) => {
    setEditingKey(key)
    setForm({ name: key.name, apiKey: key.apiKey, baseUrl: key.baseUrl || 'https://www.runninghub.cn', maxConcurrency: key.maxConcurrency })
    setDialogOpen(true)
  }

  const handleDelete = async (key: ApiKeyItem) => {
    if (!confirm(`确认删除 "${key.name}" 吗？`)) return
    try {
      await apiKeyApi.delete(key.id)
      toast.success('已删除')
      fetchKeys()
    } catch (err: unknown) {
      const error = err as { response?: { data?: { message?: string } } }
      toast.error(error.response?.data?.message || '删除失败')
    }
  }

  const handleToggle = async (key: ApiKeyItem) => {
    try {
      await apiKeyApi.update(key.id, { isActive: !key.isActive })
      toast.success(key.isActive ? '已禁用' : '已启用')
      fetchKeys()
    } catch (err: unknown) {
      const error = err as { response?: { data?: { message?: string } } }
      toast.error(error.response?.data?.message || '操作失败')
    }
  }

  const maskKey = (key: string) => {
    if (key.length <= 8) return '****'
    return key.slice(0, 4) + '****' + key.slice(-4)
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h2 className="text-2xl font-bold">RH API</h2>
        <Button onClick={() => setDialogOpen(true)}>
          <Plus className="h-4 w-4 mr-2" />添加 Key
        </Button>
      </div>

      <Dialog open={dialogOpen} onOpenChange={(open: boolean) => {
        setDialogOpen(open)
        if (!open) { setEditingKey(null); setForm({ name: '', apiKey: '', baseUrl: 'https://www.runninghub.cn', maxConcurrency: 3 }) }
      }}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{editingKey ? '编辑 API Key' : '添加 API Key'}</DialogTitle>
          </DialogHeader>
          <form onSubmit={handleSubmit} className="space-y-4">
            <div className="space-y-2">
              <Label>名称</Label>
              <Input
                value={form.name}
                onChange={(e: React.ChangeEvent<HTMLInputElement>) => setForm({ ...form, name: e.target.value })}
                placeholder="例如: 账号1-Key"
                required
              />
            </div>
            {!editingKey && (
              <div className="space-y-2">
                <Label>API Key</Label>
                <Input
                  value={form.apiKey}
                  onChange={(e: React.ChangeEvent<HTMLInputElement>) => setForm({ ...form, apiKey: e.target.value })}
                  placeholder="RunningHub API Key"
                  required
                />
              </div>
            )}
            <div className="space-y-2">
              <Label>站点</Label>
              <select
                value={form.baseUrl}
                onChange={(e) => setForm({ ...form, baseUrl: e.target.value })}
                className="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
              >
                {RH_DOMAINS.map((d) => (
                  <option key={d.value} value={d.value}>{d.label}</option>
                ))}
              </select>
            </div>
            <div className="space-y-2">
              <Label>最大并发数</Label>
              <Input
                type="number"
                min={1}
                max={10}
                value={form.maxConcurrency}
                onChange={(e: React.ChangeEvent<HTMLInputElement>) => setForm({ ...form, maxConcurrency: parseInt(e.target.value) || 3 })}
              />
            </div>
            <Button type="submit" className="w-full">
              {editingKey ? '保存' : '创建'}
            </Button>
          </form>
        </DialogContent>
      </Dialog>

      <div className="grid gap-4">
        {keys.length === 0 ? (
          <Card>
            <CardContent className="py-8 text-center text-muted-foreground">
              暂无 API Key，点击"添加 Key"开始配置
            </CardContent>
          </Card>
        ) : (
          keys.map((key) => (
            <Card key={key.id}>
              <CardHeader className="pb-3">
                <div className="flex items-center justify-between">
                  <CardTitle className="text-base">{key.name}</CardTitle>
                  <div className="flex items-center gap-2">
                    <Switch checked={key.isActive} onCheckedChange={() => handleToggle(key)} />
                    <Button variant="ghost" size="sm" onClick={() => handleEdit(key)}>
                      <Pencil className="h-4 w-4" />
                    </Button>
                    <Button variant="ghost" size="sm" onClick={() => handleDelete(key)}>
                      <Trash2 className="h-4 w-4 text-destructive" />
                    </Button>
                  </div>
                </div>
              </CardHeader>
              <CardContent>
                <div className="flex items-center gap-4 text-sm flex-wrap">
                  <span className="text-muted-foreground font-mono">{maskKey(key.apiKey)}</span>
                  <Badge variant="outline">
                    {key.baseUrl?.includes('runninghub.ai') ? '国际站' : '中文站'}
                  </Badge>
                  <Badge variant={key.isActive ? 'default' : 'secondary'}>
                    {key.isActive ? '启用' : '禁用'}
                  </Badge>
                  <span className="text-muted-foreground">
                    并发: {key.currentTasks}/{key.maxConcurrency}
                  </span>
                </div>
              </CardContent>
            </Card>
          ))
        )}
      </div>
    </div>
  )
}
