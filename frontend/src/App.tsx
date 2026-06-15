import { useRef, useState } from 'react'
import * as api from './api'
import { deleteNode } from './api'
import { Breadcrumb } from './components/Breadcrumb'
import { FileList } from './components/FileList'
import { FileViewer } from './components/FileViewer'
import { SearchBar } from './components/SearchBar'
import { Toolbar } from './components/Toolbar'
import { TrashView } from './components/TrashView'
import { useFiles } from './hooks/useFiles'
import type { Node } from './types'

export default function App() {
  const [currentNodeId, setCurrentNodeId] = useState<number | null>(null)
  const [searchQuery, setSearchQuery] = useState('')
  const { nodes, loading, error, refresh } = useFiles(currentNodeId, searchQuery)

  const [previewNode, setPreviewNode] = useState<Node | null>(null)
  const [showTrash, setShowTrash] = useState(false)

  // External file drag (Finder → app)
  const dragCounter = useRef(0)
  const [dropActive, setDropActive] = useState(false)
  const [dropping, setDropping] = useState(false)

  async function handleDelete(id: number) {
    await deleteNode(id)
    refresh()
  }

  async function handleNavigateUp() {
    if (currentNodeId == null) return
    const node = await api.getNode(currentNodeId)
    setCurrentNodeId(node.parent_id)
  }

  function isFileDrag(e: React.DragEvent) {
    return Array.from(e.dataTransfer.types).includes('Files')
  }

  function handleDragEnter(e: React.DragEvent) {
    if (!isFileDrag(e)) return
    e.preventDefault()
    dragCounter.current++
    setDropActive(true)
  }

  function handleDragOver(e: React.DragEvent) {
    if (!isFileDrag(e)) return
    e.preventDefault()
  }

  function handleDragLeave(e: React.DragEvent) {
    if (!isFileDrag(e)) return
    dragCounter.current--
    if (dragCounter.current === 0) setDropActive(false)
  }

  async function handleDrop(e: React.DragEvent) {
    e.preventDefault()
    dragCounter.current = 0
    setDropActive(false)
    if (!isFileDrag(e)) return
    const files = Array.from(e.dataTransfer.files)
    if (files.length === 0) return
    setDropping(true)
    await Promise.allSettled(files.map((f) => api.uploadFile(f, currentNodeId ?? undefined)))
    setDropping(false)
    refresh()
  }

  return (
    <div
      className="app"
      onDragEnter={handleDragEnter}
      onDragOver={handleDragOver}
      onDragLeave={handleDragLeave}
      onDrop={handleDrop}
    >
      {dropActive && (
        <div className="drop-overlay">
          <div className="drop-overlay-label">Drop to upload</div>
        </div>
      )}

      <header className="app-header">
        <h1>{showTrash ? 'Trash' : 'Files'}</h1>
        <button className="link app-header-link" onClick={() => setShowTrash((v) => !v)}>
          {showTrash ? '← Back to Files' : 'Trash'}
        </button>
      </header>

      {showTrash ? (
        <main><TrashView /></main>
      ) : (
        <>
          <Breadcrumb nodeId={currentNodeId} onNavigate={setCurrentNodeId} />
          <SearchBar onSearch={setSearchQuery} />
          <Toolbar parentId={currentNodeId} onRefresh={refresh} />
          <main>
            {dropping && <p className="status">Uploading…</p>}
            {loading && <p className="status">Loading…</p>}
            {error && <p className="status error">Error: {error}</p>}
            {!loading && !error && (
              <FileList
                nodes={nodes}
                onNavigate={setCurrentNodeId}
                onRefresh={refresh}
                onDelete={handleDelete}
                onPreview={setPreviewNode}
                onNavigateUp={currentNodeId != null ? handleNavigateUp : undefined}
              />
            )}
          </main>
        </>
      )}
      {previewNode && (
        <FileViewer node={previewNode} onClose={() => setPreviewNode(null)} />
      )}
    </div>
  )
}
