export interface Node {
  id: number
  parent_id: number | null
  name: string
  type: 'file' | 'folder'
  size?: number
  mime_type?: string
  created_at: string
  updated_at: string
  deleted_at?: string
}
