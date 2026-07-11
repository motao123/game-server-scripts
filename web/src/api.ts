type ApiOptions = Omit<RequestInit, 'body'> & { body?: unknown }

export async function api<T>(path: string, opts: ApiOptions = {}): Promise<T> {
  const headers = new Headers(opts.headers)
  if (opts.body !== undefined) headers.set('Content-Type', 'application/json')
  const csrf = localStorage.getItem('csrf')
  if (csrf) headers.set('X-CSRF-Token', csrf)
  const response = await fetch(path, {
    ...opts,
    headers,
    credentials: 'same-origin',
    body: opts.body !== undefined ? JSON.stringify(opts.body) : undefined
  })
  const data = await response.json().catch(() => ({}))
  if (!response.ok) throw new Error(data.error || '请求失败')
  return data as T
}
