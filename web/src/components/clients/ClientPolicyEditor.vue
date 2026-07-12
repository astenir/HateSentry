<script setup lang="ts">
import { computed, shallowRef, watch } from 'vue'

import type { ClientApplication, ModerationPolicy } from '@/types'

const props = defineProps<{
  client: ClientApplication
  policies: readonly ModerationPolicy[]
  busy: boolean
}>()

const emit = defineEmits<{
  assign: [client: ClientApplication, policyVersion: string]
}>()

const selectedVersion = shallowRef(props.client.policy_version || '')
const changed = computed(() => selectedVersion.value !== (props.client.policy_version || ''))
const defaultPolicy = computed(() => props.policies.find((policy) => policy.default))

watch(
  () => props.client.policy_version,
  (version) => { selectedVersion.value = version || '' },
)

function submit(): void {
  if (!changed.value || props.busy) return
  emit('assign', props.client, selectedVersion.value)
}
</script>

<template>
  <form class="policy-editor" @submit.prevent="submit">
    <label :for="`client-policy-${client.id}`" class="sr-only">
      {{ client.name }} 的审核策略
    </label>
    <select
      :id="`client-policy-${client.id}`"
      v-model="selectedVersion"
      :disabled="busy || policies.length === 0"
    >
      <option value="">
        跟随系统默认{{ defaultPolicy ? `（${defaultPolicy.version}）` : '' }}
      </option>
      <option v-for="policy in policies" :key="policy.version" :value="policy.version">
        {{ policy.version }}{{ policy.default ? '（默认版本）' : '' }}
      </option>
    </select>
    <button
      type="submit"
      :aria-label="selectedVersion ? `为 ${client.name} 应用策略` : `将 ${client.name} 恢复为跟随默认策略`"
      :disabled="busy || !changed"
    >
      {{ busy ? '保存中…' : selectedVersion ? '应用策略' : '恢复默认' }}
    </button>
  </form>
</template>

<style scoped>
@reference "../../styles.css";

.policy-editor { @apply flex min-w-0 flex-col gap-2; }
.policy-editor select { @apply min-h-11 min-w-0 w-full rounded-lg border border-[#c7cabf] bg-white px-2 text-xs text-[#334037] outline-none focus:border-[#62806a] focus:ring-4 focus:ring-[#456b4d]/15 disabled:opacity-50; }
.policy-editor button { @apply min-h-11 rounded-lg border border-[#9db09d] bg-[#edf3e9] px-2 text-xs font-bold text-[#31593a] hover:bg-[#e2eddd] focus:outline-none focus:ring-4 focus:ring-[#456b4d]/15 disabled:cursor-not-allowed disabled:opacity-50; }
</style>
