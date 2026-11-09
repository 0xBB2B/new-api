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
import { useFormContext, type FieldPath } from 'react-hook-form'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import {
  ComboboxInput,
  type ComboboxInputOption,
} from '@/components/ui/combobox-input'
import {
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { AutoGroupOrderEditor } from '@/features/keys/components/auto-group-order-editor'

import type { EffectiveCapability } from './capability'
import { FieldInfo } from './field-info'
import { SMART_TIER_IDS, tierEffortSummary } from './lib'
import type { RouteFormValues } from './route-form'
import { FIELD_ROW_CLASS, TierEffortField } from './tier-effort-field'
import type { SmartTierId } from './types'

type ModelFieldName =
  | `tiers.${SmartTierId}.model`
  | 'scorer.model'
  | 'advisor.model'
type GroupsFieldName =
  | `tiers.${SmartTierId}.groups`
  | 'scorer.groups'
  | 'advisor.groups'
type NumberFieldName = Extract<
  FieldPath<RouteFormValues>,
  'scorer.timeout_ms' | 'advisor.after_dissatisfied'
>

type ModelRowProps = {
  name: ModelFieldName
  label: string
  info?: string
  options: ComboboxInputOption[]
}

function RowLabel(props: { label: string; info?: string }) {
  return (
    <div className='flex min-h-8 items-center gap-1'>
      <FormLabel>{props.label}</FormLabel>
      {props.info ? (
        <FieldInfo label={props.label}>{props.info}</FieldInfo>
      ) : null}
    </div>
  )
}

function ModelRow(props: ModelRowProps) {
  const { t } = useTranslation()
  const form = useFormContext<RouteFormValues>()
  return (
    <FormField
      control={form.control}
      name={props.name}
      render={({ field, fieldState }) => (
        <FormItem className={FIELD_ROW_CLASS}>
          <RowLabel label={props.label} info={props.info} />
          <div className='min-w-0'>
            <ComboboxInput
              options={props.options}
              value={field.value}
              onValueChange={field.onChange}
              placeholder={t('Select a model')}
              openOnFocus={false}
              aria-label={props.label}
              aria-invalid={!!fieldState.error}
            />
            <FormMessage />
          </div>
        </FormItem>
      )}
    />
  )
}

function GroupsRow(props: {
  name: GroupsFieldName
  options: ComboboxInputOption[]
}) {
  const { t } = useTranslation()
  const form = useFormContext<RouteFormValues>()
  return (
    <FormField
      control={form.control}
      name={props.name}
      render={({ field }) => (
        <FormItem className={FIELD_ROW_CLASS}>
          <RowLabel
            label={t('Group order')}
            info={t(
              'Groups are tried in order; the next group is used when every channel in the previous one fails. At most 5 groups. Drag a group to change the order'
            )}
          />
          <div className='min-w-0'>
            <FormControl>
              <AutoGroupOrderEditor
                variant='compact'
                value={field.value}
                mode='custom'
                options={props.options}
                globalOptions={[]}
                maxCount={5}
                hideInheritControls
                addLabel={t('Add group')}
                aria-label={t('Group order')}
                onChange={({ groups }) => field.onChange(groups)}
              />
            </FormControl>
            <FormMessage />
          </div>
        </FormItem>
      )}
    />
  )
}

function NumberRow(props: {
  name: NumberFieldName
  label: string
  info?: string
}) {
  const form = useFormContext<RouteFormValues>()
  return (
    <FormField
      control={form.control}
      name={props.name}
      render={({ field }) => (
        <FormItem className={FIELD_ROW_CLASS}>
          <RowLabel label={props.label} info={props.info} />
          <div className='min-w-0'>
            <FormControl>
              <Input
                type='number'
                name={field.name}
                ref={field.ref}
                onBlur={field.onBlur}
                value={Number.isNaN(field.value) ? '' : field.value}
                onChange={(e) =>
                  field.onChange(
                    e.target.value === '' ? Number.NaN : Number(e.target.value)
                  )
                }
              />
            </FormControl>
            <FormMessage />
          </div>
        </FormItem>
      )}
    />
  )
}

function ScorerEffortField(props: {
  capability: EffectiveCapability
  unconfirmed: boolean
  unavailable: boolean
  failed: boolean
}) {
  const { t } = useTranslation()
  const form = useFormContext<RouteFormValues>()
  const model = form.watch('scorer.model')
  const items = [
    { value: '', label: t('Not set (model default)') },
    ...props.capability.efforts.map((effort) => ({
      value: effort,
      label: effort,
    })),
  ]
  const help = (
    <FieldInfo label={t('Reasoning effort')}>
      {t(
        'Leave empty to send no reasoning parameter and let the model decide. A selected level the model does not support is switched to the closest level in the capability table before sending. Scorer output has no length limit; the timeout bounds how long it takes. The list only shows the levels this model supports'
      )}
    </FieldInfo>
  )

  if (props.capability.noThinking) {
    return (
      <div className={`${FIELD_ROW_CLASS} text-muted-foreground`}>
        <div className='flex min-h-8 items-center gap-1'>
          <Label className='text-muted-foreground'>
            {t('Reasoning effort')}
          </Label>
          {help}
        </div>
        <p className='flex min-h-8 items-center text-sm'>
          {t(
            'This model does not support reasoning effort. No reasoning parameter is sent with the request'
          )}
        </p>
      </div>
    )
  }

  return (
    <FormField
      control={form.control}
      name='scorer.effort'
      render={({ field, fieldState }) => (
        <FormItem className={FIELD_ROW_CLASS}>
          <div className='flex min-h-8 items-center gap-1'>
            <FormLabel>{t('Reasoning effort')}</FormLabel>
            {help}
          </div>
          <div className='min-w-0'>
            <div className='flex min-h-8 flex-wrap items-center gap-x-4 gap-y-2'>
              <Select
                items={items}
                value={field.value}
                onValueChange={field.onChange}
                disabled={model === '' || props.unavailable}
              >
                <SelectTrigger
                  size='sm'
                  className='w-44'
                  aria-label={t('Reasoning effort')}
                  aria-invalid={!!fieldState.error}
                >
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {items.map((item) => (
                    <SelectItem key={item.value} value={item.value}>
                      {item.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              {props.unavailable && props.failed ? (
                <span className='text-destructive text-sm'>
                  {t('Thinking capability failed to load')}
                </span>
              ) : null}
              {props.unconfirmed ? (
                <Tooltip>
                  <TooltipTrigger
                    render={
                      <Badge
                        variant='warning'
                        tabIndex={0}
                        className='shrink-0'
                      />
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
            </div>
            <FormMessage />
          </div>
        </FormItem>
      )}
    />
  )
}

type PanelOptions = {
  modelOptions: ComboboxInputOption[]
  groupOptions: ComboboxInputOption[]
}

type ScorerPanelProps = PanelOptions & {
  capability: EffectiveCapability
  unconfirmed: boolean
  unavailable: boolean
  failed: boolean
}

export function ScorerPanel(props: ScorerPanelProps) {
  const { t } = useTranslation()
  return (
    <div className='flex flex-col gap-3'>
      <ModelRow
        name='scorer.model'
        label={t('Scorer model')}
        info={t(
          'Called once for each new user message. It is billed at the scorer model price as a separate usage record.'
        )}
        options={props.modelOptions}
      />
      <ScorerEffortField
        capability={props.capability}
        unconfirmed={props.unconfirmed}
        unavailable={props.unavailable}
        failed={props.failed}
      />
      <NumberRow
        name='scorer.timeout_ms'
        label={t('Timeout (ms)')}
        info={t('The default tier is used on timeout')}
      />
      <GroupsRow name='scorer.groups' options={props.groupOptions} />
    </div>
  )
}

export function AdvisorPanel(props: PanelOptions) {
  const { t } = useTranslation()
  return (
    <div className='flex flex-col gap-3'>
      <ModelRow
        name='advisor.model'
        label={t('Advisor model')}
        info={t(
          'When a user is dissatisfied several messages in a row in one conversation, the advisor answers this turn.'
        )}
        options={props.modelOptions}
      />
      <NumberRow
        name='advisor.after_dissatisfied'
        label={t('Dissatisfied messages before the advisor answers')}
      />
      <GroupsRow name='advisor.groups' options={props.groupOptions} />
    </div>
  )
}

type TiersPanelProps = PanelOptions & {
  tier: SmartTierId
  onTierChange: (tier: SmartTierId) => void
  capabilities: Record<SmartTierId, EffectiveCapability>
  unconfirmed: Record<SmartTierId, boolean>
  unavailable: Record<SmartTierId, boolean>
  capabilityFailed: boolean
  tierErrorCounts: Record<SmartTierId, number>
}

export function TiersPanel(props: TiersPanelProps) {
  const { t } = useTranslation()
  const form = useFormContext<RouteFormValues>()
  const tiers = form.watch('tiers')
  const defaultTier = form.watch('default_tier')
  const titles: Record<SmartTierId, string> = {
    simple: t('Simple tier'),
    normal: t('Normal tier'),
    hard: t('Hard tier'),
  }
  const names: Record<SmartTierId, string> = {
    simple: t('Simple', { context: 'tier' }),
    normal: t('Normal', { context: 'tier' }),
    hard: t('Hard', { context: 'tier' }),
  }
  const tierItems = SMART_TIER_IDS.map((id) => ({
    value: id,
    label: titles[id],
  }))

  return (
    <Tabs
      value={props.tier}
      onValueChange={(value) => props.onTierChange(value as SmartTierId)}
      className='gap-3'
    >
      <div className='flex flex-col gap-2 sm:flex-row sm:items-center sm:gap-3'>
        <TabsList aria-label={t('Tiers')} className='min-w-0 flex-1'>
          {SMART_TIER_IDS.map((id) => (
            <TierTab
              key={id}
              id={id}
              name={names[id]}
              isDefault={defaultTier === id}
              errorCount={props.tierErrorCounts[id]}
              summary={
                tiers[id].model
                  ? `${tiers[id].model} · ${tierEffortSummary(tiers[id], props.capabilities[id], t)}`
                  : ''
              }
              model={tiers[id].model}
            />
          ))}
        </TabsList>
        <FormField
          control={form.control}
          name='default_tier'
          render={({ field, fieldState }) => (
            <FormItem className='flex shrink-0 flex-wrap items-center gap-1.5'>
              <RowLabel
                label={t('Default tier')}
                info={t(
                  'The default tier is used in all 4 cases below: scoring fails or times out; the scorer is not sure enough; there is no session when a tool call continues; the chosen tier has no channel'
                )}
              />
              <Select
                items={tierItems}
                value={field.value}
                onValueChange={field.onChange}
              >
                <SelectTrigger
                  className='w-28'
                  aria-label={t('Default tier')}
                  aria-invalid={!!fieldState.error}
                >
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {tierItems.map((item) => (
                    <SelectItem key={item.value} value={item.value}>
                      {item.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <FormMessage className='w-full text-right' />
            </FormItem>
          )}
        />
      </div>
      {SMART_TIER_IDS.map((id) => (
        <TabsContent key={id} value={id} className='flex flex-col gap-3'>
          <ModelRow
            name={`tiers.${id}.model`}
            label={t('{{tier}} model', { tier: titles[id] })}
            info={t(
              'Each new user message is scored and then goes to one of the tiers'
            )}
            options={props.modelOptions}
          />
          <TierEffortField
            tierId={id}
            capability={props.capabilities[id]}
            unconfirmed={props.unconfirmed[id]}
            unavailable={props.unavailable[id]}
            failed={props.capabilityFailed}
          />
          <GroupsRow name={`tiers.${id}.groups`} options={props.groupOptions} />
        </TabsContent>
      ))}
    </Tabs>
  )
}

type TierTabProps = {
  id: SmartTierId
  name: string
  model: string
  summary: string
  isDefault: boolean
  errorCount: number
}

function TierTab(props: TierTabProps) {
  const { t } = useTranslation()
  const trigger = <TabsTrigger value={props.id} className='min-w-0 px-3' />
  const content = (
    <>
      <span className='shrink-0'>{props.name}</span>
      {props.errorCount > 0 ? (
        <span
          aria-hidden='true'
          className='bg-destructive size-1.5 shrink-0 rounded-full'
        />
      ) : null}
      {props.model ? (
        <span className='text-muted-foreground hidden min-w-6 truncate text-xs font-normal sm:block'>
          {`· ${props.model}`}
        </span>
      ) : null}
      {props.isDefault ? (
        <Badge variant='outline' className='shrink-0'>
          {t('Default')}
        </Badge>
      ) : null}
      {props.errorCount > 0 ? (
        <span className='text-destructive shrink-0 text-xs font-normal'>
          {t('{{count}} errors', { count: props.errorCount })}
        </span>
      ) : null}
    </>
  )
  if (!props.summary) {
    return (
      <TabsTrigger value={props.id} className='min-w-0 px-3'>
        {content}
      </TabsTrigger>
    )
  }
  return (
    <Tooltip>
      <TooltipTrigger render={trigger}>{content}</TooltipTrigger>
      <TooltipContent>{props.summary}</TooltipContent>
    </Tooltip>
  )
}
