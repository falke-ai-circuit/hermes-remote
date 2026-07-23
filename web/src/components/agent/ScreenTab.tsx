import { useState } from 'react'
import { api } from '../../api/client'

export function ScreenTab({ agentId }: { agentId: string }) {
  const [output, setOutput] = useState('')
  const [error, setError] = useState('')

  const capture = async () => {
    setError('')
    setOutput('')
    try {
      const res = await api.capture(agentId)
      setOutput(JSON.stringify(res, null, 2))
    } catch (e) {
      setError((e as Error).message)
    }
  }

  return (
    <div className="card">
      <div className="card-title">Screen Capture</div>
      {error && <div className="error-msg">{error}</div>}
      <button className="btn btn-primary btn-sm" onClick={capture}>Capture Screen</button>
      {output && <div className="terminal-output mt-16">{output}</div>}
    </div>
  )
}