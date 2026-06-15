import { useState } from 'react'
import { deleteNode } from './api'
import { Breadcrumb } from './components/Breadcrumb'
import { FileList } from './components/FileList'
import { SearchBar } from './components/SearchBar'
import { Toolbar } from './components/Toolbar'
import { useFiles } from './hooks/useFiles'

export default function App() {
  const [currentNodeId, setCurrentNodeId] = useState<number | null>(null)
  const [searchQuery, setSearchQuery] = useState('')
  const { nodes, loading, error, refresh } = useFiles(currentNodeId, searchQuery)

  async function handleDelete(id: number) {
    await deleteNode(id)
    refresh()
  }

  return (
    <div className="app">
      <header className="app-header">
        <h1>Files</h1>
      </header>

      <Breadcrumb nodeId={currentNodeId} onNavigate={setCurrentNodeId} />
      <SearchBar onSearch={setSearchQuery} />
      <Toolbar parentId={currentNodeId} onRefresh={refresh} />

      <main>
        {loading && <p className="status">Loading…</p>}
        {error && <p className="status error">Error: {error}</p>}
        {!loading && !error && (
          <FileList
            nodes={nodes}
            onNavigate={setCurrentNodeId}
            onRefresh={refresh}
            onDelete={handleDelete}
          />
        )}
      </main>
    </div>
  )
}
