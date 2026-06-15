import { useCallback, useEffect, useState } from 'react'
import * as api from '../api'
import type { Node } from '../types'

function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString(undefined, {
    month: 'short', day: 'numeric', year: 'numeric',
  })
}

export function TrashView() {
  const [nodes, setNodes] = useState<Node[]>([])
  const [loading, setLoading] = useState(true)

  const load = useCallback(() => {
    setLoading(true)
    api.listTrash()
      .then(setNodes)
      .catch(() => {})
      .finally(() => setLoading(false))
  }, [])

  useEffect(() => { load() }, [load])

  async function handleRestore(id: number) {
    await api.restoreNode(id)
    load()
  }

  async function handlePermanentDelete(id: number) {
    await api.permanentDelete(id)
    load()
  }

  async function handleDeleteAll() {
    await Promise.allSettled(nodes.map((n) => api.permanentDelete(n.id)))
    load()
  }

  if (loading) return <p className="status">Loading…</p>
  if (nodes.length === 0) return <p className="empty">Trash is empty.</p>

  return (
    <div className="trash-list">
      <div className="trash-toolbar">
        <button className="trash-delete-btn" onClick={handleDeleteAll}>Delete All</button>
      </div>
      {nodes.map((node) => (
        <div key={node.id} className="trash-row">
          <span className="file-icon">{node.type === 'folder' ? '📁' : '📄'}</span>
          <span className="trash-name">{node.name}</span>
          <span className="trash-date">{node.deleted_at ? formatDate(node.deleted_at) : ''}</span>
          <button onClick={() => handleRestore(node.id)}>Restore</button>
          <button className="trash-delete-btn" onClick={() => handlePermanentDelete(node.id)}>
            Delete forever
          </button>
        </div>
      ))}
    </div>
  )
}
