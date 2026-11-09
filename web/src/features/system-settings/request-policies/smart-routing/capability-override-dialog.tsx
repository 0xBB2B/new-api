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
import { useId, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Separator } from '@/components/ui/separator'
import { Switch } from '@/components/ui/switch'

import { SMART_EFFORTS, type EffectiveCapability } from './capability'
import type { SmartEffort, ThinkingCapabilityOverride } from './types'

type CapabilityOverrideDialogProps = {
  model: string
  current: EffectiveCapability
  onOpenChange: (open: boolean) => void
  onSave: (override: ThinkingCapabilityOverride) => Promise<void>
}

export function CapabilityOverrideDialog(props: CapabilityOverrideDialogProps) {
  const { t } = useTranslation()
  const [noThinking, setNoThinking] = useState(props.current.noThinking)
  const [efforts, setEfforts] = useState<SmartEffort[]>(props.current.efforts)
  const [showError, setShowError] = useState(false)
  const [saving, setSaving] = useState(false)
  const headingId = useId()

  const toggle = (effort: SmartEffort, checked: boolean) => {
    setShowError(false)
    setEfforts(
      checked ? [...efforts, effort] : efforts.filter((e) => e !== effort)
    )
  }

  const save = async () => {
    if (!noThinking && efforts.length === 0) {
      setShowError(true)
      return
    }
    setSaving(true)
    try {
      await props.onSave({
        no_thinking: noThinking,
        efforts: noThinking
          ? []
          : SMART_EFFORTS.filter((effort) => efforts.includes(effort)),
      })
    } catch {
      // 错误提示已由保存请求统一弹出
    } finally {
      setSaving(false)
    }
  }

  return (
    <Dialog
      open
      contentClassName='sm:max-w-md'
      onOpenChange={(open) => {
        if (!open && saving) return
        props.onOpenChange(open)
      }}
      title={t('Override thinking capability of {{model}}', {
        model: props.model,
      })}
      description={t(
        'Tick the thinking efforts this model supports. Takes effect immediately after you save.'
      )}
      footer={
        <>
          <Button
            type='button'
            variant='outline'
            disabled={saving}
            onClick={() => props.onOpenChange(false)}
          >
            {t('Cancel')}
          </Button>
          <Button type='button' disabled={saving} onClick={() => void save()}>
            {saving ? t('Saving...') : t('Save')}
          </Button>
        </>
      }
    >
      <div className='flex flex-col gap-4'>
        <label className='flex items-center justify-between gap-2 text-sm font-medium'>
          {t('No thinking')}
          <Switch
            checked={noThinking}
            onCheckedChange={(checked) => {
              setShowError(false)
              setNoThinking(checked)
            }}
          />
        </label>
        <Separator />
        <h4 id={headingId} className='text-sm font-medium'>
          {t('Supported efforts')}
        </h4>
        <div
          role='group'
          aria-labelledby={headingId}
          className='grid grid-cols-4 gap-3'
        >
          {SMART_EFFORTS.map((effort) => (
            <label key={effort} className='flex items-center gap-2 text-sm'>
              <Checkbox
                checked={efforts.includes(effort)}
                disabled={noThinking}
                onCheckedChange={(checked) => toggle(effort, checked)}
              />
              {effort}
            </label>
          ))}
        </div>
        {showError ? (
          <p className='text-destructive text-sm'>
            {t('Select at least one level')}
          </p>
        ) : null}
      </div>
    </Dialog>
  )
}
