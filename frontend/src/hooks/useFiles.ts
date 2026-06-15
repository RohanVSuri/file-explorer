import { useCallback, useEffect, useState } from 'react'
import * as api from '../api'
import type { Node } from '../types'

export function useFiles(parentId: number | null, searchQuery: string) {
  const [nodes, setNodes] = useState<Node[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const fetch = useCallback(() => {
    setLoading(true)
    setError(null)

    const req = searchQuery.trim()
      ? api.search(searchQuery.trim(), parentId ?? undefined)
      : api.listChildren(parentId ?? undefined)

    req
      .then(setNodes)
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false))
  }, [parentId, searchQuery])

  useEffect(() => {
    fetch()
  }, [fetch])

  return { nodes, loading, error, refresh: fetch }
}
