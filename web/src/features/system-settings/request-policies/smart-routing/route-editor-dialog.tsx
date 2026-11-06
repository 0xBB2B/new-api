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
import { zodResolver } from '@hookform/resolvers/zod'
import { useQuery } from '@tanstack/react-query'
import { ChevronDown } from 'lucide-react'
import { useMemo, useRef, useState } from 'react'
import { useForm, useWatch, type FieldPath } from 'react-hook-form'
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Separator } from '@/components/ui/separator'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { getEnabledModels } from '@/features/channels/api'
import { getGroups } from '@/features/users/api'
import { useMediaQuery } from '@/hooks/use-media-query'
import { handleServerError } from '@/lib/handle-server-error'
import { requireServerSuccess } from '@/lib/server-error-message'
import { cn } from '@/lib/utils'

import {
  getBuiltinThinkingCapabilities,
  validateSmartRoute,
  type BuiltinThinkingCapabilities,
} from './api'
import { effectiveCapability } from './capability'
import {
  errorFields,
  firstErrorLocation,
  tabOfField,
  type SmartEditorTab,
} from './error-location'
import { FieldInfo } from './field-info'
import { endpointShortNames, slotSummaries, SMART_TIER_IDS } from './lib'
import { AdvisorPanel, ScorerPanel, TiersPanel } from './route-editor-panels'
import {
  createRouteFormSchema,
  emptyRouteFormValues,
  formValuesToRoute,
  routeToFormValues,
  type RouteFormValues,
} from './route-form'
import type {
  SmartEndpoint,
  SmartRoute,
  SmartTierId,
  ThinkingCapabilityOverride,
} from './types'

const ROUTE_FORM_ID = 'smart-route-form'
const ENDPOINTS: SmartEndpoint[] = ['openai-response', 'openai', 'anthropic']
const ADVANCED_FIELDS = [
  'downgrade_max_history_tokens',
  'confidence_threshold',
  'simple_max_message_chars',
]
const TAB_TRIGGER_CLASS =
  'h-auto min-h-14 w-full flex-col items-start justify-center gap-1 border-input px-3 py-2 text-left after:hidden data-active:border-primary/30 data-active:bg-primary/10! data-active:text-primary'

type AdvancedFieldName = Extract<
  FieldPath<RouteFormValues>,
  | 'downgrade_max_history_tokens'
  | 'confidence_threshold'
  | 'simple_max_message_chars'
>

type RouteEditorDialogProps = {
  route: SmartRoute | null
  otherNames: string[]
  overrides: Record<string, ThinkingCapabilityOverride>
  builtin: BuiltinThinkingCapabilities | undefined
  onOpenChange: (open: boolean) => void
  onSave: (route: SmartRoute) => void
}

export function RouteEditorDialog(props: RouteEditorDialogProps) {
  const { t } = useTranslation()
  const [advancedOpen, setAdvancedOpen] = useState(false)
  const isPhone = useMediaQuery('(max-width: 767px)')
  const [tab, setTab] = useState<SmartEditorTab>('scoring')
  const [tier, setTier] = useState<SmartTierId>('simple')
  const modelsQuery = useQuery({
    queryKey: ['smart-routing', 'enabled-models'],
    queryFn: async () => requireServerSuccess(await getEnabledModels()),
  })
  const groupsQuery = useQuery({
    queryKey: ['smart-routing', 'groups'],
    queryFn: async () => requireServerSuccess(await getGroups()),
  })
  const modelOptions = useMemo(
    () => (modelsQuery.data?.data ?? []).map((m) => ({ value: m, label: m })),
    [modelsQuery.data]
  )
  const groupOptions = useMemo(
    () => (groupsQuery.data?.data ?? []).map((g) => ({ value: g, label: g })),
    [groupsQuery.data]
  )
  const schema = useMemo(() => createRouteFormSchema(t), [t])
  const form = useForm<RouteFormValues>({
    resolver: zodResolver(schema),
    defaultValues: props.route
      ? routeToFormValues(props.route)
      : emptyRouteFormValues(),
  })
  const values = useWatch({ control: form.control }) as RouteFormValues
  const route = formValuesToRoute(values)
  const summaries = slotSummaries(route, t)

  const askedModels = [
    ...new Set(
      SMART_TIER_IDS.map((id) => route.tiers[id].model).filter(Boolean)
    ),
  ].sort()
  const builtinQuery = useQuery({
    queryKey: ['smart-routing', 'builtin-capabilities', askedModels],
    queryFn: () => getBuiltinThinkingCapabilities(askedModels),
    enabled: askedModels.length > 0,
    placeholderData: (previous) => previous ?? props.builtin,
  })
  const builtin = builtinQuery.data ?? {}
  const capabilities = {
    simple: effectiveCapability(
      route.tiers.simple.model,
      builtin,
      props.overrides
    ),
    normal: effectiveCapability(
      route.tiers.normal.model,
      builtin,
      props.overrides
    ),
    hard: effectiveCapability(route.tiers.hard.model, builtin, props.overrides),
  }
  const unconfirmed = {
    simple: isUnconfirmed(route.tiers.simple.model),
    normal: isUnconfirmed(route.tiers.normal.model),
    hard: isUnconfirmed(route.tiers.hard.model),
  }
  const unavailable = {
    simple: isUnavailable(route.tiers.simple.model),
    normal: isUnavailable(route.tiers.normal.model),
    hard: isUnavailable(route.tiers.hard.model),
  }
  function isUnavailable(model: string) {
    return model !== '' && !(model in builtin) && !props.overrides[model]
  }
  function isUnconfirmed(model: string) {
    return (
      model !== '' &&
      model in builtin &&
      effectiveCapability(model, builtin, props.overrides).source ===
        'unconfirmed'
    )
  }

  const fields = errorFields(form.formState.errors)
  const errorCount = (target: SmartEditorTab) =>
    fields.filter((field) => tabOfField(field) === target).length
  const tierErrorCounts = {
    simple: fields.filter((f) => f.startsWith('tiers.simple.')).length,
    normal: fields.filter((f) => f.startsWith('tiers.normal.')).length,
    hard: fields.filter((f) => f.startsWith('tiers.hard.')).length,
  }

  const closedRef = useRef(false)
  const close = (open: boolean) => {
    if (!open) closedRef.current = true
    props.onOpenChange(open)
  }

  const showErrors = (errorFieldNames: string[]) => {
    const location = firstErrorLocation(errorFieldNames)
    if (location) {
      setTab(location.tab)
      if (location.tier) setTier(location.tier)
    }
    if (errorFieldNames.some((field) => ADVANCED_FIELDS.includes(field))) {
      setAdvancedOpen(true)
    }
  }

  const submit = async (formValues: RouteFormValues) => {
    const next = formValuesToRoute(formValues)
    if (props.otherNames.includes(next.name)) {
      form.setError('name', {
        type: 'server',
        message: t('A virtual model named {{name}} already exists', {
          name: next.name,
        }),
      })
      return
    }
    try {
      const result = await validateSmartRoute({ route: next })
      if (closedRef.current) return
      if (result.errors.length > 0) {
        for (const error of result.errors) {
          form.setError(error.field as FieldPath<RouteFormValues>, {
            type: 'server',
            message: t(error.message.key, error.message.params),
          })
        }
        showErrors(result.errors.map((error) => error.field))
        return
      }
      props.onSave(next)
      close(false)
    } catch (error) {
      handleServerError(error)
    }
  }

  const tabs: { id: SmartEditorTab; title: string; summary: string }[] = [
    { id: 'scoring', title: t('Scoring'), summary: summaries.scorer },
    { id: 'tiers', title: t('Tiers'), summary: summaries.tiers },
    { id: 'advisor', title: t('Advisor'), summary: summaries.advisor },
  ]

  return (
    <Dialog
      open
      onOpenChange={close}
      title={props.route ? t('Edit virtual model') : t('Add virtual model')}
      contentClassName='sm:max-w-5xl'
      footer={
        <>
          <Button type='button' variant='outline' onClick={() => close(false)}>
            {t('Cancel')}
          </Button>
          <Button
            type='submit'
            form={ROUTE_FORM_ID}
            disabled={form.formState.isSubmitting}
          >
            {t('Save')}
          </Button>
        </>
      }
    >
      <Form {...form}>
        <form
          id={ROUTE_FORM_ID}
          noValidate
          onSubmit={form.handleSubmit(submit, (errors) =>
            showErrors(errorFields(errors))
          )}
          className='flex min-w-0 flex-col gap-5'
        >
          <div className='grid items-start gap-3 sm:grid-cols-2'>
            <FormField
              control={form.control}
              name='name'
              render={({ field }) => (
                <FormItem className='gap-1.5'>
                  <FormLabel required>{t('Name')}</FormLabel>
                  <FormControl>
                    <Input placeholder={t('e.g. auto-gpt')} {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='endpoints'
              render={({ field }) => (
                <FormItem className='gap-1.5'>
                  <div className='flex items-center gap-1'>
                    <FormLabel>{t('Interface')}</FormLabel>
                    <FieldInfo label={t('Interface')}>
                      {t(
                        'Clients can request this virtual model in any ticked format; other formats get a 400. The channel converts the format. Pick at least one'
                      )}
                    </FieldInfo>
                  </div>
                  <ToggleGroup
                    multiple
                    variant='outline'
                    size='sm'
                    aria-label={t('Interface')}
                    value={field.value}
                    onValueChange={(next) =>
                      field.onChange(
                        ENDPOINTS.filter((endpoint) => next.includes(endpoint))
                      )
                    }
                  >
                    {ENDPOINTS.map((endpoint) => (
                      <Tooltip key={endpoint}>
                        <TooltipTrigger
                          render={
                            <ToggleGroupItem
                              value={endpoint}
                              className='aria-pressed:border-primary/30 aria-pressed:bg-primary/10 aria-pressed:text-primary text-muted-foreground'
                            />
                          }
                        >
                          {endpointShortNames([endpoint])}
                        </TooltipTrigger>
                        <TooltipContent>
                          {endpointPath(endpoint, t)}
                        </TooltipContent>
                      </Tooltip>
                    ))}
                  </ToggleGroup>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>

          <Separator />

          <Tabs
            orientation={isPhone ? 'horizontal' : 'vertical'}
            value={tab}
            onValueChange={(value) => setTab(value as SmartEditorTab)}
            className='flex-col gap-4 md:flex-row md:gap-6'
          >
            <TabsList
              variant='line'
              aria-label={t('Virtual model settings')}
              aria-orientation={isPhone ? 'horizontal' : 'vertical'}
              className={
                isPhone
                  ? 'grid h-auto! w-full grid-cols-3 gap-2 p-0'
                  : 'w-full gap-2 p-0 md:w-38 md:shrink-0'
              }
            >
              {tabs.map((item) => (
                <TabsTrigger
                  key={item.id}
                  value={item.id}
                  className={TAB_TRIGGER_CLASS}
                >
                  <span className='flex items-center gap-1.5 text-sm font-medium'>
                    {item.title}
                    {errorCount(item.id) > 0 ? (
                      <span
                        aria-hidden='true'
                        className='bg-destructive size-1.5 shrink-0 rounded-full'
                      />
                    ) : null}
                  </span>
                  <span
                    className={cn(
                      'max-w-full truncate text-xs font-normal',
                      errorCount(item.id) > 0
                        ? 'text-destructive'
                        : 'text-muted-foreground'
                    )}
                  >
                    {item.summary}
                  </span>
                  {errorCount(item.id) > 0 ? (
                    <span className='text-destructive text-xs font-normal'>
                      {t('{{count}} errors', { count: errorCount(item.id) })}
                    </span>
                  ) : null}
                </TabsTrigger>
              ))}
            </TabsList>
            <div className='min-w-0 flex-1'>
              <TabsContent value='scoring'>
                <ScorerPanel
                  modelOptions={modelOptions}
                  groupOptions={groupOptions}
                />
              </TabsContent>
              <TabsContent value='tiers'>
                <TiersPanel
                  modelOptions={modelOptions}
                  groupOptions={groupOptions}
                  tier={tier}
                  onTierChange={setTier}
                  capabilities={capabilities}
                  unconfirmed={unconfirmed}
                  tierErrorCounts={tierErrorCounts}
                  unavailable={unavailable}
                  capabilityFailed={builtinQuery.isError}
                />
              </TabsContent>
              <TabsContent value='advisor'>
                <AdvisorPanel
                  modelOptions={modelOptions}
                  groupOptions={groupOptions}
                />
              </TabsContent>
            </div>
          </Tabs>

          <Separator />

          <Collapsible open={advancedOpen} onOpenChange={setAdvancedOpen}>
            <CollapsibleTrigger
              render={<Button type='button' variant='ghost' size='sm' />}
            >
              <ChevronDown
                aria-hidden='true'
                data-icon='inline-start'
                className={advancedOpen ? '' : '-rotate-90'}
              />
              {t('Advanced settings')}
            </CollapsibleTrigger>
            <CollapsibleContent keepMounted className='pt-4'>
              <div className='grid gap-4 sm:grid-cols-3'>
                <AdvancedField
                  name='downgrade_max_history_tokens'
                  label={t('Downgrade history token limit')}
                  hint={t(
                    'No downgrade once the history exceeds this length, to avoid re-reading the whole history at full price'
                  )}
                />
                <AdvancedField
                  name='confidence_threshold'
                  label={t('Confidence threshold')}
                  hint={t(
                    'The default tier is used when the scorer is less sure than this'
                  )}
                  step='0.05'
                />
                <AdvancedField
                  name='simple_max_message_chars'
                  label={t('Simple message length limit (characters)')}
                  hint={t(
                    'A latest message longer than this is not treated as simple'
                  )}
                />
              </div>
            </CollapsibleContent>
          </Collapsible>
        </form>
      </Form>
    </Dialog>
  )
}

function endpointPath(
  endpoint: SmartEndpoint,
  t: ReturnType<typeof useTranslation>['t']
) {
  if (endpoint === 'openai-response') {
    return t(
      'OpenAI Responses (/v1/responses). Responses compact requests (/v1/responses/compact) follow it'
    )
  }
  if (endpoint === 'openai') return 'OpenAI (/v1/chat/completions)'
  return 'Anthropic (/v1/messages)'
}

type AdvancedFieldProps = {
  name: AdvancedFieldName
  label: string
  hint: string
  step?: string
}

function AdvancedField(props: AdvancedFieldProps) {
  return (
    <FormField
      name={props.name}
      render={({ field }) => (
        <FormItem>
          <div className='flex items-center gap-1'>
            <FormLabel>{props.label}</FormLabel>
            <FieldInfo label={props.label}>{props.hint}</FieldInfo>
          </div>
          <FormControl>
            <Input
              type='number'
              step={props.step}
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
        </FormItem>
      )}
    />
  )
}
