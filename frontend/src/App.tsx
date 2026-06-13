import { useEffect, useState } from 'react'
import { getHello } from './api'

export default function App() {
  const [message, setMessage] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    getHello()
      .then(data => setMessage(data.message))
      .catch(err => setError(err.message))
  }, [])

  if (error) return <p>Error: {error}</p>
  if (!message) return <p>Loading...</p>
  return <p>{message}</p>
}
