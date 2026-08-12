export const API = 'http://localhost:8080/api/v1'

export async function api(path, options) {
  const resp = await fetch(`${API}${path}`, options)
  if (!resp.ok) {
    const d = await resp.json().catch(() => ({}))
    throw new Error(d.error || `请求失败（${resp.status}）`)
  }
  return resp.json()
}

export const sevText = l => ({ 0: '灾难', 1: '严重', 2: '高', 3: '中', 4: '低' }[l] || '未知')
export const sevColor = l => ({ 0: 'red', 1: 'red', 2: 'orange', 3: 'gold', 4: 'blue' }[l] || 'default')
