import { useState } from 'react'
import { List } from 'react-window'
import type { CSSProperties } from 'react'
import * as api from '../api'
import type { Node } from '../types'
import { FileRow } from './FileRow'

interface RowProps {
  nodes: Node[]
  onNavigate: (id: number) => void
  onDelete: (id: number) => void
  onPreview: (node: Node) => void
  onNavigateUp: (() => void) | undefined
  hasParent: boolean
  draggedId: number | null
  dragOverId: number | null
  onDragStart: (id: number) => void
  onDragEnd: () => void
  onDragOver: (id: number) => void
  onDragLeave: () => void
  onDrop: (targetId: number) => void
}

function Row({
  index,
  style,
  nodes,
  onNavigate,
  onDelete,
  onPreview,
  onNavigateUp,
  hasParent,
  draggedId,
  dragOverId,
  onDragStart,
  onDragEnd,
  onDragOver,
  onDragLeave,
  onDrop,
}: { index: number; style: CSSProperties } & RowProps) {
  if (hasParent && index === 0) {
    return (
      <div style={style} className="file-row file-row--up" onClick={onNavigateUp}>
        <span className="file-icon">📁</span>
        <span className="file-name">..</span>
      </div>
    )
  }

  const nodeIndex = hasParent ? index - 1 : index
  return (
    <FileRow
      node={nodes[nodeIndex]}
      style={style}
      onNavigate={onNavigate}
      onDelete={onDelete}
      onPreview={onPreview}
      draggedId={draggedId}
      dragOverId={dragOverId}
      onDragStart={onDragStart}
      onDragEnd={onDragEnd}
      onDragOver={onDragOver}
      onDragLeave={onDragLeave}
      onDrop={onDrop}
    />
  )
}

interface FileListProps {
  nodes: Node[]
  onNavigate: (id: number) => void
  onRefresh: () => void
  onDelete: (id: number) => void
  onPreview: (node: Node) => void
  onNavigateUp?: () => void
}

export function FileList({ nodes, onNavigate, onDelete, onRefresh, onPreview, onNavigateUp }: FileListProps) {
  const [draggedId, setDraggedId] = useState<number | null>(null)
  const [dragOverId, setDragOverId] = useState<number | null>(null)
  const [moveError, setMoveError] = useState<string | null>(null)

  const hasParent = onNavigateUp != null
  const rowCount = nodes.length + (hasParent ? 1 : 0)

  async function handleMove(targetId: number) {
    if (draggedId == null || draggedId === targetId) return
    try {
      await api.updateNode(draggedId, { parent_id: targetId })
      onRefresh()
    } catch {
      setMoveError('Move failed — cycle detected or name conflict')
      setTimeout(() => setMoveError(null), 3000)
    } finally {
      setDraggedId(null)
      setDragOverId(null)
    }
  }

  if (rowCount === 0) {
    return <p className="empty">No files or folders here.</p>
  }

  return (
    <>
      {moveError && <p className="status error">{moveError}</p>}
      <List<RowProps>
        rowCount={rowCount}
        rowHeight={48}
        rowComponent={Row}
        rowProps={{
          nodes,
          onNavigate,
          onDelete,
          onPreview,
          onNavigateUp,
          hasParent,
          draggedId,
          dragOverId,
          onDragStart: setDraggedId,
          onDragEnd: () => { setDraggedId(null); setDragOverId(null) },
          onDragOver: (id) => setDragOverId(prev => prev === id ? prev : id),
          onDragLeave: () => setDragOverId(null),
          onDrop: handleMove,
        }}
        defaultHeight={600}
        style={{ width: '100%' }}
      />
    </>
  )
}
