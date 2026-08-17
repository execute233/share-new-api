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
import { useState, type ChangeEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Loader2 } from 'lucide-react'

interface RedemptionCardProps {
  redeeming: boolean
  onRedeem: (code: string) => Promise<boolean | void>
}

export function RedemptionCard(props: RedemptionCardProps) {
  const { t } = useTranslation()
  const [code, setCode] = useState('')

  return (
    <Card>
      <div className='flex flex-col gap-4 p-4'>
        <div>
          <h3 className='text-lg font-semibold'>{t('Redeem Code')}</h3>
          <p className='text-sm text-muted-foreground'>
            {t('Enter a redemption code to add quota to your account.')}
          </p>
        </div>
        <div className='flex flex-col sm:flex-row gap-2'>
          <Input
            value={code}
            onChange={(e: ChangeEvent<HTMLInputElement>) => setCode(e.target.value)}
            placeholder={t('Redemption code')}
            disabled={props.redeeming}
          />
          <Button
            disabled={props.redeeming || code.trim() === ''}
            onClick={async () => {
              await props.onRedeem(code)
              setCode('')
            }}
          >
            {props.redeeming && <Loader2 className='mr-2 h-4 w-4 animate-spin' />}
            {t('Redeem')}
          </Button>
        </div>
      </div>
    </Card>
  )
}
