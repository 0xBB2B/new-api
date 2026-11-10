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
import type { ReactNode } from 'react'
import { useFormContext } from 'react-hook-form'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import {
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'

import type { EffectiveCapability } from './capability'
import { FieldInfo } from './field-info'
import type { RouteFormValues } from './route-form'
import type { SmartEffortMode, SmartTierId } from './types'

export const FIELD_ROW_CLASS =
  'grid min-w-0 grid-cols-[5.25rem_minmax(0,1fr)] items-start gap-2 sm:grid-cols-[7rem_minmax(0,1fr)] sm:gap-3'

const INHERIT = 'inherit'
const EFFORT_FIELDS = ['effort', 'effort_min', 'effort_max'] as const

export function NoThinkingEffortRow(props: { help: ReactNode }) {
  const { t } = useTranslation()
  return (
    <div className={`${FIELD_ROW_CLASS} text-muted-foreground`}>
      <div className='flex min-h-8 items-center gap-1'>
        <Label className='text-muted-foreground'>{t('Reasoning effort')}</Label>
        {props.help}
      </div>
      <p className='flex min-h-8 items-center text-sm'>
        {t(
          'This model does not support reasoning effort. No reasoning parameter is sent with the request'
        )}
      </p>
    </div>
  )
}

export function EffortCapabilityStatus(props: {
  failed: boolean
  unconfirmed: boolean
}) {
  const { t } = useTranslation()
  return (
    <>
      {props.failed ? (
        <span className='text-destructive text-sm'>
          {t('Thinking capability failed to load')}
        </span>
      ) : null}
      {props.unconfirmed ? (
        <Tooltip>
          <TooltipTrigger
            render={
              <Badge variant='warning' tabIndex={0} className='shrink-0' />
            }
          >
            {t('Capability not confirmed')}
          </TooltipTrigger>
          <TooltipContent role='tooltip' className='max-w-72'>
            {t(
              'This model is not in the built-in capability table. Add it under Model thinking capabilities'
            )}
          </TooltipContent>
        </Tooltip>
      ) : null}
    </>
  )
}

type TierEffortFieldProps = {
  tierId: SmartTierId
  capability: EffectiveCapability
  unconfirmed: boolean
  unavailable: boolean
  failed: boolean
}

export function TierEffortField(props: TierEffortFieldProps) {
  const { t } = useTranslation()
  const form = useFormContext<RouteFormValues>()
  const base = `tiers.${props.tierId}` as const
  const items = props.capability.efforts.map((effort) => ({
    value: effort,
    label: effort,
  }))
  const modes: { value: string; label: string }[] = [
    { value: INHERIT, label: t('Not overridden') },
    { value: 'fixed', label: t('Fixed', { context: 'routing' }) },
    { value: 'follow', label: t('Follow scoring') },
  ]

  const effortSelect = (
    name: `${typeof base}.${(typeof EFFORT_FIELDS)[number]}`,
    label: string
  ) => (
    <FormField
      control={form.control}
      name={name}
      render={({ field, fieldState }) => (
        <Select
          items={items}
          value={field.value}
          onValueChange={field.onChange}
        >
          <SelectTrigger
            size='sm'
            className='w-28'
            aria-label={label}
            aria-invalid={!!fieldState.error}
          >
            <SelectValue placeholder={t('Select')} />
          </SelectTrigger>
          <SelectContent>
            {items.map((item) => (
              <SelectItem key={item.value} value={item.value}>
                {item.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      )}
    />
  )

  const changeMode = (mode: SmartEffortMode) => {
    form.setValue(`${base}.effort_mode`, mode)
    if (mode !== 'fixed') form.setValue(`${base}.effort`, '')
    if (mode !== 'follow') {
      form.setValue(`${base}.effort_min`, '')
      form.setValue(`${base}.effort_max`, '')
    }
    form.clearErrors(EFFORT_FIELDS.map((name) => `${base}.${name}` as const))
  }

  const help = (
    <FieldInfo label={t('Reasoning effort')}>
      {t(
        'Not overridden keeps the value sent by the client and switches to the closest level when the model does not support it. Follow scoring uses the thinking need from scoring (0 to 1, higher when the goal is vague or the problem is hard) and picks proportionally among the levels this model supports between the minimum and maximum; with 3 levels, 0 to 0.33 picks the lowest and 0.67 to 1 the highest. The highest of them is used when scoring fails. The dropdowns only list the levels this model supports'
      )}
    </FieldInfo>
  )

  if (props.capability.noThinking) {
    return <NoThinkingEffortRow help={help} />
  }

  return (
    <FormField
      control={form.control}
      name={`${base}.effort_mode`}
      render={({ field }) => (
        <FormItem className={FIELD_ROW_CLASS}>
          <div className='flex min-h-8 items-center gap-1'>
            <FormLabel>{t('Reasoning effort')}</FormLabel>
            {help}
          </div>
          <div className='min-w-0'>
            <div className='flex min-h-8 flex-wrap items-center gap-x-4 gap-y-2'>
              <ToggleGroup
                variant='outline'
                size='sm'
                aria-label={t('Reasoning effort mode')}
                disabled={props.unavailable}
                value={[field.value || INHERIT]}
                onValueChange={(next) => {
                  if (next[0]) {
                    changeMode(
                      next[0] === INHERIT ? '' : (next[0] as SmartEffortMode)
                    )
                  }
                }}
              >
                {modes.map((mode) => (
                  <ToggleGroupItem
                    key={mode.value}
                    value={mode.value}
                    className='aria-pressed:border-primary/30 aria-pressed:bg-primary/10 aria-pressed:text-primary text-muted-foreground'
                  >
                    {mode.label}
                  </ToggleGroupItem>
                ))}
              </ToggleGroup>
              {!props.unavailable &&
                field.value === 'fixed' &&
                effortSelect(`${base}.effort`, t('Reasoning effort'))}
              {!props.unavailable && field.value === 'follow' && (
                <span className='inline-flex min-w-0 flex-wrap items-center gap-1.5'>
                  {effortSelect(`${base}.effort_min`, t('Minimum effort'))}
                  <span aria-hidden='true'>～</span>
                  {effortSelect(`${base}.effort_max`, t('Maximum effort'))}
                </span>
              )}
              <EffortCapabilityStatus
                failed={props.unavailable && props.failed}
                unconfirmed={props.unconfirmed}
              />
            </div>
            {EFFORT_FIELDS.map((name) => (
              <EffortMessage key={name} name={`${base}.${name}`} />
            ))}
          </div>
        </FormItem>
      )}
    />
  )
}

function EffortMessage(props: {
  name: `tiers.${SmartTierId}.${(typeof EFFORT_FIELDS)[number]}`
}) {
  const form = useFormContext<RouteFormValues>()
  return (
    <FormField
      control={form.control}
      name={props.name}
      render={() => <FormMessage />}
    />
  )
}
