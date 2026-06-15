import { useEffect, useState } from 'react'
import { getNode } from '../api'
import type { Node } from '../types'

interface BreadcrumbProps {
  nodeId: number | null
  onNavigate: (id: number | null) => void
}

export function Breadcrumb({ nodeId, onNavigate }: BreadcrumbProps) {
  const [ancestors, setAncestors] = useState<Node[]>([])

  useEffect(() => {
    if (nodeId == null) {
      setAncestors([])
      return
    }

    // Walk up the parent chain to build the breadcrumb trail.
    async function buildTrail(id: number) {
      const trail: Node[] = []
      let current: Node | null = null
      let next: number | null = id

      while (next != null) {
        current = await getNode(next)
        trail.unshift(current)
        next = current.parent_id
      }
      setAncestors(trail)
    }

    buildTrail(nodeId)
  }, [nodeId])

  return (
    <nav className="breadcrumb">
      <button className="link" onClick={() => onNavigate(null)}>
        Home
      </button>
      {ancestors.map((node) => (
        <span key={node.id}>
          <span className="breadcrumb-sep"> / </span>
          <button className="link" onClick={() => onNavigate(node.id)}>
            {node.name}
          </button>
        </span>
      ))}
    </nav>
  )
}
