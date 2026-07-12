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

  await page.getByRole('button', { name: '客户端管理' }).click()
  await page.getByRole('heading', { name: '客户端管理', exact: true }).waitFor()
  const clientName = `browser-client-${Date.now()}`
  await page.getByLabel('客户端名称').fill(clientName)
  await page.getByRole('button', { name: '创建客户端' }).click()
  const oneTimeKey = page.getByTestId('one-time-api-key')
  await oneTimeKey.waitFor()
  const createdKey = (await oneTimeKey.textContent())?.trim() ?? ''
  if (!createdKey) throw new Error('created client did not show its one-time API key')
  await page.getByRole('button', { name: '关闭一次性 API Key' }).click()
  await oneTimeKey.waitFor({ state: 'detached' })

  const clientRow = page.locator('tr').filter({ hasText: clientName })
  const policySelect = clientRow.getByLabel(`${clientName} 的审核策略`)
  await policySelect.selectOption('strict-v1')
  await clientRow.getByRole('button', { name: '应用策略' }).click()
  await page.getByText('客户端策略已更新为 strict-v1，将用于后续审核请求。').waitFor()
  if (await policySelect.inputValue() !== 'strict-v1') {
    throw new Error('client policy selection did not persist strict-v1')
  }

  await clientRow.getByRole('button', { name: '停用' }).click()
  await clientRow.getByRole('button', { name: '启用' }).waitFor()
  const inactiveResponse = await submitWithAPIKey(page, baseURL, createdKey, 'inactive')
  if (inactiveResponse.status() !== 401) {
    throw new Error(`inactive client key status = ${inactiveResponse.status()}, want 401`)
  }

  await clientRow.getByRole('button', { name: '启用' }).click()
  await clientRow.getByRole('button', { name: '停用' }).waitFor()
  await clientRow.getByRole('button', { name: '轮换 API Key' }).click()
  await clientRow.getByRole('button', { name: '确认轮换并使旧 Key 失效' }).click()
  await oneTimeKey.waitFor()
  const rotatedKey = (await oneTimeKey.textContent())?.trim() ?? ''
  if (!rotatedKey || rotatedKey === createdKey) {
    throw new Error('rotated client key was missing or unchanged')
  }

  const oldKeyResponse = await submitWithAPIKey(page, baseURL, createdKey, 'rotated-old')
  if (oldKeyResponse.status() !== 401) {
    throw new Error(`rotated old key status = ${oldKeyResponse.status()}, want 401`)
  }
  const newKeyResponse = await submitWithAPIKey(page, baseURL, rotatedKey, 'rotated-new')
  if (!newKeyResponse.ok()) {
    throw new Error(`rotated new key failed with ${newKeyResponse.status()}`)
  }
  const newKeyModeration = await newKeyResponse.json()
  if (newKeyModeration.policy_version !== 'strict-v1') {
    throw new Error(`new key policy = ${newKeyModeration.policy_version}, want strict-v1`)
  }

  await page.getByRole('button', { name: '关闭一次性 API Key' }).click()
  await policySelect.selectOption('')
  await clientRow.getByRole('button', { name: `将 ${clientName} 恢复为跟随默认策略` }).click()
  await page.getByText('客户端已恢复为跟随系统默认策略。').waitFor()
  if (await policySelect.inputValue() !== '') {
    throw new Error('client policy selection did not reset to following the default')
  }
  const clientID = (await clientRow.locator('small').first().textContent())?.replace('#', '').trim()
  if (!clientID) throw new Error('client row did not expose its identifier')
  const resetClientResponse = await page.request.get(
    `${baseURL}/api/v1/admin/clients/${clientID}`,
    { headers: { Authorization: `Bearer ${token}` } },
  )
  if (!resetClientResponse.ok()) {
    throw new Error(`reset client lookup failed with ${resetClientResponse.status()}`)
  }
  const resetClient = await resetClientResponse.json()
  if ('policy_version' in resetClient) {
    throw new Error(`reset client still has explicit policy ${resetClient.policy_version}`)
  }
  const defaultPolicyResponse = await submitWithAPIKey(page, baseURL, rotatedKey, 'reset-default')
  if (!defaultPolicyResponse.ok()) {
    throw new Error(`default policy moderation failed with ${defaultPolicyResponse.status()}`)
  }
  const defaultPolicyModeration = await defaultPolicyResponse.json()
  if (defaultPolicyModeration.policy_version !== 'default-v1') {
    throw new Error(`reset policy = ${defaultPolicyModeration.policy_version}, want default-v1`)
  }
  await clientRow.getByRole('button', { name: '停用' }).click()
  await clientRow.getByRole('button', { name: '启用' }).waitFor()

  process.stdout.write(`${JSON.stringify({
    client_created: true,
    client_deactivated: true,
    client_reactivated: true,
    api_key_rotated: true,
    policy_assigned: true,
    policy_applied_to_moderation: true,
    policy_reset_to_default: true,
    old_key_rejected: true,
    new_key_accepted: true,
  }, null, 2)}\n`)
} catch (error) {
  await page.getByTestId('one-time-api-key').evaluateAll((nodes) => {
    for (const node of nodes) node.textContent = '[REDACTED]'
  }).catch(() => undefined)
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

function submitWithAPIKey(page, baseURL, apiKey, suffix) {
  return page.request.post(`${baseURL}/api/v1/moderation/check`, {
    headers: { 'X-API-Key': apiKey },
    data: {
      content: `Browser client key smoke ${suffix}`,
      source: 'console-client-smoke',
      external_id: `console-client-${suffix}-${Date.now()}`,
      actor_id: 'browser-smoke',
    },
  })
}
