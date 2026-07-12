<script setup lang="ts">
import type { ModerationPolicy } from '@/types'

defineProps<{
  policies: readonly ModerationPolicy[]
  loading: boolean
}>()

function formatThreshold(value: number): string {
  return `${Math.round(value * 100)}%`
}
</script>

<template>
  <section class="policy-catalog" aria-labelledby="policy-catalog-title">
    <div class="catalog-heading">
      <div>
        <span>只读策略目录</span>
        <h3 id="policy-catalog-title">可分配审核策略</h3>
      </div>
      <small>阈值由服务端配置管理</small>
    </div>
    <div v-if="loading" class="catalog-state" role="status">正在加载策略目录…</div>
    <div v-else-if="policies.length === 0" class="catalog-state">当前没有可分配策略。</div>
    <ul v-else class="policy-grid">
      <li v-for="policy in policies" :key="policy.version">
        <div class="policy-name">
          <strong>{{ policy.version }}</strong>
          <span v-if="policy.default">系统默认</span>
        </div>
        <dl>
          <div><dt>进入复核</dt><dd>≥ {{ formatThreshold(policy.review_threshold) }}</dd></div>
          <div><dt>自动阻断</dt><dd>≥ {{ formatThreshold(policy.block_threshold) }}</dd></div>
        </dl>
      </li>
    </ul>
  </section>
</template>

<style scoped>
@reference "../../styles.css";

.policy-catalog { @apply overflow-hidden rounded-2xl border border-[#d8d8cd] bg-[#eef1e8] shadow-sm; }
.catalog-heading { @apply flex items-end justify-between gap-4 border-b border-[#d8d8cd] px-4 py-4 md:px-5; }
.catalog-heading span { @apply text-[0.68rem] font-bold uppercase tracking-[0.16em] text-[#708074]; }
.catalog-heading h3 { @apply mt-1 text-sm font-bold text-[#263129]; }
.catalog-heading small { @apply text-right text-xs text-[#758078]; }
.catalog-state { @apply p-5 text-sm text-[#687169]; }
.policy-grid { @apply grid gap-3 p-3 md:grid-cols-2 xl:grid-cols-3; }
.policy-grid li { @apply rounded-xl border border-[#d4d8cd] bg-white p-4; }
.policy-name { @apply flex items-center justify-between gap-3; }
.policy-name strong { @apply min-w-0 truncate text-sm text-[#263129]; }
.policy-name span { @apply shrink-0 rounded-full bg-[#deedda] px-2 py-1 text-[0.68rem] font-bold text-[#37633f]; }
.policy-grid dl { @apply mt-3 grid grid-cols-2 gap-3; }
.policy-grid dl div { @apply rounded-lg bg-[#f6f5ef] p-2; }
.policy-grid dt { @apply text-[0.68rem] text-[#758078]; }
.policy-grid dd { @apply mt-1 text-xs font-bold text-[#3f4d43]; }
</style>
