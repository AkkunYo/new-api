/*
Copyright (C) 2025 QuantumNous

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
import { useEffect, useMemo, useState } from 'react'
import { Check, Copy, ExternalLink, Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { tryPrettyJson } from '@/lib/utils'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { completeAntigravityOAuth, startAntigravityOAuth } from '../../api'

type AntigravityOAuthDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  /** Called with the generated JSON key (new channel flow). */
  onKeyGenerated?: (key: string) => void
  /** Present when editing an existing channel — key is written server-side. */
  channelId?: number
}

export function AntigravityOAuthDialog({
  open,
  onOpenChange,
  onKeyGenerated,
  channelId,
}: AntigravityOAuthDialogProps) {
  const { t } = useTranslation()
  const { copiedText, copyToClipboard } = useCopyToClipboard({ notify: false })

  const [state, setState] = useState({
    authorizeUrl: '',
    callbackUrl: '',
    isStarting: false,
    isCompleting: false,
  })

  useEffect(() => {
    if (!open) {
      setState({
        authorizeUrl: '',
        callbackUrl: '',
        isStarting: false,
        isCompleting: false,
      })
    }
  }, [open])

  const canCopyAuthorizeUrl = Boolean(state.authorizeUrl && !state.isStarting)
  const canComplete = useMemo(
    () =>
      Boolean(state.authorizeUrl) &&
      Boolean(state.callbackUrl.trim()) &&
      !state.isCompleting,
    [state.authorizeUrl, state.callbackUrl, state.isCompleting]
  )

  const handleStart = async () => {
    setState((prev) => ({ ...prev, isStarting: true }))
    try {
      const res = await startAntigravityOAuth(channelId)
      if (!res.success) {
        throw new Error(res.message || 'Failed to start OAuth')
      }

      const url = res.data?.authorize_url || ''
      if (!url) {
        throw new Error('Missing authorize_url in response')
      }

      setState((prev) => ({ ...prev, authorizeUrl: url }))
      try {
        window.open(url, '_blank', 'noopener,noreferrer')
        toast.success(t('Opened authorization page'))
      } catch {
        toast.warning(t('Please manually copy and open the authorization link'))
      }
    } catch (error) {
      toast.error(
        error instanceof Error ? error.message : t('OAuth start failed')
      )
    } finally {
      setState((prev) => ({ ...prev, isStarting: false }))
    }
  }

  const handleComplete = async () => {
    if (!state.callbackUrl.trim()) return
    setState((prev) => ({ ...prev, isCompleting: true }))
    try {
      const res = await completeAntigravityOAuth(
        state.callbackUrl.trim(),
        channelId
      )
      if (!res.success) {
        throw new Error(res.message || 'OAuth failed')
      }

      if (channelId) {
        // Existing channel: key written server-side
        const projectId = res.data?.project_id || ''
        toast.success(
          projectId
            ? t('Authorization successful, project: {{id}}', { id: projectId })
            : t('Authorization successful')
        )
        onOpenChange(false)
        return
      }

      // New channel: return key to caller
      const rawKey = res.data?.key || ''
      if (!rawKey) {
        throw new Error('Missing key in response')
      }
      onKeyGenerated?.(tryPrettyJson(rawKey))
      toast.success(t('Credential generated'))
      onOpenChange(false)
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('OAuth failed'))
    } finally {
      setState((prev) => ({ ...prev, isCompleting: false }))
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='sm:max-w-2xl'>
        <DialogHeader>
          <DialogTitle>{t('Antigravity Google Authorization')}</DialogTitle>
          <DialogDescription>
            {t(
              'Authorize via Google OAuth to obtain a credential for the Antigravity channel.'
            )}
          </DialogDescription>
        </DialogHeader>

        <div className='space-y-4'>
          <Alert>
            <AlertDescription>
              {t(
                '1) Click "Open authorization page" and complete Google login. 2) Your browser may redirect to localhost (it is OK if the page does not load). 3) Copy the full URL from the address bar and paste it below. 4) Click "Complete authorization".'
              )}
            </AlertDescription>
          </Alert>

          <div className='flex flex-wrap gap-2'>
            <Button onClick={handleStart} disabled={state.isStarting}>
              {state.isStarting ? (
                <Loader2 className='mr-2 h-4 w-4 animate-spin' />
              ) : (
                <ExternalLink className='mr-2 h-4 w-4' />
              )}
              {t('Open authorization page')}
            </Button>

            <Button
              type='button'
              variant='outline'
              disabled={!canCopyAuthorizeUrl}
              onClick={async () => {
                if (!state.authorizeUrl) return
                await copyToClipboard(state.authorizeUrl)
              }}
              aria-label={t('Copy authorization link')}
              title={t('Copy authorization link')}
            >
              {copiedText === state.authorizeUrl ? (
                <Check className='mr-2 h-4 w-4 text-green-600' />
              ) : (
                <Copy className='mr-2 h-4 w-4' />
              )}
              {t('Copy authorization link')}
            </Button>
          </div>

          <div className='space-y-2'>
            <div className='text-sm font-medium'>{t('Callback URL')}</div>
            <Input
              value={state.callbackUrl}
              onChange={(e) =>
                setState((prev) => ({ ...prev, callbackUrl: e.target.value }))
              }
              placeholder={t(
                'Paste the full callback URL (includes code & state)'
              )}
              autoComplete='off'
              spellCheck={false}
              disabled={!state.authorizeUrl}
            />
            <div className='text-muted-foreground text-xs'>
              {t(
                'After authorization succeeds, the system automatically discovers and saves the Google Cloud project_id. Access tokens refresh automatically 50 minutes before expiry.'
              )}
            </div>
          </div>
        </div>

        <DialogFooter>
          <Button
            type='button'
            variant='outline'
            onClick={() => onOpenChange(false)}
            disabled={state.isStarting || state.isCompleting}
          >
            {t('Cancel')}
          </Button>
          <Button onClick={handleComplete} disabled={!canComplete}>
            {state.isCompleting && (
              <Loader2 className='mr-2 h-4 w-4 animate-spin' />
            )}
            {state.isCompleting
              ? t('Authorizing...')
              : t('Complete authorization')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
