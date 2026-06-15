import type { Node } from '../types'

const BASE_URL = import.meta.env.VITE_API_URL ?? 'http://localhost:8080'

async function apiFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE_URL}${path}`, init)
  if (!res.ok) throw new Error(`${res.status}`)
  if (res.status === 204 || res.headers.get('content-length') === '0') return undefined as T
  return res.json()
}

export function listChildren(parentId?: number): Promise<Node[]> {
  const qs = parentId != null ? `?parent_id=${parentId}` : ''
  return apiFetch(`/nodes/children${qs}`)
}

export function getNode(id: number): Promise<Node> {
  return apiFetch(`/nodes/${id}`)
}

export function createFolder(name: string, parentId?: number): Promise<Node> {
  return apiFetch('/nodes', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name, parent_id: parentId ?? null }),
  })
}

export function updateNode(
  id: number,
  patch: { name?: string; parent_id?: number | null }
): Promise<Node> {
  return apiFetch(`/nodes/${id}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(patch),
  })
}

export function deleteNode(id: number): Promise<void> {
  return apiFetch(`/nodes/${id}`, { method: 'DELETE' })
}

export function uploadFile(
  file: File,
  parentId?: number,
  onProgress?: (pct: number) => void
): Promise<Node> {
  return new Promise((resolve, reject) => {
    const form = new FormData()
    form.append('file', file)
    if (parentId != null) form.append('parent_id', String(parentId))

    const xhr = new XMLHttpRequest()
    xhr.upload.onprogress = (e) => {
      if (e.lengthComputable) onProgress?.(Math.round((e.loaded / e.total) * 100))
    }
    xhr.onload = () => {
      if (xhr.status >= 200 && xhr.status < 300) {
        resolve(JSON.parse(xhr.responseText) as Node)
      } else {
        reject(new Error(`upload failed: ${xhr.status}`))
      }
    }
    xhr.onerror = () => reject(new Error('upload failed'))
    xhr.open('POST', `${BASE_URL}/files`)
    xhr.send(form)
  })
}

export function downloadUrl(id: number): string {
  return `${BASE_URL}/files/${id}/content`
}

export function search(q: string, parentId?: number): Promise<Node[]> {
  const qs = new URLSearchParams({ q })
  if (parentId != null) qs.set('parent_id', String(parentId))
  return apiFetch(`/search?${qs}`)
}

export function restoreNode(id: number): Promise<Node> {
  return apiFetch(`/trash/${id}/restore`, { method: 'POST' })
}

export function permanentDelete(id: number): Promise<void> {
  return apiFetch(`/trash/${id}`, { method: 'DELETE' })
}
