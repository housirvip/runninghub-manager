import axios from 'axios'

const api = axios.create({
  baseURL: '',
})

api.interceptors.request.use((config) => {
  const token = localStorage.getItem('token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

api.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      localStorage.removeItem('token')
      localStorage.removeItem('user')
      window.location.href = '/login'
    }
    return Promise.reject(error)
  }
)

export default api

// Auth
export const authApi = {
  login: (username: string, password: string) =>
    api.post('/api/auth/login', { username, password }),
  register: (username: string, password: string) =>
    api.post('/api/auth/register', { username, password }),
}

// API Keys
export const apiKeyApi = {
  list: () => api.get('/api/apikeys'),
  create: (data: { name: string; apiKey: string; baseUrl: string; maxConcurrency: number }) =>
    api.post('/api/apikeys', data),
  update: (id: number, data: { name?: string; baseUrl?: string; maxConcurrency?: number; isActive?: boolean }) =>
    api.put(`/api/apikeys/${id}`, data),
  delete: (id: number) => api.delete(`/api/apikeys/${id}`),
}

// Tasks
export const taskApi = {
  list: (params: { page?: number; pageSize?: number; status?: string }) =>
    api.get('/api/tasks', { params }),
  get: (id: number | string) => api.get(`/api/tasks/${id}`),
  cancel: (id: number | string) => api.post(`/api/tasks/${id}/cancel`),
}

// Dashboard
export const dashboardApi = {
  stats: () => api.get('/api/dashboard/stats'),
}

// Settings
export const settingsApi = {
  getStrategy: () => api.get('/api/settings/strategy'),
  setStrategy: (strategy: string) => api.put('/api/settings/strategy', { strategy }),
  getTick: () => api.get('/api/settings/tick'),
  setTick: (tickMs: number) => api.put('/api/settings/tick', { tickMs }),
  getPoll: () => api.get('/api/settings/poll'),
  setPoll: (data: { pollInterval?: number; pollMaxAttempts?: number }) => api.put('/api/settings/poll', data),
}


// Custom Apps
export const appsApi = {
  list: () => api.get('/api/apps'),
}

// Platform Keys
export const platformKeyApi = {
  list: () => api.get('/api/platform-keys'),
  create: (data: { name: string; expiresAt?: string }) => api.post('/api/platform-keys', data),
  delete: (id: number) => api.delete(`/api/platform-keys/${id}`),
  reveal: (id: number) => api.get(`/api/platform-keys/${id}/reveal`),
}