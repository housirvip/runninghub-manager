import { useEffect, useState } from 'react'
import { uploadsApi } from '@/lib/api'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { toast } from 'sonner'
import { Copy, ExternalLink, Search } from 'lucide-react'

interface UploadItem {
  id: number
  userId: number
  originalName: string
  fileName: string
  fileType: string
  fileSize: number
  url: string
  isLocal: boolean
  createdAt: string
}

interface UploadListResponse {
  items: UploadItem[]
  total: number
  page: number
  pageSize: number
}

function formatFileSize(bytes: number): string {
  if (bytes === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  return (bytes / Math.pow(1024, i)).toFixed(i > 0 ? 1 : 0) + ' ' + units[i]
}

export function UploadsPage() {
  const [data, setData] = useState<UploadListResponse | null>(null)
  const [page, setPage] = useState(1)
  const [search, setSearch] = useState('')
  const [searchInput, setSearchInput] = useState('')

  const fetchUploads = async () => {
    try {
      const res = await uploadsApi.list({ page, pageSize: 20, search: search || undefined })
      setData(res.data.data)
    } catch (err) {
      console.error('Failed to fetch uploads:', err)
    }
  }

  useEffect(() => {
    fetchUploads()
  }, [page, search])

  const handleSearch = () => {
    setSearch(searchInput)
    setPage(1)
  }

  const handleCopyFileName = (fileName: string) => {
    navigator.clipboard.writeText(fileName)
    toast.success('已复制 fileName')
  }

  const handleCopyUrl = (url: string) => {
    navigator.clipboard.writeText(url)
    toast.success('已复制 URL')
  }

  const totalPages = data ? Math.ceil(data.total / data.pageSize) : 0

  return (
    <div className="space-y-6">
      <h2 className="text-2xl font-bold">文件上传记录</h2>

      {/* Search */}
      <div className="flex gap-2">
        <Input
          placeholder="按原始文件名搜索..."
          value={searchInput}
          onChange={(e) => setSearchInput(e.target.value)}
          onKeyDown={(e) => e.key === 'Enter' && handleSearch()}
          className="max-w-sm"
        />
        <Button variant="outline" size="sm" onClick={handleSearch}>
          <Search className="h-4 w-4 mr-1" />
          搜索
        </Button>
        {search && (
          <Button variant="ghost" size="sm" onClick={() => { setSearch(''); setSearchInput(''); setPage(1) }}>
            清除
          </Button>
        )}
      </div>

      {/* Upload list */}
      <div className="space-y-3">
        {!data || data.items.length === 0 ? (
          <Card>
            <CardContent className="py-8 text-center text-muted-foreground">
              暂无上传记录
            </CardContent>
          </Card>
        ) : (
          data.items.map((item) => (
            <Card key={item.id}>
              <CardContent className="py-4">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-3 min-w-0 flex-1">
                    <span className="font-mono text-xs text-muted-foreground">#{item.id}</span>
                    <span className="font-medium text-sm truncate" title={item.originalName}>
                      {item.originalName}
                    </span>
                    <Badge variant={item.isLocal ? 'outline' : 'secondary'}>
                      {item.isLocal ? '本地' : 'RunningHub'}
                    </Badge>
                    <Badge variant="outline">{item.fileType}</Badge>
                    <span className="text-xs text-muted-foreground">{formatFileSize(item.fileSize)}</span>
                  </div>
                  <div className="flex items-center gap-2 shrink-0">
                    <span className="text-xs text-muted-foreground">
                      {new Date(item.createdAt).toLocaleString()}
                    </span>
                  </div>
                </div>
                <div className="mt-2 flex items-center gap-2">
                  <code className="text-xs bg-muted px-2 py-1 rounded truncate max-w-md" title={item.fileName}>
                    {item.fileName}
                  </code>
                  <Button
                    variant="ghost"
                    size="sm"
                    className="h-6 w-6 p-0"
                    onClick={() => handleCopyFileName(item.fileName)}
                    title="复制 fileName"
                  >
                    <Copy className="h-3 w-3" />
                  </Button>
                  <Button
                    variant="ghost"
                    size="sm"
                    className="h-6 w-6 p-0"
                    onClick={() => handleCopyUrl(item.url)}
                    title="复制 URL"
                  >
                    <ExternalLink className="h-3 w-3" />
                  </Button>
                </div>
              </CardContent>
            </Card>
          ))
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
    </div>
  )
}
