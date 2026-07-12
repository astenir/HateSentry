<script setup lang="ts">
import { reactive, watch } from 'vue'

import type { ReviewActionInput, ReviewCase } from '@/types'

const props = withDefaults(defineProps<{
  item: ReviewCase | null
  loading: boolean
  busy: boolean
  context?: 'queue' | 'history'
}>(), {
  context: 'queue',
})

const emit = defineEmits<{
  action: [input: ReviewActionInput]
}>()

const form = reactive({
  notes: '',
  mistakeDecision: '' as '' | 'allow' | 'block',
  validationError: '',
})

const dateFormatter = new Intl.DateTimeFormat('zh-CN', {
  year: 'numeric',
  month: '2-digit',
  day: '2-digit',
  hour: '2-digit',
  minute: '2-digit',
})

function formatDate(value: string): string {
  return dateFormatter.format(new Date(value))
}

watch(
  () => props.item?.id,
  () => {
    form.notes = ''
    form.mistakeDecision = ''
    form.validationError = ''
  },
)

function submit(action: ReviewActionInput['action']): void {
  if (action === 'mark-mistake' && !form.mistakeDecision) {
    form.validationError = '标记误判时，请选择人工最终决定。'
    return
  }

  form.validationError = ''
  emit('action', {
    action,
    notes: form.notes.trim(),
    finalDecision: form.mistakeDecision || undefined,
  })
}
</script>

<template>
  <section class="detail-panel" aria-labelledby="detail-title">
    <div v-if="loading" class="detail-state" role="status">正在加载案件详情…</div>
    <div v-else-if="!item" class="detail-placeholder">
      <div class="placeholder-orbit" aria-hidden="true">
        <span></span>
      </div>
      <p class="section-kicker">REVIEW WORKSPACE</p>
      <h2 id="detail-title">
        {{ context === 'history' ? '选择一条审核历史' : '选择一条待复核内容' }}
      </h2>
      <p v-if="context === 'history'">在左侧选择已处理案件，查看策略判断和人工复核记录。</p>
      <p v-else>在左侧队列中选择案件，查看策略依据并记录人工最终决定。</p>
    </div>
    <div v-else class="detail-content">
      <header class="detail-heading">
        <div>
          <p class="section-kicker">CASE #{{ item.id }}</p>
          <h2 id="detail-title">复核内容</h2>
        </div>
        <span class="status-badge" :class="`status-${item.status}`">{{ item.status }}</span>
      </header>

      <article class="content-card">
        <p class="content-text">{{ item.content }}</p>
        <dl class="metadata-grid">
          <div>
            <dt>来源</dt>
            <dd>{{ item.source || '—' }}</dd>
          </div>
          <div>
            <dt>外部 ID</dt>
            <dd>{{ item.external_id || '—' }}</dd>
          </div>
          <div>
            <dt>Actor</dt>
            <dd>{{ item.actor_id || '—' }}</dd>
          </div>
          <div>
            <dt>请求 ID</dt>
            <dd class="mono-value">{{ item.request_id }}</dd>
          </div>
        </dl>
      </article>

      <div class="assessment-grid">
        <section class="assessment-card">
          <p class="section-kicker">POLICY SIGNAL</p>
          <div class="risk-row">
            <strong>{{ Math.round(item.risk_score * 100) }}</strong>
            <span>/ 100 风险</span>
          </div>
          <div class="risk-track" aria-hidden="true">
            <span :style="{ width: `${Math.round(item.risk_score * 100)}%` }"></span>
          </div>
          <div class="label-list" aria-label="风险标签">
            <span v-for="label in item.labels" :key="label">{{ label }}</span>
          </div>
        </section>

        <section class="assessment-card">
          <p class="section-kicker">DECISION TRACE</p>
          <dl class="decision-list">
            <div>
              <dt>策略决定</dt>
              <dd>{{ item.policy_decision }}</dd>
            </div>
            <div>
              <dt>策略版本</dt>
              <dd>{{ item.policy_version }}</dd>
            </div>
            <div v-if="item.final_decision">
              <dt>人工决定</dt>
              <dd>{{ item.final_decision }}</dd>
            </div>
          </dl>
        </section>
      </div>

      <section class="reason-card">
        <p class="section-kicker">AI SUGGESTION</p>
        <h3>AI 建议依据</h3>
        <p>{{ item.reason }}</p>
      </section>

      <section v-if="item.status === 'pending'" class="action-card">
        <div>
          <label class="field-label" for="review-notes">复核备注</label>
          <textarea
            id="review-notes"
            v-model="form.notes"
            class="notes-input"
            rows="3"
            maxlength="2000"
            placeholder="记录判断依据，便于后续审计（可选）"
            :disabled="busy"
          ></textarea>
        </div>

        <fieldset class="mistake-fieldset">
          <legend>标记误判时的人工最终决定</legend>
          <label>
            <input v-model="form.mistakeDecision" type="radio" value="allow" :disabled="busy" />
            允许
          </label>
          <label>
            <input v-model="form.mistakeDecision" type="radio" value="block" :disabled="busy" />
            阻断
          </label>
        </fieldset>

        <p v-if="form.validationError" class="validation-error" role="alert">
          {{ form.validationError }}
        </p>

        <div class="action-row">
          <button class="action-button action-allow" type="button" :disabled="busy" @click="submit('approve')">
            通过并允许
          </button>
          <button class="action-button action-block" type="button" :disabled="busy" @click="submit('reject')">
            拒绝并阻断
          </button>
          <button class="action-button action-mistake" type="button" :disabled="busy" @click="submit('mark-mistake')">
            标记误判
          </button>
        </div>
      </section>

      <section v-else class="completed-card">
        <strong>该案件已经处理</strong>
        <p>人工最终决定：{{ item.final_decision || '—' }}</p>
        <p>复核人：{{ item.reviewer_id ? `操作员 #${item.reviewer_id}` : '—' }}</p>
        <p v-if="item.reviewed_at">
          复核时间：<time :datetime="item.reviewed_at">{{ formatDate(item.reviewed_at) }}</time>
        </p>
        <p v-if="item.review_notes">备注：{{ item.review_notes }}</p>
      </section>
    </div>
  </section>
</template>

<style scoped>
@reference "../../styles.css";

.detail-panel {
  @apply min-h-0 overflow-y-auto bg-[#fbfaf5];
}

.detail-state {
  @apply grid min-h-72 place-items-center p-8 text-sm text-[#687169];
}

.detail-placeholder {
  @apply flex min-h-[30rem] flex-col items-center justify-center px-8 text-center text-[#69736b];
}

.detail-placeholder h2 {
  @apply mt-4 text-2xl font-semibold tracking-[-0.03em] text-[#1f2b22];
}

.detail-placeholder > p:last-child {
  @apply mt-3 max-w-md text-sm leading-6;
}

.placeholder-orbit {
  @apply relative grid h-24 w-24 place-items-center rounded-full border border-[#cbd0c5];
}

.placeholder-orbit::before {
  content: '';
  @apply absolute h-16 w-16 rounded-full border border-dashed border-[#9ead9c];
}

.placeholder-orbit span {
  @apply h-3 w-3 rounded-full bg-[#d9983c] shadow-[0_0_0_8px_rgba(217,152,60,0.12)];
}

.detail-content {
  @apply mx-auto max-w-5xl p-5 md:p-8 lg:p-10;
}

.detail-heading {
  @apply flex items-start justify-between gap-4;
}

.detail-heading h2 {
  @apply mt-1 text-3xl font-semibold tracking-[-0.04em] text-[#172119];
}

.section-kicker {
  @apply text-[0.65rem] font-bold tracking-[0.22em] text-[#758079];
}

.status-badge {
  @apply rounded-full px-3 py-1 text-xs font-bold uppercase tracking-wider;
}

.status-pending {
  @apply bg-[#fff0cf] text-[#8a5a13];
}

.status-approved {
  @apply bg-[#deedda] text-[#37633f];
}

.status-rejected {
  @apply bg-[#f2d9d4] text-[#8b392f];
}

.status-mistake {
  @apply bg-[#e6def1] text-[#5e4478];
}

.content-card,
.assessment-card,
.reason-card,
.action-card,
.completed-card {
  @apply rounded-2xl border border-[#dddcd2] bg-white shadow-[0_1px_0_rgba(20,30,22,0.03)];
}

.content-card {
  @apply mt-7 p-5 md:p-7;
}

.content-text {
  @apply whitespace-pre-wrap text-lg font-medium leading-8 text-[#202c23];
}

.metadata-grid {
  @apply mt-7 grid grid-cols-1 gap-4 border-t border-[#e6e5dc] pt-5 text-sm sm:grid-cols-2;
}

.metadata-grid dt,
.decision-list dt {
  @apply text-xs text-[#7a827b];
}

.metadata-grid dd,
.decision-list dd {
  @apply mt-1 break-words font-medium text-[#303b32];
}

.mono-value {
  @apply font-mono text-xs;
}

.assessment-grid {
  @apply mt-4 grid grid-cols-1 gap-4 md:grid-cols-2;
}

.assessment-card {
  @apply p-5 md:p-6;
}

.risk-row {
  @apply mt-4 flex items-baseline gap-2;
}

.risk-row strong {
  @apply text-4xl font-semibold tracking-[-0.05em] text-[#bd6930];
}

.risk-row span {
  @apply text-sm text-[#777f78];
}

.risk-track {
  @apply mt-4 h-2 overflow-hidden rounded-full bg-[#eee9dd];
}

.risk-track span {
  @apply block h-full rounded-full bg-gradient-to-r from-[#d9a349] to-[#bd5637];
}

.label-list {
  @apply mt-5 flex flex-wrap gap-2;
}

.label-list span {
  @apply rounded-full border border-[#d7d8cd] bg-[#f7f6f0] px-2.5 py-1 text-xs font-semibold text-[#4f5d52];
}

.decision-list {
  @apply mt-4 space-y-3;
}

.decision-list div {
  @apply flex items-center justify-between gap-4 border-b border-[#eeede7] pb-3 last:border-0 last:pb-0;
}

.reason-card {
  @apply mt-4 p-5 md:p-6;
}

.reason-card > p:last-child {
  @apply mt-3 text-sm leading-6 text-[#4e5a50];
}

.reason-card h3 {
  @apply mt-2 text-sm font-semibold text-[#27352a];
}

.action-card {
  @apply mt-4 space-y-5 p-5 md:p-6;
}

.field-label {
  @apply mb-2 block text-sm font-semibold text-[#27352a];
}

.notes-input {
  @apply w-full resize-y rounded-xl border border-[#cbcdc3] bg-[#fbfaf6] px-4 py-3 text-sm leading-6 text-[#202c23] outline-none transition focus:border-[#456b4d] focus:ring-4 focus:ring-[#456b4d]/15 disabled:opacity-60;
}

.mistake-fieldset {
  @apply flex flex-wrap items-center gap-4 rounded-xl bg-[#f3f1e9] px-4 py-3 text-sm text-[#4f5b51];
}

.mistake-fieldset legend {
  @apply mb-2 w-full text-xs font-semibold text-[#687169];
}

.mistake-fieldset label {
  @apply flex cursor-pointer items-center gap-2 font-medium;
}

.mistake-fieldset input {
  @apply h-4 w-4 accent-[#456b4d];
}

.validation-error {
  @apply text-sm text-[#9b3c30];
}

.action-row {
  @apply grid grid-cols-1 gap-3 sm:grid-cols-3;
}

.action-button {
  @apply min-h-11 rounded-xl px-4 text-sm font-bold transition focus:outline-none focus:ring-4 disabled:cursor-not-allowed disabled:opacity-50;
}

.action-allow {
  @apply bg-[#365e3e] text-white hover:bg-[#294d31] focus:ring-[#365e3e]/20;
}

.action-block {
  @apply bg-[#8d3f34] text-white hover:bg-[#743229] focus:ring-[#8d3f34]/20;
}

.action-mistake {
  @apply border border-[#a994bf] bg-white text-[#624b76] hover:bg-[#f5f0f8] focus:ring-[#8c6ca8]/20;
}

.completed-card {
  @apply mt-4 border-[#bed0bb] bg-[#f0f7ee] p-5 text-sm text-[#3d5942];
}

.completed-card p {
  @apply mt-2;
}
</style>
