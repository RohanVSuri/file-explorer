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
  onPreview: (node: Node) => void
  draggedId: number | null
  dragOverId: number | null
  onDragStart: (id: number) => void
  onDragEnd: () => void
  onDragOver: (id: number) => void
  onDragLeave: () => void
  onDrop: (targetId: number) => void
}

export function FileRow({
  node,
  style,
  onNavigate,
  onDelete,
  onPreview,
  draggedId,
  dragOverId,
  onDragStart,
  onDragEnd,
  onDragOver,
  onDragLeave,
  onDrop,
}: FileRowProps) {
  const isFolder = node.type === 'folder'

  const className = [
    'file-row',
    draggedId === node.id  ? 'file-row--dragging'  : '',
    dragOverId === node.id ? 'file-row--drag-over' : '',
  ].filter(Boolean).join(' ')

  return (
    <div
      style={style}
      className={className}
      draggable
      onDragStart={(e) => { e.dataTransfer.setData('application/x-node-id', String(node.id)); onDragStart(node.id) }}
      onDragEnd={onDragEnd}
      onDragOver={(e) => { if (isFolder) { e.preventDefault(); onDragOver(node.id) } }}
      onDragLeave={onDragLeave}
      onDrop={(e) => { e.preventDefault(); if (isFolder) onDrop(node.id) }}
    >
      <span className="file-icon">{isFolder ? '📁' : '📄'}</span>

      {isFolder ? (
        <button className="file-name link" onClick={() => onNavigate(node.id)}>
          {node.name}
        </button>
      ) : (
        <>
          <button className="file-name link" onClick={() => onPreview(node)}>
            {node.name}
          </button>
          <a className="file-download-icon" href={downloadUrl(node.id)} download={node.name} title="Download">⬇</a>
        </>
      )}

      <span className="file-size">{formatSize(node.size)}</span>
      <span className="file-date">{formatDate(node.updated_at)}</span>

      <button className="file-delete" onClick={() => onDelete(node.id)} title="Delete">
        ✕
      </button>
    </div>
  )
}
