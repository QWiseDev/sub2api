type JSONRecord = Record<string, unknown>

export function extractConversationResponseText(protocol: string, contentType: string, raw: string): string {
  const body = raw.trim()
  if (!body) return ''
  if (isEventStream(contentType, body)) {
    return extractEventStreamText(body)
  }
  try {
    return extractJSONResponseText(protocol, JSON.parse(body)) || body
  } catch {
    return body
  }
}

function isEventStream(contentType: string, body: string): boolean {
  return contentType.toLowerCase().includes('text/event-stream') || /(^|\n)data:\s*/.test(body)
}

function extractEventStreamText(body: string): string {
  const parts: string[] = []
  for (const line of body.split(/\r?\n/)) {
    if (!line.startsWith('data:')) continue
    const data = line.slice(5).trim()
    if (!data || data === '[DONE]') continue
    try {
      const event = JSON.parse(data) as JSONRecord
      const chatDelta = collectText(getPath(event, 'choices.0.delta.content'))
      const textDelta = typeof event.delta === 'string' ? event.delta : collectText(getPath(event, 'delta.text'))
      const blockText = collectText(getPath(event, 'content_block.text'))
      if (chatDelta) parts.push(chatDelta)
      else if (textDelta) parts.push(textDelta)
      else if (blockText) parts.push(blockText)
    } catch {
      continue
    }
  }
  return parts.join('')
}

function extractJSONResponseText(protocol: string, value: unknown): string {
  const root = asRecord(value)
  if (!root) return ''

  const direct = root.output_text
  if (typeof direct === 'string') return direct

  const chat = collectText(getPath(root, 'choices.0.message.content'))
  if (chat) return chat

  const anthropic = collectText(root.content)
  if (anthropic) return anthropic

  const responses = collectText(root.output)
  if (responses) return responses

  if (protocol.includes('responses')) {
    return collectText(root.response)
  }
  return ''
}

function collectText(value: unknown): string {
  if (typeof value === 'string') return value
  if (Array.isArray(value)) return value.map(collectText).filter(Boolean).join('')
  const record = asRecord(value)
  if (!record) return ''
  if (typeof record.text === 'string') return record.text
  if (typeof record.output_text === 'string') return record.output_text
  return collectText(record.content) || collectText(record.output)
}

function getPath(value: unknown, path: string): unknown {
  let current: unknown = value
  for (const segment of path.split('.')) {
    if (Array.isArray(current)) {
      current = current[Number(segment)]
      continue
    }
    const record = asRecord(current)
    if (!record) return undefined
    current = record[segment]
  }
  return current
}

function asRecord(value: unknown): JSONRecord | null {
  return value !== null && typeof value === 'object' && !Array.isArray(value) ? value as JSONRecord : null
}
