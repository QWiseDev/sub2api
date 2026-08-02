<template>
  <div ref="containerRef" class="relative">
    <label class="input-label">
      {{ t('admin.conversations.config.apiKeys') }}
      <span class="font-normal text-gray-400">{{ t('common.selectedCount', { count: selectedIDs.length }) }}</span>
    </label>

    <div v-if="selectedIDs.length > 0" class="mb-2 flex flex-wrap gap-2">
      <span
        v-for="id in selectedIDs"
        :key="id"
        class="inline-flex max-w-full items-center gap-1.5 rounded-md bg-gray-100 px-2.5 py-1.5 text-xs text-gray-700 dark:bg-dark-600 dark:text-gray-200"
      >
        <span class="max-w-64 truncate font-medium" :title="keyLabel(id)">{{ keyLabel(id) }}</span>
        <span class="shrink-0 text-gray-400">#{{ id }}</span>
        <button
          type="button"
          class="shrink-0 rounded text-gray-400 hover:text-red-600 dark:hover:text-red-400"
          :aria-label="t('admin.conversations.config.removeAPIKey')"
          :title="t('admin.conversations.config.removeAPIKey')"
          @click="removeKey(id)"
        >
          <Icon name="x" size="xs" :stroke-width="2" />
        </button>
      </span>
    </div>

    <div class="relative">
      <Icon name="search" size="sm" class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
      <input
        v-model="searchQuery"
        type="text"
        autocomplete="off"
        class="input w-full pl-9"
        :placeholder="t('admin.conversations.config.apiKeySearchPlaceholder')"
        @input="scheduleSearch"
        @focus="showDropdown = true"
      />
    </div>

    <div
      v-if="showDropdown && searchQuery.trim()"
      class="absolute z-50 mt-1 max-h-60 w-full overflow-auto rounded-lg border border-gray-200 bg-white shadow-lg dark:border-dark-600 dark:bg-dark-700"
    >
      <div v-if="searchLoading" class="px-4 py-3 text-sm text-gray-500 dark:text-gray-400">
        {{ t('common.loading') }}
      </div>
      <div v-else-if="availableResults.length === 0" class="px-4 py-3 text-sm text-gray-500 dark:text-gray-400">
        {{ t('admin.conversations.config.apiKeySearchEmpty') }}
      </div>
      <template v-else>
        <button
          v-for="key in availableResults"
          :key="key.id"
          type="button"
          class="flex w-full items-center justify-between gap-3 px-4 py-2 text-left text-sm hover:bg-gray-100 dark:hover:bg-dark-600"
          @click="selectKey(key)"
        >
          <span class="min-w-0 truncate font-medium text-gray-900 dark:text-white">{{ key.name || `#${key.id}` }}</span>
          <span class="shrink-0 text-xs text-gray-400">#{{ key.id }} · {{ t('admin.conversations.config.apiKeyUser', { id: key.user_id }) }}</span>
        </button>
      </template>
    </div>

    <p class="mt-2 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.conversations.config.apiKeysHint') }}</p>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { SimpleApiKey } from '@/api/admin/usage'
import Icon from '@/components/icons/Icon.vue'

const props = defineProps<{
  modelValue: number[]
}>()

const emit = defineEmits<{
  'update:modelValue': [value: number[]]
}>()

const { t } = useI18n()
const containerRef = ref<HTMLElement | null>(null)
const searchQuery = ref('')
const searchResults = ref<SimpleApiKey[]>([])
const selectedKeys = ref<Record<number, SimpleApiKey>>({})
const searchLoading = ref(false)
const showDropdown = ref(false)
let searchTimer: ReturnType<typeof setTimeout> | null = null
let searchSequence = 0
let hydrated = false

const selectedIDs = computed(() =>
  Array.from(new Set(props.modelValue.filter((id) => Number.isInteger(id) && id > 0))).sort((a, b) => a - b)
)

const availableResults = computed(() => {
  const selected = new Set(selectedIDs.value)
  return searchResults.value.filter((key) => !selected.has(key.id))
})

function keyLabel(id: number): string {
  return selectedKeys.value[id]?.name || t('admin.conversations.config.apiKeyFallback', { id })
}

function clearSearchTimer(): void {
  if (searchTimer) {
    clearTimeout(searchTimer)
    searchTimer = null
  }
  searchSequence += 1
}

function scheduleSearch(): void {
  clearSearchTimer()
  const query = searchQuery.value.trim()
  showDropdown.value = true
  if (!query) {
    searchResults.value = []
    searchLoading.value = false
    return
  }

  const sequence = searchSequence
  searchTimer = setTimeout(async () => {
    searchLoading.value = true
    try {
      const results = await adminAPI.usage.searchApiKeys(undefined, query)
      if (sequence !== searchSequence) return
      searchResults.value = results
      const next = { ...selectedKeys.value }
      for (const key of results) next[key.id] = key
      selectedKeys.value = next
    } catch {
      if (sequence === searchSequence) searchResults.value = []
    } finally {
      if (sequence === searchSequence) searchLoading.value = false
    }
  }, 300)
}

function selectKey(key: SimpleApiKey): void {
  selectedKeys.value = { ...selectedKeys.value, [key.id]: key }
  emit('update:modelValue', [...selectedIDs.value, key.id])
  clearSearchTimer()
  searchQuery.value = ''
  searchResults.value = []
  searchLoading.value = false
  showDropdown.value = false
}

function removeKey(id: number): void {
  emit('update:modelValue', selectedIDs.value.filter((value) => value !== id))
}

async function hydrateSelectedKeys(): Promise<void> {
  if (hydrated || selectedIDs.value.length === 0) return
  hydrated = true
  try {
    const results = await adminAPI.usage.searchApiKeys()
    const selected = new Set(selectedIDs.value)
    const next = { ...selectedKeys.value }
    for (const key of results) {
      if (selected.has(key.id)) next[key.id] = key
    }
    selectedKeys.value = next
  } catch {
    // ID fallback remains usable when metadata lookup fails.
  }
}

function handleDocumentClick(event: MouseEvent): void {
  const target = event.target as Node | null
  if (target && !containerRef.value?.contains(target)) showDropdown.value = false
}

watch(selectedIDs, () => void hydrateSelectedKeys(), { immediate: true })

onMounted(() => document.addEventListener('click', handleDocumentClick))
onUnmounted(() => {
  clearSearchTimer()
  document.removeEventListener('click', handleDocumentClick)
})
</script>
