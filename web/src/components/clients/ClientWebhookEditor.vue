<script setup lang="ts">
import { computed, shallowRef, watch } from 'vue'

import type { ClientApplication } from '@/types'

const props = defineProps<{
  client: ClientApplication
  busy: boolean
  credentialOpen: boolean
}>()

const emit = defineEmits<{
  save: [client: ClientApplication, webhookURL: string]
  close: []
}>()

const webhookURL = shallowRef(props.client.webhook_url || '')
const confirmingClear = shallowRef(false)
const validationError = shallowRef('')
const normalizedURL = computed(() => webhookURL.value.trim())
const changed = computed(() => normalizedURL.value !== (props.client.webhook_url || ''))
const canSave = computed(() => Boolean(normalizedURL.value) && (changed.value || Boolean(props.client.webhook_url)))

watch(
  () => props.client.webhook_url,
  (value) => {
    webhookURL.value = value || ''
    confirmingClear.value = false
    validationError.value = ''
  },
)

function submit(): void {
  validationError.value = ''
  if (!canSave.value || props.busy || props.credentialOpen) return
  if (!normalizedURL.value.startsWith('https://')) {
    validationError.value = 'Webhook URL 必须使用 HTTPS。'
    return
  }
  emit('save', props.client, normalizedURL.value)
}

function clearWebhook(): void {
  validationError.value = ''
  if (props.busy || props.credentialOpen || !props.client.webhook_url) return
  if (!confirmingClear.value) {
    confirmingClear.value = true
    return
  }
  confirmingClear.value = false
  emit('save', props.client, '')
}
</script>

<template>
  <section class="webhook-editor" aria-labelledby="webhook-editor-title">
    <div class="editor-heading">
      <div>
        <span>敏感配置</span>
        <h3 id="webhook-editor-title">配置 {{ client.name }} 的 Webhook</h3>
      </div>
      <button type="button" class="close-button" aria-label="关闭 Webhook 配置" :disabled="busy" @click="emit('close')">关闭</button>
    </div>
    <form @submit.prevent="submit">
      <label :for="`client-webhook-${client.id}`">HTTPS 回调地址</label>
      <div class="url-actions">
        <input
          :id="`client-webhook-${client.id}`"
          v-model="webhookURL"
          type="url"
          inputmode="url"
          maxlength="500"
          placeholder="https://example.com/moderation/webhook"
          :aria-invalid="Boolean(validationError)"
          :aria-describedby="validationError ? `client-webhook-error-${client.id}` : `client-webhook-help-${client.id}`"
          :disabled="busy || credentialOpen"
        />
        <button type="submit" class="save-button" :disabled="busy || credentialOpen || !canSave">
          {{ busy ? '保存中…' : client.webhook_url ? '更新并轮换 secret' : '配置 Webhook' }}
        </button>
      </div>
      <p :id="`client-webhook-help-${client.id}`" class="help-text">
        每次保存非空地址都会生成新的签名 secret，旧 secret 立即失效。仅支持 HTTPS；保存时拒绝明显的本地或私网地址，投递时会重新解析并拦截私网目标。
      </p>
      <p v-if="validationError" :id="`client-webhook-error-${client.id}`" class="validation-error" role="alert">
        {{ validationError }}
      </p>
    </form>
    <div v-if="client.webhook_url" class="clear-zone">
      <div>
        <strong>停止回调</strong>
        <span>清除地址会停止后续回调，并立即废止当前签名 secret。</span>
      </div>
      <button
        type="button"
        class="clear-button"
        :class="{ confirming: confirmingClear }"
        :disabled="busy || credentialOpen"
        @click="clearWebhook"
      >{{ confirmingClear ? '确认清除 Webhook' : '清除 Webhook' }}</button>
      <button v-if="confirmingClear" type="button" class="cancel-button" @click="confirmingClear = false">取消</button>
    </div>
  </section>
</template>

<style scoped>
@reference "../../styles.css";

.webhook-editor { @apply rounded-2xl border border-[#d4c4a5] bg-[#fffdf7] p-4 shadow-sm md:p-5; }
.editor-heading { @apply flex items-start justify-between gap-4; }
.editor-heading span { @apply text-[0.68rem] font-bold uppercase tracking-[0.16em] text-[#8a6a35]; }
.editor-heading h3 { @apply mt-1 text-base font-bold text-[#352e22]; }
.close-button { @apply min-h-11 rounded-lg border border-[#d5d0c3] px-3 text-xs font-bold text-[#5f625d] focus:outline-none focus:ring-4 focus:ring-[#456b4d]/15; }
.webhook-editor form { @apply mt-4; }
.webhook-editor label { @apply text-xs font-bold text-[#445047]; }
.url-actions { @apply mt-2 grid gap-2 lg:grid-cols-[minmax(0,1fr)_auto]; }
.url-actions input { @apply min-h-11 min-w-0 rounded-xl border border-[#c7cabf] bg-white px-3 text-sm outline-none focus:border-[#62806a] focus:ring-4 focus:ring-[#456b4d]/15 disabled:opacity-50; }
.save-button { @apply min-h-11 rounded-xl bg-[#31593a] px-4 text-sm font-bold text-white hover:bg-[#26492f] focus:outline-none focus:ring-4 focus:ring-[#456b4d]/20 disabled:cursor-not-allowed disabled:opacity-50; }
.help-text { @apply mt-2 text-xs leading-5 text-[#6e746e]; }
.validation-error { @apply mt-2 text-xs font-bold text-[#8b392f]; }
.clear-zone { @apply mt-5 grid gap-3 border-t border-[#e1ddd2] pt-4 md:grid-cols-[minmax(0,1fr)_auto_auto] md:items-center; }
.clear-zone div { @apply flex flex-col gap-1; }
.clear-zone strong { @apply text-xs text-[#6f392f]; }
.clear-zone span { @apply text-xs leading-5 text-[#756d64]; }
.clear-zone button { @apply min-h-11 rounded-lg border px-3 text-xs font-bold focus:outline-none focus:ring-4 focus:ring-[#9b4f40]/15 disabled:opacity-50; }
.clear-button { @apply border-[#d5a99f] bg-[#fff4f1] text-[#8b392f]; }
.clear-button.confirming { @apply border-[#a95747] bg-[#f7ded8]; }
.cancel-button { @apply border-[#d5d0c3] bg-white text-[#5f625d]; }
</style>
