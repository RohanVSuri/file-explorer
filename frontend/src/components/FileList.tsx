import { List } from 'react-window'
import type { CSSProperties } from 'react'
import type { Node } from '../types'
import { FileRow } from './FileRow'

interface RowProps {
  nodes: Node[]
  onNavigate: (id: number) => void
  onDelete: (id: number) => void
}

// react-window v2 injects index and style; RowProps holds the shared data.
function Row({
  index,
  style,
  nodes,
  onNavigate,
  onDelete,
}: { index: number; style: CSSProperties } & RowProps) {
  return (
    <FileRow
      node={nodes[index]}
      style={style}
      onNavigate={onNavigate}
      onDelete={onDelete}
    />
  )
}

interface FileListProps {
  nodes: Node[]
  onNavigate: (id: number) => void
  onRefresh: () => void
  onDelete: (id: number) => void
}

export function FileList({ nodes, onNavigate, onDelete }: FileListProps) {
  if (nodes.length === 0) {
    return <p className="empty">No files or folders here.</p>
  }

  return (
    <List<RowProps>
      rowCount={nodes.length}
      rowHeight={48}
      rowComponent={Row}
      rowProps={{ nodes, onNavigate, onDelete }}
      defaultHeight={600}
      style={{ width: '100%' }}
    />
  )
}
