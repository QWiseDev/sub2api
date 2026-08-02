import { apiClient } from '../client'
import type { PaginatedResponse } from '@/types'

export interface ConversationRecordConfig {
  enabled: boolean
  group_ids: number[]
  api_key_ids: number[]
}

export interface ConversationSessionSummary {
  conversation_key: string
  session_id: string
  user_id: number
  username: string
  user_email: string
  api_key_id: number
  api_key_name: string
  group_id: number
  group_name: string
  provider: string
  endpoint: string
  protocol: string
  model: string
  turn_count: number
  last_request_text: string
  first_activity_at: string
  last_activity_at: string
}

export interface ConversationTurn {
  id: number
  conversation_key: string
  session_id: string
  request_id: string
  user_id: number
  username: string
  user_email: string
  api_key_id: number
  api_key_name: string
  group_id: number
  group_name: string
  provider: string
  endpoint: string
  protocol: string
  model: string
  stream: boolean
  status_code: number
  content_type: string
  request_text: string
  request_body: string
  response_body: string
  request_truncated: boolean
  response_truncated: boolean
  created_at: string
  completed_at: string
}

export interface ListConversationsParams {
  page?: number
  page_size?: number
  search?: string
  model?: string
  user_id?: number
  api_key_id?: number
  from?: string
  to?: string
}

export async function list(
  params: ListConversationsParams,
  options?: { signal?: AbortSignal }
): Promise<PaginatedResponse<ConversationSessionSummary>> {
  const { data } = await apiClient.get<PaginatedResponse<ConversationSessionSummary>>('/admin/conversations', {
    params,
    signal: options?.signal
  })
  return data
}

export async function get(
  conversationKey: string,
  options?: { signal?: AbortSignal }
): Promise<ConversationTurn[]> {
  const { data } = await apiClient.get<ConversationTurn[]>(`/admin/conversations/${encodeURIComponent(conversationKey)}`, {
    signal: options?.signal
  })
  return data
}

export async function getConfig(): Promise<ConversationRecordConfig> {
  const { data } = await apiClient.get<ConversationRecordConfig>('/admin/conversations/config')
  return data
}

export async function updateConfig(config: ConversationRecordConfig): Promise<ConversationRecordConfig> {
  const { data } = await apiClient.put<ConversationRecordConfig>('/admin/conversations/config', config)
  return data
}

export default { list, get, getConfig, updateConfig }
