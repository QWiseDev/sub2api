<template>
  <AppLayout>
    <div class="space-y-6">
      <div class="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
        <div>
          <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">{{ t('admin.conversations.title') }}</h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.conversations.description') }}</p>
        </div>
        <button type="button" class="btn btn-secondary inline-flex items-center gap-2" :disabled="loading" @click="loadSessions">
          <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
          {{ t('admin.conversations.refresh') }}
        </button>
      </div>

      <section class="card p-5">
        <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
          <div>
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.conversations.config.title') }}</h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.conversations.config.description') }}</p>
          </div>
          <div v-if="configReady" class="flex items-center gap-3">
            <span class="text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ configForm.enabled ? t('admin.conversations.config.enabled') : t('admin.conversations.config.disabled') }}
            </span>
            <Toggle v-model="configForm.enabled" />
          </div>
        </div>

        <div v-if="configLoading" class="flex items-center justify-center py-10">
          <LoadingSpinner />
        </div>
        <div v-else class="mt-5 space-y-4">
          <div class="grid grid-cols-1 gap-5 xl:grid-cols-2">
            <GroupSelector v-model="configForm.group_ids" :groups="groups" searchable />
            <ConversationAPIKeySelector v-model="configForm.api_key_ids" />
          </div>

          <div class="rounded-lg bg-gray-50 px-4 py-3 text-xs text-gray-600 dark:bg-dark-800 dark:text-gray-400">
            {{ t('admin.conversations.config.matchHint') }}
          </div>
          <div
            v-if="configForm.enabled && configForm.group_ids.length === 0 && configForm.api_key_ids.length === 0"
            class="rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-700 dark:border-amber-900/50 dark:bg-amber-950/30 dark:text-amber-300"
          >
            {{ t('admin.conversations.config.emptyScopeWarning') }}
          </div>
          <div v-if="configError" class="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-900/50 dark:bg-red-950/30 dark:text-red-300">
            {{ configError }}
          </div>
          <div class="flex justify-end">
            <button type="button" class="btn btn-primary min-w-28" :disabled="configSaving || !configReady" @click="saveConfig">
              <Icon v-if="configSaving" name="refresh" size="sm" class="mr-2 animate-spin" />
              {{ configSaving ? t('admin.conversations.config.saving') : t('admin.conversations.config.save') }}
            </button>
          </div>
        </div>
      </section>

      <form class="card grid grid-cols-1 gap-3 p-4 md:grid-cols-2 xl:grid-cols-[minmax(260px,1fr)_240px_170px_170px_auto]" @submit.prevent="applyFilters">
        <input v-model.trim="filters.search" class="input" :placeholder="t('admin.conversations.searchPlaceholder')" />
        <input v-model.trim="filters.model" class="input font-mono" :placeholder="t('admin.conversations.modelPlaceholder')" />
        <label class="space-y-1 text-xs text-gray-500 dark:text-gray-400">
          <span>{{ t('admin.conversations.from') }}</span>
          <input v-model="filters.from" type="date" class="input" />
        </label>
        <label class="space-y-1 text-xs text-gray-500 dark:text-gray-400">
          <span>{{ t('admin.conversations.to') }}</span>
          <input v-model="filters.to" type="date" class="input" />
        </label>
        <div class="flex items-end gap-2">
          <button type="submit" class="btn btn-primary flex-1">{{ t('admin.conversations.filter') }}</button>
          <button type="button" class="btn btn-secondary" @click="resetFilters">{{ t('admin.conversations.reset') }}</button>
        </div>
      </form>

      <div v-if="error" class="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-900/50 dark:bg-red-950/30 dark:text-red-300">
        {{ error }}
      </div>

      <div class="card overflow-hidden">
        <div v-if="loading" class="flex items-center justify-center py-20">
          <LoadingSpinner size="lg" />
        </div>
        <EmptyState
          v-else-if="sessions.length === 0"
          :title="t('admin.conversations.empty')"
          :description="t('admin.conversations.emptyHint')"
        />
        <template v-else>
          <div class="overflow-x-auto">
            <table class="min-w-full divide-y divide-gray-200 dark:divide-dark-700">
              <thead class="bg-gray-50 dark:bg-dark-800/80">
                <tr class="text-left text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">
                  <th class="px-4 py-3">{{ t('admin.conversations.columns.user') }}</th>
                  <th class="px-4 py-3">{{ t('admin.conversations.columns.apiKey') }}</th>
                  <th class="px-4 py-3">{{ t('admin.conversations.columns.model') }}</th>
                  <th class="px-4 py-3">{{ t('admin.conversations.columns.turns') }}</th>
                  <th class="px-4 py-3">{{ t('admin.conversations.columns.session') }}</th>
                  <th class="min-w-[320px] px-4 py-3">{{ t('admin.conversations.columns.lastInput') }}</th>
                  <th class="whitespace-nowrap px-4 py-3">{{ t('admin.conversations.columns.lastActivity') }}</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-100 bg-white dark:divide-dark-700 dark:bg-dark-800">
                <tr
                  v-for="session in sessions"
                  :key="session.conversation_key"
                  class="cursor-pointer transition-colors hover:bg-primary-50/60 dark:hover:bg-primary-950/20"
                  tabindex="0"
                  @click="openDetails(session)"
                  @keydown.enter="openDetails(session)"
                >
                  <td class="px-4 py-3">
                    <p class="text-sm font-medium text-gray-900 dark:text-white">{{ session.username || session.user_email || `#${session.user_id}` }}</p>
                    <p class="text-xs text-gray-500 dark:text-gray-400">{{ session.user_email }}</p>
                  </td>
                  <td class="px-4 py-3">
                    <p class="text-sm text-gray-900 dark:text-white">{{ session.api_key_name || `#${session.api_key_id}` }}</p>
                    <p class="text-xs text-gray-500 dark:text-gray-400">#{{ session.api_key_id }}</p>
                  </td>
                  <td class="px-4 py-3">
                    <p class="max-w-[220px] truncate font-mono text-sm text-gray-800 dark:text-gray-200" :title="session.model">{{ session.model || '-' }}</p>
                    <p class="text-xs text-gray-500 dark:text-gray-400">{{ session.protocol }}</p>
                  </td>
                  <td class="px-4 py-3 text-sm font-semibold text-gray-900 dark:text-white">{{ session.turn_count }}</td>
                  <td class="px-4 py-3">
                    <p v-if="session.session_id" class="max-w-[220px] truncate font-mono text-xs text-gray-700 dark:text-gray-300" :title="session.session_id">{{ session.session_id }}</p>
                    <span v-else class="rounded-full bg-gray-100 px-2 py-1 text-xs text-gray-500 dark:bg-dark-700 dark:text-gray-400">{{ t('admin.conversations.independentRequest') }}</span>
                  </td>
                  <td class="px-4 py-3">
                    <p class="line-clamp-2 whitespace-pre-wrap text-sm text-gray-700 dark:text-gray-300">{{ session.last_request_text || '-' }}</p>
                  </td>
                  <td class="whitespace-nowrap px-4 py-3 text-sm text-gray-600 dark:text-gray-400">{{ formatDateTime(session.last_activity_at) }}</td>
                </tr>
              </tbody>
            </table>
          </div>
          <Pagination
            :total="total"
            :page="page"
            :page-size="pageSize"
            @update:page="changePage"
            @update:page-size="changePageSize"
          />
        </template>
      </div>
    </div>

    <BaseDialog
      :show="Boolean(selectedSession)"
      :title="detailTitle"
      width="full"
      @close="closeDetails"
    >
      <div class="max-h-[78vh] overflow-y-auto pr-1">
        <div v-if="detailLoading" class="flex items-center justify-center py-20">
          <LoadingSpinner size="lg" />
        </div>
        <div v-else-if="detailError" class="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-900/50 dark:bg-red-950/30 dark:text-red-300">
          {{ detailError }}
        </div>
        <div v-else class="space-y-6">
          <div v-if="selectedSession" class="grid grid-cols-2 gap-3 rounded-lg bg-gray-50 p-4 text-sm dark:bg-dark-800 md:grid-cols-4">
            <MetadataItem :label="t('admin.conversations.columns.user')" :value="selectedSession.username || selectedSession.user_email || `#${selectedSession.user_id}`" />
            <MetadataItem :label="t('admin.conversations.columns.apiKey')" :value="selectedSession.api_key_name || `#${selectedSession.api_key_id}`" />
            <MetadataItem :label="t('admin.conversations.columns.session')" :value="selectedSession.session_id || t('admin.conversations.independentRequest')" mono />
            <MetadataItem :label="t('admin.conversations.columns.turns')" :value="String(turns.length)" />
          </div>

          <article v-for="(turn, index) in turns" :key="turn.id" class="rounded-xl border border-gray-200 p-4 dark:border-dark-700">
            <div class="mb-4 flex flex-wrap items-center gap-x-4 gap-y-2 border-b border-gray-100 pb-3 text-xs text-gray-500 dark:border-dark-700 dark:text-gray-400">
              <span class="font-semibold text-gray-800 dark:text-gray-200">#{{ index + 1 }}</span>
              <span>{{ t('admin.conversations.capturedAt') }}: {{ formatDateTime(turn.created_at) }}</span>
              <span>{{ t('admin.conversations.protocol') }}: {{ turn.protocol }}</span>
              <span>{{ t('admin.conversations.endpoint') }}: {{ turn.endpoint }}</span>
              <span>{{ t('admin.conversations.provider') }}: {{ turn.provider || '-' }}</span>
              <span>{{ turn.stream ? t('admin.conversations.stream') : t('admin.conversations.nonStream') }}</span>
              <span :class="turn.status_code >= 400 ? 'text-red-600 dark:text-red-400' : 'text-emerald-600 dark:text-emerald-400'">
                {{ t('admin.conversations.status') }}: {{ turn.status_code }}
              </span>
            </div>

            <div class="space-y-4">
              <div class="flex justify-end">
                <div class="max-w-[88%] rounded-2xl rounded-br-md bg-primary-600 px-4 py-3 text-white shadow-sm">
                  <p class="mb-1 text-xs font-semibold text-primary-100">{{ t('admin.conversations.userRole') }}</p>
                  <p class="whitespace-pre-wrap break-words text-sm leading-6">{{ turn.request_text || t('admin.conversations.emptyRequest') }}</p>
                </div>
              </div>
              <div class="flex justify-start">
                <div class="max-w-[88%] rounded-2xl rounded-bl-md bg-gray-100 px-4 py-3 text-gray-900 shadow-sm dark:bg-dark-700 dark:text-gray-100">
                  <p class="mb-1 text-xs font-semibold text-gray-500 dark:text-gray-400">{{ t('admin.conversations.assistantRole') }}</p>
                  <p class="whitespace-pre-wrap break-words text-sm leading-6">{{ responseText(turn) || t('admin.conversations.emptyResponse') }}</p>
                </div>
              </div>
            </div>

            <div class="mt-4 grid grid-cols-1 gap-3 xl:grid-cols-2">
              <details class="rounded-lg border border-gray-200 dark:border-dark-600">
                <summary class="cursor-pointer px-3 py-2 text-sm font-medium text-gray-700 dark:text-gray-200">{{ t('admin.conversations.rawRequest') }}</summary>
                <div v-if="turn.request_truncated" class="mx-3 mb-2 rounded bg-amber-50 px-3 py-2 text-xs text-amber-700 dark:bg-amber-950/30 dark:text-amber-300">{{ t('admin.conversations.truncated') }}</div>
                <pre class="max-h-96 overflow-auto whitespace-pre-wrap break-all border-t border-gray-100 bg-gray-50 p-3 text-xs text-gray-700 dark:border-dark-700 dark:bg-dark-900 dark:text-gray-300">{{ prettyPayload(turn.request_body) }}</pre>
              </details>
              <details class="rounded-lg border border-gray-200 dark:border-dark-600">
                <summary class="cursor-pointer px-3 py-2 text-sm font-medium text-gray-700 dark:text-gray-200">{{ t('admin.conversations.rawResponse') }}</summary>
                <div class="px-3 pb-2 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.conversations.contentType') }}: {{ turn.content_type || '-' }}</div>
                <div v-if="turn.response_truncated" class="mx-3 mb-2 rounded bg-amber-50 px-3 py-2 text-xs text-amber-700 dark:bg-amber-950/30 dark:text-amber-300">{{ t('admin.conversations.truncated') }}</div>
                <pre class="max-h-96 overflow-auto whitespace-pre-wrap break-all border-t border-gray-100 bg-gray-50 p-3 text-xs text-gray-700 dark:border-dark-700 dark:bg-dark-900 dark:text-gray-300">{{ prettyPayload(turn.response_body) }}</pre>
              </details>
            </div>

            <p class="mt-3 break-all font-mono text-[11px] text-gray-400">{{ t('admin.conversations.requestId') }}: {{ turn.request_id || '-' }}</p>
          </article>
        </div>
      </div>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import Pagination from '@/components/common/Pagination.vue'
import Toggle from '@/components/common/Toggle.vue'
import GroupSelector from '@/components/common/GroupSelector.vue'
import Icon from '@/components/icons/Icon.vue'
import ConversationAPIKeySelector from '@/views/admin/conversations/ConversationAPIKeySelector.vue'
import conversationsAPI, { type ConversationRecordConfig, type ConversationSessionSummary, type ConversationTurn } from '@/api/admin/conversations'
import { groupsAPI } from '@/api/admin'
import type { AdminGroup } from '@/types'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { extractConversationResponseText } from '@/utils/conversationResponse'
import { formatDateTime } from '@/utils/format'

const { t } = useI18n()
const appStore = useAppStore()

const MetadataItem = defineComponent({
  props: {
    label: { type: String, required: true },
    value: { type: String, required: true },
    mono: { type: Boolean, default: false }
  },
  setup(props) {
    return () => h('div', { class: 'min-w-0' }, [
      h('p', { class: 'text-xs text-gray-500 dark:text-gray-400' }, props.label),
      h('p', { class: ['mt-1 truncate text-gray-900 dark:text-white', props.mono ? 'font-mono text-xs' : 'font-medium'] }, props.value)
    ])
  }
})

const sessions = ref<ConversationSessionSummary[]>([])
const turns = ref<ConversationTurn[]>([])
const selectedSession = ref<ConversationSessionSummary | null>(null)
const loading = ref(false)
const detailLoading = ref(false)
const error = ref('')
const detailError = ref('')
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)
const filters = reactive({ search: '', model: '', from: '', to: '' })
const groups = ref<AdminGroup[]>([])
const configForm = reactive<ConversationRecordConfig>({ enabled: true, group_ids: [], api_key_ids: [] })
const configLoading = ref(true)
const configSaving = ref(false)
const configReady = ref(false)
const configError = ref('')
let listController: AbortController | null = null
let detailController: AbortController | null = null

const detailTitle = computed(() => {
  if (!selectedSession.value) return t('admin.conversations.detailTitle')
  const identity = selectedSession.value.username || selectedSession.value.user_email || `#${selectedSession.value.user_id}`
  return `${t('admin.conversations.detailTitle')} · ${identity}`
})

function applyConfig(config: ConversationRecordConfig): void {
  configForm.enabled = config.enabled
  configForm.group_ids = Array.isArray(config.group_ids) ? [...config.group_ids] : []
  configForm.api_key_ids = Array.isArray(config.api_key_ids) ? [...config.api_key_ids] : []
}

async function loadConfig(): Promise<void> {
  configLoading.value = true
  configReady.value = false
  configError.value = ''
  try {
    const config = await conversationsAPI.getConfig()
    applyConfig(config)
    configReady.value = true
  } catch (cause) {
    configError.value = extractApiErrorMessage(cause, t('admin.conversations.config.loadFailed'))
    configLoading.value = false
    return
  }
  try {
    groups.value = await groupsAPI.getAllIncludingInactive()
  } catch (cause) {
    configError.value = extractApiErrorMessage(cause, t('admin.conversations.config.groupsLoadFailed'))
  } finally {
    configLoading.value = false
  }
}

async function saveConfig(): Promise<void> {
  configSaving.value = true
  configError.value = ''
  try {
    const config = await conversationsAPI.updateConfig({
      enabled: configForm.enabled,
      group_ids: [...new Set(configForm.group_ids)].sort((a, b) => a - b),
      api_key_ids: [...new Set(configForm.api_key_ids)].sort((a, b) => a - b)
    })
    applyConfig(config)
    appStore.showSuccess(t('admin.conversations.config.saved'))
  } catch (cause) {
    const message = extractApiErrorMessage(cause, t('admin.conversations.config.saveFailed'))
    configError.value = message
    appStore.showError(message)
  } finally {
    configSaving.value = false
  }
}

async function loadSessions(): Promise<void> {
  listController?.abort()
  listController = new AbortController()
  loading.value = true
  error.value = ''
  try {
    const result = await conversationsAPI.list({
      page: page.value,
      page_size: pageSize.value,
      search: filters.search || undefined,
      model: filters.model || undefined,
      from: filters.from || undefined,
      to: filters.to || undefined
    }, { signal: listController.signal })
    sessions.value = result.items
    total.value = result.total
  } catch (cause) {
    if (!isCanceled(cause)) error.value = errorMessage(cause, t('admin.conversations.loadFailed'))
  } finally {
    loading.value = false
  }
}

async function openDetails(session: ConversationSessionSummary): Promise<void> {
  selectedSession.value = session
  turns.value = []
  detailError.value = ''
  detailLoading.value = true
  detailController?.abort()
  detailController = new AbortController()
  try {
    turns.value = await conversationsAPI.get(session.conversation_key, { signal: detailController.signal })
  } catch (cause) {
    if (!isCanceled(cause)) detailError.value = errorMessage(cause, t('admin.conversations.detailFailed'))
  } finally {
    detailLoading.value = false
  }
}

function closeDetails(): void {
  detailController?.abort()
  selectedSession.value = null
  turns.value = []
  detailError.value = ''
}

function applyFilters(): void {
  page.value = 1
  void loadSessions()
}

function resetFilters(): void {
  filters.search = ''
  filters.model = ''
  filters.from = ''
  filters.to = ''
  applyFilters()
}

function changePage(value: number): void {
  page.value = value
  void loadSessions()
}

function changePageSize(value: number): void {
  pageSize.value = value
  page.value = 1
  void loadSessions()
}

function responseText(turn: ConversationTurn): string {
  return extractConversationResponseText(turn.protocol, turn.content_type, turn.response_body)
}

function prettyPayload(value: string): string {
  const trimmed = value.trim()
  if (!trimmed) return ''
  try {
    return JSON.stringify(JSON.parse(trimmed), null, 2)
  } catch {
    return value
  }
}

function isCanceled(cause: unknown): boolean {
  return typeof cause === 'object' && cause !== null && ('code' in cause) && (cause as { code?: string }).code === 'ERR_CANCELED'
}

function errorMessage(cause: unknown, fallback: string): string {
  if (typeof cause === 'object' && cause !== null) {
    const value = cause as { message?: string; detail?: string }
    return value.message || value.detail || fallback
  }
  return fallback
}

onMounted(() => {
  void loadConfig()
  void loadSessions()
})
onBeforeUnmount(() => {
  listController?.abort()
  detailController?.abort()
})
</script>
