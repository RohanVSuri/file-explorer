import { useState } from 'react'
import { createFolder } from '../api'

interface CreateFolderModalProps {
  parentId: number | null
  onClose: () => void
  onRefresh: () => void
}

export function CreateFolderModal({ parentId, onClose, onRefresh }: CreateFolderModalProps) {
  const [name, setName] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!name.trim()) return
    setLoading(true)
    setError(null)
    try {
      await createFolder(name.trim(), parentId ?? undefined)
      onRefresh()
      onClose()
    } catch {
      setError('A folder with that name already exists.')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="modal" onClick={(e) => e.stopPropagation()}>
        <h2>New Folder</h2>
        <form onSubmit={handleSubmit}>
          <input
            autoFocus
            type="text"
            placeholder="Folder name"
            value={name}
            onChange={(e) => setName(e.target.value)}
          />
          {error && <p className="error">{error}</p>}
          <div className="modal-actions">
            <button type="button" onClick={onClose}>Cancel</button>
            <button type="submit" disabled={loading || !name.trim()}>Create</button>
          </div>
        </form>
      </div>
    </div>
  )
}
