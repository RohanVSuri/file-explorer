import { useRef, useState } from 'react'
import { uploadFile } from '../api'
import { CreateFolderModal } from './CreateFolderModal'

interface UploadProgress {
  name: string
  pct: number
}

interface ToolbarProps {
  parentId: number | null
  onRefresh: () => void
}

export function Toolbar({ parentId, onRefresh }: ToolbarProps) {
  const fileInput = useRef<HTMLInputElement>(null)
  const [uploads, setUploads] = useState<UploadProgress[]>([])
  const [showModal, setShowModal] = useState(false)

  function handleFiles(files: FileList) {
    const list = Array.from(files)
    setUploads(list.map((f) => ({ name: f.name, pct: 0 })))

    let done = 0
    list.forEach((file, i) => {
      uploadFile(file, parentId ?? undefined, (pct) => {
        setUploads((prev) => prev.map((u, j) => (j === i ? { ...u, pct } : u)))
      }).then(() => {
        done++
        if (done === list.length) {
          setUploads([])
          onRefresh()
        }
      })
    })
  }

  return (
    <div className="toolbar">
      <button onClick={() => fileInput.current?.click()}>Upload</button>
      <input
        ref={fileInput}
        type="file"
        multiple
        hidden
        onChange={(e) => e.target.files && handleFiles(e.target.files)}
      />
      <button onClick={() => setShowModal(true)}>New Folder</button>

      {uploads.length > 0 && (
        <div className="upload-progress">
          {uploads.map((u) => (
            <div key={u.name} className="upload-item">
              <span>{u.name}</span>
              <progress value={u.pct} max={100} />
            </div>
          ))}
        </div>
      )}

      {showModal && (
        <CreateFolderModal
          parentId={parentId}
          onClose={() => setShowModal(false)}
          onRefresh={onRefresh}
        />
      )}
    </div>
  )
}
