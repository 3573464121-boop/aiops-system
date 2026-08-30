export const API = 'http://localhost:8080/api/v1'

const TOKEN_KEY = 'aiops_token'
const USER_KEY = 'aiops_user'

export const getToken = () => localStorage.getItem(TOKEN_KEY) || ''
export function getUser() {
  try { return JSON.parse(localStorage.getItem(USER_KEY) || 'null') } catch { return null }
}
export function setSession(token, user) {
  localStorage.setItem(TOKEN_KEY, token)
  localStorage.setItem(USER_KEY, JSON.stringify(user))
}
export function clearSession() {
  localStorage.removeItem(TOKEN_KEY)
  localStorage.removeItem(USER_KEY)
}
export const isLoggedIn = () => !!getToken()
export const isAdmin = () => getUser()?.role === 'admin'

// authHeaders 返回带令牌的请求头，供绕过 api()（如 SSE 流式）的调用直接复用。
export function authHeaders(extra = {}) {
  const token = getToken()
  return token ? { ...extra, Authorization: `Bearer ${token}` } : { ...extra }
}

function unauthorized() {
  clearSession()
  if (!location.hash.startsWith('#/login')) location.hash = '#/login'
}

export async function api(path, options = {}) {
  const resp = await fetch(`${API}${path}`, { ...options, headers: authHeaders(options.headers) })
  if (resp.status === 401) {
    unauthorized()
    throw new Error('登录已过期，请重新登录')
  }
  if (!resp.ok) {
    const d = await resp.json().catch(() => ({}))
    throw new Error(d.error || `请求失败（${resp.status}）`)
  }
  return resp.json()
}

export const sevText = l => ({ 0: '灾难', 1: '严重', 2: '高', 3: '中', 4: '低' }[l] || '未知')
export const sevColor = l => ({ 0: 'red', 1: 'red', 2: 'orange', 3: 'gold', 4: 'blue' }[l] || 'default')
