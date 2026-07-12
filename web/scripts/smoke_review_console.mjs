#!/usr/bin/env node

import { chromium } from 'playwright'

const baseURL = requiredEnv('HATESENTRY_BASE_URL').replace(/\/$/, '')
const email = requiredEnv('HATESENTRY_ADMIN_EMAIL')
const password = requiredEnv('HATESENTRY_ADMIN_PASSWORD')
const content = process.env.HATESENTRY_CONSOLE_SMOKE_CONTENT
  ?? `Browser console smoke review ${Date.now()}`
const screenshotPath = process.env.HATESENTRY_CONSOLE_SCREENSHOT
  ?? '/tmp/hatesentry-console-smoke-failure.png'

const browser = await chromium.launch({ headless: true })
const page = await browser.newPage()

try {
  await page.goto(`${baseURL}/console/`, { waitUntil: 'networkidle' })
  await page.getByLabel('管理员邮箱').fill(email)
  await page.getByLabel('密码').fill(password)
  await page.getByRole('button', { name: '进入复核队列' }).click()
  await page.getByRole('button', { name: '退出' }).waitFor()

  const token = await page.evaluate(() => {
    const raw = sessionStorage.getItem('hatesentry-operator-session')
    if (!raw) return ''
    return JSON.parse(raw).token ?? ''
  })
  if (!token) throw new Error('review console did not persist the administrator JWT')

  const moderationResponse = await page.request.post(
    `${baseURL}/api/v1/moderation/check`,
    {
      headers: { Authorization: `Bearer ${token}` },
      data: {
        content,
        source: 'console-smoke',
        external_id: `console-smoke-${Date.now()}`,
        actor_id: 'browser-smoke',
      },
    },
  )
  if (!moderationResponse.ok()) {
    throw new Error(
      `seed moderation request failed with ${moderationResponse.status()}: ${await moderationResponse.text()}`,
    )
  }
  const moderation = await moderationResponse.json()
  if (moderation.decision !== 'review') {
    throw new Error(`seed moderation decision = ${moderation.decision}, want review`)
  }

  await page.getByRole('button', { name: '刷新待复核队列' }).click()
  const queueContent = page.getByText(content, { exact: true })
  await queueContent.waitFor()
  await queueContent.click()
  await page.getByRole('heading', { name: '复核内容' }).waitFor()
  await page.getByLabel('复核备注').fill('Chromium smoke approved this review case.')
  await page.getByRole('button', { name: '通过并允许' }).click()
  await page.getByText('复核结果已保存，待处理队列已更新。').waitFor()
  await page.getByText('人工最终决定：allow').waitFor()

  const completedHistoryResponse = page.waitForResponse((response) => {
    const url = new URL(response.url())
    return url.pathname === '/api/v1/reviews'
      && url.searchParams.get('status') === 'completed'
  })
  await page.getByRole('button', { name: '审核历史' }).click()
  const completedResponse = await completedHistoryResponse
  if (!completedResponse.ok()) {
    throw new Error(`completed history request failed with ${completedResponse.status()}`)
  }
  const completedURL = new URL(completedResponse.url())
  if (completedURL.searchParams.get('limit') !== '50') {
    throw new Error(`completed history limit = ${completedURL.searchParams.get('limit')}, want 50`)
  }
  await page.getByRole('heading', { name: '审核历史', exact: true }).waitFor()
  await page.getByLabel('人工状态').selectOption('approved')
  const historyContent = page.getByText(content, { exact: true })
  await historyContent.waitFor()
  await historyContent.click()
  await page.getByText('复核人：操作员 #1').waitFor()
  await page.getByText('人工最终决定：allow').waitFor()

  const resultResponse = await page.request.get(
    `${baseURL}/api/v1/moderation/results/${moderation.request_id}`,
    { headers: { Authorization: `Bearer ${token}` } },
  )
  if (!resultResponse.ok()) {
    throw new Error(
      `final result request failed with ${resultResponse.status()}: ${await resultResponse.text()}`,
    )
  }
  const result = await resultResponse.json()
  if (result.review_status !== 'approved' || result.final_decision !== 'allow') {
    throw new Error(
      `final review state = ${result.review_status}/${result.final_decision}, want approved/allow`,
    )
  }

  process.stdout.write(`${JSON.stringify({
    console_url: `${baseURL}/console/`,
    decision: moderation.decision,
    final_decision: result.final_decision,
    history_filter: 'approved',
    history_query: 'completed',
    request_id: moderation.request_id,
    review_status: result.review_status,
  }, null, 2)}\n`)
} catch (error) {
  await page.screenshot({ path: screenshotPath, fullPage: true }).catch(() => undefined)
  process.stderr.write(`review console smoke failed; screenshot: ${screenshotPath}\n`)
  throw error
} finally {
  await browser.close()
}

function requiredEnv(name) {
  const value = process.env[name]?.trim()
  if (!value) throw new Error(`${name} is required`)
  return value
}
