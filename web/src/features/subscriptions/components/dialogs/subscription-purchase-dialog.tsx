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

For commercial licensing, please contact support@quantumnous.com.
*/
import { Crown, Wallet } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Dialog } from '@/components/dialog'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { useSystemConfig } from '@/hooks/use-system-config'
import { formatQuota } from '@/lib/format'
import { DEFAULT_CURRENCY_CONFIG } from '@/stores/system-config-store'

import { paySubscriptionBalance } from '../../api'
import type { PlanRecord } from '../../types'

interface Props {
  open: boolean
  onOpenChange: (open: boolean) => void
  plan: PlanRecord | null
  purchaseLimit?: number
  purchaseCount?: number
  userQuota?: number
  onPurchaseSuccess?: () => void | Promise<void>
}

export function SubscriptionPurchaseDialog(props: Props) {
  const { t } = useTranslation()
  const { currency } = useSystemConfig()
  const [paying, setPaying] = useState(false)

  const plan = props.plan?.plan
  if (!plan) return null

  const price = Number(plan.price_amount || 0).toFixed(2)
  const totalAmount = Number(plan.total_amount || 0)
  const quotaPerUnit =
    currency?.quotaPerUnit && currency.quotaPerUnit > 0
      ? currency.quotaPerUnit
      : DEFAULT_CURRENCY_CONFIG.quotaPerUnit
  const balanceCost = Math.max(
    0,
    Math.ceil(Number(plan.price_amount || 0) * quotaPerUnit),
  )
  const userQuota = Math.max(0, Number(props.userQuota || 0))
  const allowBalancePay = plan.allow_balance_pay !== false
  const insufficientBalance = userQuota < balanceCost
  const limitReached =
    (props.purchaseLimit || 0) > 0 &&
    (props.purchaseCount || 0) >= (props.purchaseLimit || 0)

  const handlePayBalance = async () => {
    setPaying(true)
    try {
      const res = await paySubscriptionBalance({ plan_id: plan.id })
      if (res.message === 'success') {
        toast.success(t('Subscription activated'))
        props.onOpenChange(false)
        if (props.onPurchaseSuccess) {
          await props.onPurchaseSuccess()
        }
      } else {
        toast.error(
          res.message && res.message !== 'success'
            ? res.message
            : t('Payment request failed'),
        )
      }
    } catch {
      toast.error(t('Payment request failed'))
    } finally {
      setPaying(false)
    }
  }

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={t('Subscribe to {{title}}', { title: plan.title })}
      contentClassName='flex max-h-[calc(100dvh-2rem)] flex-col max-sm:w-screen max-sm:max-w-none max-sm:rounded-none max-sm:p-4 sm:max-w-md'
      contentHeight='auto'
      bodyClassName='space-y-3'
    >
      <div className='space-y-4'>
        <div className='flex items-center gap-3'>
          <Crown className='h-5 w-5 text-amber-500' />
          <div>
            <div className='font-semibold'>{plan.title}</div>
            {plan.subtitle ? (
              <div className='text-muted-foreground text-sm'>
                {plan.subtitle}
              </div>
            ) : null}
          </div>
        </div>

        <div className='grid grid-cols-2 gap-3 rounded-lg border p-3'>
          <div>
            <div className='text-muted-foreground text-xs'>
              {t('Price')}
            </div>
            <div className='text-lg font-semibold'>${price}</div>
          </div>
          <div>
            <div className='text-muted-foreground text-xs'>
              {t('Quota per cycle')}
            </div>
            <div className='text-lg font-semibold'>
              {totalAmount > 0 ? formatQuota(totalAmount) : '∞'}
            </div>
          </div>
        </div>

        {!allowBalancePay ? (
          <Alert variant='destructive'>
            <AlertDescription>
              {t('This plan does not allow balance payment.')}
            </AlertDescription>
          </Alert>
        ) : insufficientBalance ? (
          <Alert variant='destructive'>
            <AlertDescription>
              {t(
                'Insufficient wallet balance. Please redeem a code or contact an administrator.',
              )}
            </AlertDescription>
          </Alert>
        ) : null}

        {limitReached ? (
          <Alert variant='destructive'>
            <AlertDescription>
              {t('You have reached the purchase limit for this plan.')}
            </AlertDescription>
          </Alert>
        ) : null}

        <Button
          className='w-full'
          disabled={
            paying || !allowBalancePay || insufficientBalance || limitReached
          }
          onClick={handlePayBalance}
        >
          <Wallet className='mr-2 h-4 w-4' />
          {paying ? t('Processing...') : t('Pay with wallet ({{quota}})', {
            quota: formatQuota(balanceCost),
          })}
        </Button>
      </div>
    </Dialog>
  )
}
