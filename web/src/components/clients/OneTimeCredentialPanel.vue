<script setup lang="ts">
import { shallowRef } from 'vue'

import type { OneTimeCredential } from '@/composables/useClients'

const props = defineProps<{ credential: OneTimeCredential }>()
const emit = defineEmits<{ close: [] }>()
const copyStatus = shallowRef('')

async function copyKey(): Promise<void> {
  try {
    await navigator.clipboard.writeText(props.credential.apiKey)
    copyStatus.value = '已复制到剪贴板。'
  } catch {
    copyStatus.value = '复制失败，请手动选择并复制。'
  }
}
</script>

<template>
  <section class="credential-panel" aria-labelledby="credential-title">
    <div class="credential-heading">
      <div>
        <span class="eyebrow">一次性凭证</span>
        <h3 id="credential-title">
          {{ credential.kind === 'created' ? '客户端已创建' : 'API Key 已轮换' }}
        </h3>
      </div>
      <button type="button" class="close-button" aria-label="关闭一次性 API Key" @click="emit('close')">
        关闭
      </button>
    </div>
    <p><strong>{{ credential.clientName }}</strong> 的完整 API Key 仅在此处显示一次。</p>
    <code data-testid="one-time-api-key">{{ credential.apiKey }}</code>
    <div class="credential-actions">
      <button type="button" class="copy-button" @click="copyKey">复制 API Key</button>
      <span role="status">{{ copyStatus }}</span>
    </div>
    <p class="warning">请立即复制并安全保存。关闭后无法再次查看；如丢失，只能轮换 API Key。</p>
  </section>
</template>

<style scoped>
@reference "../../styles.css";

.credential-panel { @apply rounded-2xl border border-[#d5a44e] bg-[#fff6dd] p-5 shadow-sm; }
.credential-heading { @apply flex items-start justify-between gap-4; }
.eyebrow { @apply text-[0.68rem] font-bold uppercase tracking-[0.16em] text-[#8a5a13]; }
.credential-heading h3 { @apply mt-1 text-lg font-bold text-[#4f3717]; }
.close-button { @apply min-h-11 rounded-lg border border-[#d7b878] px-3 text-xs font-bold text-[#674817] hover:bg-white/50 focus:outline-none focus:ring-4 focus:ring-[#d5a44e]/20; }
.credential-panel p { @apply text-sm leading-6 text-[#604b2a]; }
.credential-panel code { @apply mt-3 block overflow-x-auto rounded-xl border border-[#e0c58e] bg-white p-4 font-mono text-sm text-[#25332a]; }
.credential-actions { @apply mt-3 flex flex-wrap items-center gap-3; }
.copy-button { @apply min-h-11 rounded-xl bg-[#31593a] px-4 text-sm font-bold text-white hover:bg-[#26492f] focus:outline-none focus:ring-4 focus:ring-[#456b4d]/20; }
.credential-actions span { @apply text-xs font-semibold text-[#604b2a]; }
.credential-panel .warning { @apply mt-4 border-t border-[#e0c58e] pt-3 text-xs font-bold; }
</style>
