/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { useMemo, useState } from 'react'
import { Copy, Check, RefreshCw, Hash, Tag } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { StatusBadge, type StatusBadgeProps } from '@/components/status-badge'
import { type AntigravityUsageResponse } from '../../api'

export type AntigravityUsageDialogData = AntigravityUsageResponse

type AntigravityCredit = {
  creditType?: string
  creditAmount?: string
  minimumCreditAmountForUsage?: string
}

type AntigravityPayload = {
  cloudaicompanionProject?: string
  projectId?: string
  project?: string | { id?: string }
  currentTier?: { id?: string }
  allowedTiers?: Array<{ id?: string; isDefault?: boolean }>
  paidTier?: {
    id?: string
    name?: string
    availableCredits?: AntigravityCredit[]
  }
}

type AntigravityUsageDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  channelName?: string
  channelId?: number
  response: AntigravityUsageDialogData | null
  onRefresh?: () => void
  isRefreshing?: boolean
}

const MODEL_GROUPS: Array<{ label: string; keys: string[]; color: string }> = [
  {
    label: 'Gemini Pro',
    keys: [
      'gemini-2.5-pro', 'gemini-3-pro-low', 'gemini-3-pro-high', 'gemini-3-pro-preview',
      'gemini-3.1-pro-low', 'gemini-3.1-pro-high', 'gemini-pro-agent',
    ],
    color: 'bg-indigo-500',
  },
  {
    label: 'Gemini Flash',
    keys: [
      'gemini-2.5-flash', 'gemini-2.5-flash-lite', 'gemini-2.0-flash',
      'gemini-3-flash', 'gemini-3-flash-agent', 'gemini-3.1-flash-lite',
      'gemini-3.5-flash-low', 'gemini-3.5-flash-extra-low',
    ],
    color: 'bg-emerald-500',
  },
  {
    label: 'Gemini Image',
    keys: ['gemini-2.5-flash-image', 'gemini-3.1-flash-image', 'gemini-3-pro-image'],
    color: 'bg-purple-500',
  },
  {
    label: 'Claude',
    keys: [
      'claude-sonnet-4-5', 'claude-sonnet-4-5-thinking', 'claude-sonnet-4-6',
      'claude-opus-4-5-thinking', 'claude-opus-4-6', 'claude-opus-4-6-thinking', 'claude-opus-4-7',
      'claude-sonnet-4', 'claude-sonnet-4-20250514',
    ],
    color: 'bg-amber-500',
  },
]

type ModelQuotaBar = {
  label: string
  utilization: number | null
  resetTime: string | null
  color: string
}

function extractModelQuotaBars(
  modelsQuota: AntigravityUsageResponse['models_quota']
): ModelQuotaBar[] {
  const models = modelsQuota?.models
  if (!models) return []
  const bars: ModelQuotaBar[] = []
  for (const group of MODEL_GROUPS) {
    let maxUtil = -1
    let earliestReset: string | null = null
    let hasModel = false
    for (const key of group.keys) {
      const m = models[key]
      if (!m) continue
      hasModel = true
      if (!m.quotaInfo) continue
      const util = Math.round((1 - (m.quotaInfo.remainingFraction ?? 1)) * 100)
      if (util > maxUtil) maxUtil = util
      const rt = m.quotaInfo.resetTime ?? null
      if (rt && (!earliestReset || rt < earliestReset)) earliestReset = rt
    }
    if (!hasModel) continue
    bars.push({ label: group.label, utilization: maxUtil < 0 ? null : maxUtil, resetTime: earliestReset, color: group.color })
  }
  return bars
}

function formatResetTime(resetTime: string | null): string {
  if (!resetTime) return ''
  const d = new Date(resetTime)
  if (isNaN(d.getTime())) return ''
  const diffMs = d.getTime() - Date.now()
  if (diffMs <= 0) return 'now'
  const diffMin = Math.floor(diffMs / 60000)
  const diffH = Math.floor(diffMin / 60)
  if (diffH >= 1) return `${diffH}h${diffMin % 60}m`
  return `${diffMin}m`
}

function extractProjectId(
  payload: AntigravityPayload | null,
  keyProjectId?: string
): string {
  if (keyProjectId?.trim()) return keyProjectId.trim()
  if (!payload) return ''
  if (typeof payload.cloudaicompanionProject === 'string')
    return payload.cloudaicompanionProject.trim()
  if (typeof payload.projectId === 'string') return payload.projectId.trim()
  if (typeof payload.project === 'string') return payload.project.trim()
  if (typeof payload.project === 'object' && payload.project?.id)
    return String(payload.project.id).trim()
  return ''
}

function extractTierId(payload: AntigravityPayload | null): string {
  if (!payload) return ''
  // paidTier.id reflects actual membership (e.g. g1-pro-tier), prefer over free-tier default
  if (payload.paidTier?.id?.trim()) return payload.paidTier.id.trim()
  if (payload.currentTier?.id?.trim()) return payload.currentTier.id.trim()
  if (payload.allowedTiers) {
    const def = payload.allowedTiers.find((t) => t.isDefault)
    if (def?.id) return def.id.trim()
  }
  return ''
}

function parseCreditsFloat(value: string | undefined): number {
  if (!value) return 0
  const n = parseFloat(value.trim())
  return Number.isFinite(n) ? n : 0
}

function CopyableField(props: {
  icon: React.ReactNode
  label: string
  value?: string | null
  mono?: boolean
}) {
  const { copyToClipboard, copiedText } = useCopyToClipboard({ notify: false })
  const text = props.value?.trim() || ''
  const hasCopied = copiedText === text

  return (
    <div className='flex items-center justify-between gap-2 py-1'>
      <div className='flex min-w-0 items-center gap-2'>
        <span className='text-muted-foreground flex-shrink-0'>{props.icon}</span>
        <span className='text-muted-foreground flex-shrink-0 text-xs'>
          {props.label}
        </span>
        <span
          className={`min-w-0 truncate text-xs ${props.mono ? 'font-mono' : ''}`}
        >
          {text || '-'}
        </span>
      </div>
      {text && (
        <Button
          type='button'
          variant='ghost'
          size='sm'
          className='h-6 w-6 flex-shrink-0 p-0'
          onClick={() => copyToClipboard(text)}
        >
          {hasCopied ? (
            <Check className='h-3 w-3 text-green-600' />
          ) : (
            <Copy className='h-3 w-3' />
          )}
        </Button>
      )}
    </div>
  )
}

function getTierBadge(tierId: string): {
  label: string
  variant: StatusBadgeProps['variant']
} {
  if (!tierId) return { label: '-', variant: 'neutral' }
  const lower = tierId.toLowerCase()
  if (lower.includes('paid') || lower.includes('pro') || lower.includes('plus') || lower.includes('g1'))
    return { label: tierId, variant: 'success' }
  if (lower.includes('free')) return { label: tierId, variant: 'warning' }
  return { label: tierId, variant: 'info' }
}

export function AntigravityUsageDialog({
  open,
  onOpenChange,
  channelName,
  channelId,
  response,
  onRefresh,
  isRefreshing,
}: AntigravityUsageDialogProps) {
  const { t } = useTranslation()
  const [showRawJson, setShowRawJson] = useState(false)
  const { copyToClipboard, copiedText } = useCopyToClipboard({ notify: false })

  const payload: AntigravityPayload | null = useMemo(() => {
    const raw = response?.data
    if (!raw || typeof raw !== 'object') return null
    return raw as AntigravityPayload
  }, [response?.data])

  const projectId = extractProjectId(payload, response?.key_project_id)
  const tierId = extractTierId(payload)
  const tierBadge = getTierBadge(tierId)

  const paidTier = payload?.paidTier
  const credits = paidTier?.availableCredits ?? []
  const g1Credit = credits.find(
    (c) => c.creditType?.toUpperCase() === 'GOOGLE_ONE_AI'
  )

  const creditAmount = parseCreditsFloat(g1Credit?.creditAmount)
  const minAmount = parseCreditsFloat(g1Credit?.minimumCreditAmountForUsage)
  const creditsAvailable = g1Credit != null && creditAmount >= minAmount

  const modelQuotaBars = useMemo(
    () => extractModelQuotaBars(response?.models_quota),
    [response?.models_quota]
  )

  const errorMessage =
    response?.success === false
      ? response?.message?.trim() || t('Failed to fetch account info')
      : ''

  const rawJsonText = useMemo(() => {
    if (!response) return ''
    try {
      return JSON.stringify(
        {
          success: response.success,
          message: response.message,
          upstream_status: response.upstream_status,
          data: response.data,
        },
        null,
        2
      )
    } catch {
      return String(response?.data ?? '')
    }
  }, [response])

  const rawCopied = copiedText === rawJsonText

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='sm:max-w-2xl'>
        <DialogHeader>
          <DialogTitle>{t('Antigravity Account Info')}</DialogTitle>
          <DialogDescription>
            {t('Channel:')} <strong>{channelName || '-'}</strong>{' '}
            {channelId ? `(#${channelId})` : ''}
          </DialogDescription>
        </DialogHeader>

        <div className='space-y-4'>
          {errorMessage && (
            <div className='rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-800 dark:bg-red-950/30 dark:text-red-400'>
              {errorMessage}
            </div>
          )}

          {/* Summary bar */}
          <div className='rounded-lg border p-4'>
            <div className='flex flex-wrap items-center justify-between gap-2'>
              <div className='flex flex-wrap items-center gap-2'>
                <StatusBadge
                  label='Antigravity'
                  variant='blue'
                  copyable={false}
                />
                {g1Credit != null && (
                  <StatusBadge
                    label={creditsAvailable ? t('Credits Available') : t('Credits Exhausted')}
                    variant={creditsAvailable ? 'success' : 'danger'}
                    copyable={false}
                  />
                )}
                {tierId && (
                  <StatusBadge
                    label={tierBadge.label}
                    variant={tierBadge.variant}
                    copyable={false}
                  />
                )}
                {typeof response?.upstream_status === 'number' && (
                  <StatusBadge
                    label={`${t('Status:')} ${response.upstream_status}`}
                    variant='neutral'
                    copyable={false}
                  />
                )}
              </div>
              {onRefresh && (
                <Button
                  type='button'
                  variant='outline'
                  size='sm'
                  onClick={onRefresh}
                  disabled={Boolean(isRefreshing)}
                >
                  <RefreshCw className='mr-1.5 h-3.5 w-3.5' />
                  {t('Refresh')}
                </Button>
              )}
            </div>

            {/* Identity fields */}
            <div className='bg-muted/30 mt-3 rounded-md px-3 py-2'>
              <CopyableField
                icon={<Hash className='h-3.5 w-3.5' />}
                label='Project ID'
                value={projectId}
                mono
              />
              {paidTier?.name && (
                <CopyableField
                  icon={<Tag className='h-3.5 w-3.5' />}
                  label={t('Membership')}
                  value={paidTier.name}
                />
              )}
              {paidTier?.id && (
                <CopyableField
                  icon={<Tag className='h-3.5 w-3.5' />}
                  label={t('Paid Tier')}
                  value={paidTier.id}
                  mono
                />
              )}
              {payload?.currentTier?.id && (
                <CopyableField
                  icon={<Tag className='h-3.5 w-3.5' />}
                  label={t('Code Assist Tier')}
                  value={payload.currentTier.id}
                  mono
                />
              )}
            </div>
          </div>

          {/* Google One AI Credits */}
          {g1Credit && (
            <div className='rounded-lg border p-4'>
              <div className='flex items-center justify-between gap-2'>
                <div className='text-sm font-medium'>
                  {t('Google One AI Credits')}
                </div>
                <StatusBadge
                  label={creditsAvailable ? t('Available') : t('Exhausted')}
                  variant={creditsAvailable ? 'success' : 'danger'}
                  copyable={false}
                />
              </div>
              <div className='text-muted-foreground mt-2 flex flex-wrap gap-4 text-xs'>
                <div>
                  {t('Balance:')}
                  <span className='ml-1 font-medium text-foreground'>
                    {g1Credit.creditAmount ?? '-'}
                  </span>
                </div>
                <div>
                  {t('Minimum for usage:')}
                  <span className='ml-1 font-medium text-foreground'>
                    {g1Credit.minimumCreditAmountForUsage ?? '-'}
                  </span>
                </div>
              </div>
            </div>
          )}

          {/* Model Quota Windows */}
          {modelQuotaBars.length > 0 && (
            <div className='rounded-lg border p-4'>
              <div className='mb-3 text-sm font-medium'>{t('Model Quota')}</div>
              <div className='space-y-2'>
                {modelQuotaBars.map((bar) => {
                  const pct = bar.utilization != null ? Math.min(100, Math.max(0, bar.utilization)) : null
                  const reset = formatResetTime(bar.resetTime)
                  return (
                    <div key={bar.label} className='flex items-center gap-2'>
                      <span className='w-24 flex-shrink-0 text-xs text-muted-foreground'>{bar.label}</span>
                      <div className='h-1.5 flex-1 overflow-hidden rounded-full bg-gray-200 dark:bg-gray-700'>
                        {pct != null && (
                          <div
                            className={`h-full transition-all duration-300 ${bar.color}`}
                            style={{ width: `${pct}%` }}
                          />
                        )}
                      </div>
                      <span className='w-9 flex-shrink-0 text-right text-xs font-medium'>
                        {pct != null ? `${pct}%` : 'N/A'}
                      </span>
                      {reset && (
                        <span className='flex-shrink-0 text-xs text-muted-foreground'>{reset}</span>
                      )}
                    </div>
                  )
                })}
              </div>
            </div>
          )}

          {/* Raw JSON toggle */}
          <div>
            <Button
              type='button'
              variant='ghost'
              size='sm'
              className='text-muted-foreground h-7 px-2 text-xs'
              onClick={() => setShowRawJson((v) => !v)}
            >
              {showRawJson ? t('Hide raw JSON') : t('Show raw JSON')}
            </Button>
            {showRawJson && rawJsonText && (
              <div className='relative mt-2'>
                <Button
                  type='button'
                  variant='ghost'
                  size='sm'
                  className='absolute top-2 right-2 h-6 w-6 p-0'
                  onClick={() => copyToClipboard(rawJsonText)}
                >
                  {rawCopied ? (
                    <Check className='h-3 w-3 text-green-600' />
                  ) : (
                    <Copy className='h-3 w-3' />
                  )}
                </Button>
                <pre className='bg-muted/40 max-h-64 overflow-auto rounded-md p-3 font-mono text-xs whitespace-pre-wrap break-all'>
                  {rawJsonText}
                </pre>
              </div>
            )}
          </div>
        </div>

        <DialogFooter>
          <Button variant='outline' onClick={() => onOpenChange(false)}>
            {t('Close')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
