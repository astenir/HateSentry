<script setup lang="ts">
import { shallowRef } from 'vue'

defineProps<{ busy: boolean, locked?: boolean }>()

const emit = defineEmits<{ submit: [name: string] }>()
const name = shallowRef('')

function submit(): void {
  const normalized = name.value.trim()
  if (!normalized) return
  emit('submit', normalized)
  name.value = ''
}
</script>

<template>
  <form class="create-form" @submit.prevent="submit">
    <div>
      <label for="client-name">客户端名称</label>
      <span>用于识别接入审核 API 的应用。</span>
    </div>
    <input
      id="client-name"
      v-model="name"
      name="client-name"
      type="text"
      maxlength="100"
      autocomplete="off"
      placeholder="例如：blog-comments"
      required
      :disabled="busy || locked"
    />
    <button type="submit" :disabled="busy || locked || !name.trim()">
      {{ locked ? '请先关闭当前 Key' : busy ? '正在创建…' : '创建客户端' }}
    </button>
  </form>
</template>

<style scoped>
@reference "../../styles.css";

.create-form {
  @apply grid gap-3 rounded-2xl border border-[#d8d8cd] bg-white p-4 shadow-sm md:grid-cols-[minmax(12rem,1fr)_minmax(16rem,2fr)_auto] md:items-end md:p-5;
}

.create-form div { @apply flex flex-col gap-1; }
.create-form label { @apply text-sm font-bold text-[#263129]; }
.create-form span { @apply text-xs leading-5 text-[#758078]; }
.create-form input { @apply min-h-11 min-w-0 rounded-xl border border-[#c7cabf] bg-[#fbfaf5] px-3 text-sm outline-none focus:border-[#62806a] focus:ring-4 focus:ring-[#456b4d]/15 disabled:opacity-60; }
.create-form button { @apply min-h-11 rounded-xl bg-[#31593a] px-5 text-sm font-bold text-white transition hover:bg-[#26492f] focus:outline-none focus:ring-4 focus:ring-[#456b4d]/20 disabled:cursor-not-allowed disabled:opacity-50; }
</style>
