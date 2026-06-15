import { useEffect, useState } from 'react'
import type { Node } from '../types'
import { downloadUrl } from '../api'

const TEXT_MIME_TYPES = new Set(['application/json', 'application/xml'])
const TEXT_TRUNCATE_BYTES = 500 * 1024 // 500 KB

function isTextMime(mime: string) {
  return mime.startsWith('text/') || TEXT_MIME_TYPES.has(mime)
}

interface Props {
  node: Node
  onClose: () => void
}

export function FileViewer({ node, onClose }: Props) {
  const url = downloadUrl(node.id)
  const mime = node.mime_type ?? ''
  const isImage = mime.startsWith('image/')
  const isVideo = mime.startsWith('video/')
  const isText = isTextMime(mime)

  const [text, setText] = useState<string | null>(null)
  const [truncated, setTruncated] = useState(false)
  const [textLoading, setTextLoading] = useState(false)
  const [viewAsText, setViewAsText] = useState(false)

  useEffect(() => {
    if (isText) loadText()
  }, [])

  async function loadText() {
    setTextLoading(true)
    try {
      const res = await fetch(url)
      const raw = await res.text()
      if (raw.length > TEXT_TRUNCATE_BYTES) {
        setText(raw.slice(0, TEXT_TRUNCATE_BYTES))
        setTruncated(true)
      } else {
        setText(raw)
      }
    } finally {
      setTextLoading(false)
    }
  }

  function handleViewAsText() {
    setViewAsText(true)
    loadText()
  }

  const showText = (isText || viewAsText) && !isImage && !isVideo

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="viewer-modal" onClick={(e) => e.stopPropagation()}>
        <div className="viewer-header">
          <span className="viewer-title" title={node.name}>{node.name}</span>
          <a className="viewer-download link" href={url} download={node.name}>Download</a>
          <button onClick={onClose} title="Close">✕</button>
        </div>

        <div className="viewer-body">
          {isImage && (
            <img src={url} alt={node.name} className="viewer-image" />
          )}

          {isVideo && (
            // Range requests are handled by http.ServeContent — seeking works.
            <video src={url} controls className="viewer-video" />
          )}

          {showText && (
            textLoading
              ? <p className="status">Loading…</p>
              : <>
                  {truncated && (
                    <p className="viewer-truncated">
                      File is large — showing first 500 KB
                    </p>
                  )}
                  <pre className="viewer-text">{text}</pre>
                </>
          )}

          {!isImage && !isVideo && !showText && (
            <div className="viewer-unknown">
              <p>No preview available for this file type.</p>
              <button onClick={handleViewAsText}>View as text</button>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
