import type { CSSProperties } from 'react'
import type { Node } from '../types'
import { downloadUrl } from '../api'

function formatSize(bytes?: number): string {
  if (bytes == null) return '—'
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
  return `${(bytes / (1024 * 1024 * 1024)).toFixed(1)} GB`
}

function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString(undefined, {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
  })
}

interface FileRowProps {
  node: Node
  style: CSSProperties
  onNavigate: (id: number) => void
  onDelete: (id: number) => void
}

export function FileRow({ node, style, onNavigate, onDelete }: FileRowProps) {
  const isFolder = node.type === 'folder'

  return (
    <div style={style} className="file-row">
      <span className="file-icon">{isFolder ? '📁' : '📄'}</span>

      {isFolder ? (
        <button className="file-name link" onClick={() => onNavigate(node.id)}>
          {node.name}
        </button>
      ) : (
        <a className="file-name link" href={downloadUrl(node.id)} download={node.name}>
          {node.name}
        </a>
      )}

      <span className="file-size">{formatSize(node.size)}</span>
      <span className="file-date">{formatDate(node.updated_at)}</span>

      <button className="file-delete" onClick={() => onDelete(node.id)} title="Delete">
        ✕
      </button>
    </div>
  )
}
