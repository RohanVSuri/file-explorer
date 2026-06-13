const BASE_URL = import.meta.env.VITE_API_URL ?? 'http://localhost:8080'

async function apiFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE_URL}${path}`, init)
  if (!res.ok) throw new Error(`${res.status}`)
  return res.json()
}

export function getHello() {
  return apiFetch<{ message: string }>('/hello')
}
