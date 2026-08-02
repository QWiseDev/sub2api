import { describe, expect, it } from 'vitest'
import { extractConversationResponseText } from '../conversationResponse'

describe('extractConversationResponseText', () => {
  it('extracts OpenAI chat JSON text', () => {
    expect(extractConversationResponseText('openai_chat', 'application/json', JSON.stringify({
      choices: [{ message: { content: 'hello' } }]
    }))).toBe('hello')
  })

  it('joins OpenAI and Anthropic SSE deltas', () => {
    const body = [
      'data: {"choices":[{"delta":{"content":"Hel"}}]}',
      'data: {"type":"content_block_delta","delta":{"text":"lo"}}',
      'data: [DONE]'
    ].join('\n\n')
    expect(extractConversationResponseText('openai_chat', 'text/event-stream', body)).toBe('Hello')
  })

  it('extracts Responses API output text', () => {
    expect(extractConversationResponseText('openai_responses', 'application/json', JSON.stringify({
      output: [{ type: 'message', content: [{ type: 'output_text', text: 'done' }] }]
    }))).toBe('done')
  })

  it('falls back to the raw body', () => {
    expect(extractConversationResponseText('unknown', 'text/plain', 'plain reply')).toBe('plain reply')
  })
})
