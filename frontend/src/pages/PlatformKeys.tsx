import { useEffect, useState } from 'react'
import { platformKeyApi } from '@/lib/api'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { toast } from 'sonner'
import { Plus, Trash2, Copy, AlertTriangle, Eye } from 'lucide-react'
interface PlatformKeyItem {
  id: number
  name: string
  key: string
  isActive: boolean
  expiresAt?: string
  lastUsedAt?: string
  createdAt: string
}

export function PlatformKeysPage() {
  const [keys, setKeys] = useState<PlatformKeyItem[]>([])
  const [createDialogOpen, setCreateDialogOpen] = useState(false)
  const [showKeyDialog, setShowKeyDialog] = useState(false)
  const [newKey, setNewKey] = useState('')
  const [revealedKey, setRevealedKey] = useState('')
  const [revealDialogOpen, setRevealDialogOpen] = useState(false)
  const [form, setForm] = useState({ name: '', expiresAt: '' })

  const fetchKeys = async () => {
    try {
      const res = await platformKeyApi.list()
      setKeys(res.data.data || [])
    } catch (err) {
      console.error('Failed to fetch platform keys:', err)
    }
  }

  useEffect(() => {
    fetchKeys()
  }, [])

  const handleCreate = async (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault()
    try {
      const data: { name: string; expiresAt?: string } = { name: form.name }
      if (form.expiresAt) {
        data.expiresAt = new Date(form.expiresAt).toISOString()
      }
      const res = await platformKeyApi.create(data)
      const key = res.data.data.key
      setNewKey(key)
      setCreateDialogOpen(false)
      setShowKeyDialog(true)
      setForm({ name: '', expiresAt: '' })
      fetchKeys()
    } catch (err: unknown) {
      const error = err as { response?: { data?: { message?: string } } }
      toast.error(error.response?.data?.message || '创建失败')
    }
  }

  const handleDelete = async (key: PlatformKeyItem) => {
    if (!confirm(`确认删除密钥 "${key.name}" 吗？删除后使用此密钥的应用将无法访问。`)) return
    try {
      await platformKeyApi.delete(key.id)
      toast.success('已删除')
      fetchKeys()
    } catch (err: unknown) {
      const error = err as { response?: { data?: { message?: string } } }
      toast.error(error.response?.data?.message || '删除失败')
    }
  }

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text)
    toast.success('已复制到剪贴板')
  }

  const handleReveal = async (key: PlatformKeyItem) => {
    try {
      const res = await platformKeyApi.reveal(key.id)
      setRevealedKey(res.data.data.key)
      setRevealDialogOpen(true)
    } catch (err: unknown) {
      const error = err as { response?: { data?: { message?: string } } }
      toast.error(error.response?.data?.message || '查看密钥失败')
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-bold">平台密钥</h2>
          <p className="text-sm text-muted-foreground mt-1">
            用于外部应用直接调用平台 API，无需登录获取 JWT
          </p>
        </div>
        <Button onClick={() => setCreateDialogOpen(true)}>
          <Plus className="h-4 w-4 mr-2" />创建密钥
        </Button>
      </div>

      {/* Create dialog */}
      <Dialog open={createDialogOpen} onOpenChange={setCreateDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>创建平台密钥</DialogTitle>
          </DialogHeader>
          <form onSubmit={handleCreate} className="space-y-4">
            <div className="space-y-2">
              <Label>名称</Label>
              <Input
                value={form.name}
                onChange={(e: React.ChangeEvent<HTMLInputElement>) => setForm({ ...form, name: e.target.value })}
                placeholder="例如: 我的机器人"
                required
              />
            </div>
            <div className="space-y-2">
              <Label>过期时间（可选）</Label>
              <Input
                type="datetime-local"
                value={form.expiresAt}
                onChange={(e: React.ChangeEvent<HTMLInputElement>) => setForm({ ...form, expiresAt: e.target.value })}
              />
              <p className="text-xs text-muted-foreground">留空表示永不过期</p>
            </div>
            <Button type="submit" className="w-full">创建</Button>
          </form>
        </DialogContent>
      </Dialog>

      {/* Show new key dialog */}
      <Dialog open={showKeyDialog} onOpenChange={setShowKeyDialog}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>密钥已创建</DialogTitle>
          </DialogHeader>
          <div className="space-y-4">
            <div className="flex items-center gap-2 p-3 bg-yellow-50 border border-yellow-200 rounded text-sm">
              <AlertTriangle className="h-4 w-4 text-yellow-600 shrink-0" />
              <span className="text-yellow-800">请立即复制此密钥，关闭后将无法再次查看完整密钥。</span>
            </div>
            <div className="flex items-center gap-2">
              <Input value={newKey} readOnly className="font-mono text-sm" />
              <Button variant="outline" size="sm" onClick={() => copyToClipboard(newKey)}>
                <Copy className="h-4 w-4" />
              </Button>
            </div>
            <p className="text-xs text-muted-foreground">
              使用方式：在请求头中设置 <code className="bg-muted px-1 rounded">Authorization: Bearer {newKey.slice(0, 10)}...</code>
            </p>
          </div>
        </DialogContent>
      </Dialog>

      {/* Reveal key dialog */}
      <Dialog open={revealDialogOpen} onOpenChange={setRevealDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>查看密钥</DialogTitle>
          </DialogHeader>
          <div className="space-y-4">
            <div className="flex items-center gap-2">
              <Input value={revealedKey} readOnly className="font-mono text-sm" />
              <Button variant="outline" size="sm" onClick={() => copyToClipboard(revealedKey)}>
                <Copy className="h-4 w-4" />
              </Button>
            </div>
            <p className="text-xs text-muted-foreground">
              使用方式：<code className="bg-muted px-1 rounded">Authorization: Bearer {revealedKey}</code>
            </p>
          </div>
        </DialogContent>
      </Dialog>

      {/* Key list */}
      <div className="grid gap-4">
        {keys.length === 0 ? (
          <Card>
            <CardContent className="py-8 text-center text-muted-foreground">
              暂无平台密钥，点击"创建密钥"为外部应用生成访问令牌
            </CardContent>
          </Card>
        ) : (
          keys.map((key) => (
            <Card key={key.id}>
              <CardHeader className="pb-3">
                <div className="flex items-center justify-between">
                  <CardTitle className="text-base">{key.name}</CardTitle>
                  <div className="flex items-center gap-1">
                    <Badge variant={key.isActive ? 'default' : 'secondary'}>
                      {key.isActive ? '有效' : '已禁用'}
                    </Badge>
                    <Button variant="ghost" size="sm" onClick={() => handleReveal(key)} title="查看完整密钥">
                      <Eye className="h-4 w-4" />
                    </Button>
                    <Button variant="ghost" size="sm" onClick={async () => {
                      try {
                        const res = await platformKeyApi.reveal(key.id)
                        copyToClipboard(res.data.data.key)
                      } catch { toast.error('复制失败') }
                    }} title="复制密钥">
                      <Copy className="h-4 w-4" />
                    </Button>
                    <Button variant="ghost" size="sm" onClick={() => handleDelete(key)} title="删除">
                      <Trash2 className="h-4 w-4 text-destructive" />
                    </Button>
                  </div>
                </div>
              </CardHeader>
              <CardContent>
                <div className="flex flex-wrap items-center gap-4 text-sm">
                  <span className="text-muted-foreground font-mono">{key.key}</span>
                  <span className="text-muted-foreground">
                    创建: {new Date(key.createdAt).toLocaleDateString()}
                  </span>
                  {key.lastUsedAt && (
                    <span className="text-muted-foreground">
                      最后使用: {new Date(key.lastUsedAt).toLocaleString()}
                    </span>
                  )}
                  {key.expiresAt && (
                    <span className={`text-sm ${new Date(key.expiresAt) < new Date() ? 'text-destructive' : 'text-muted-foreground'}`}>
                      {new Date(key.expiresAt) < new Date() ? '已过期' : `过期: ${new Date(key.expiresAt).toLocaleDateString()}`}
                    </span>
                  )}
                </div>
              </CardContent>
            </Card>
          ))
        )}
      </div>
    </div>
  )
}
